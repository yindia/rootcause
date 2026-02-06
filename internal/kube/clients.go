package kube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Clients struct {
	RestConfig *rest.Config
	Typed      kubernetes.Interface
	Dynamic    dynamic.Interface
	Discovery  discovery.DiscoveryInterface
	Mapper     meta.RESTMapper
	Metrics    metricsclient.Interface
}

type Config struct {
	Kubeconfig string
	Context    string
}

func NewClients(cfg Config) (*Clients, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if explicit := kubeconfigPath(cfg.Kubeconfig); explicit != "" {
		loadingRules.ExplicitPath = explicit
	}
	loading := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: cfg.Context},
	)
	restConfig, err := loading.ClientConfig()
	if err != nil {
		return nil, err
	}

	typed, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	metricsClient, err := metricsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &Clients{
		RestConfig: restConfig,
		Typed:      typed,
		Dynamic:    dynamicClient,
		Discovery:  discoveryClient,
		Mapper:     mapper,
		Metrics:    metricsClient,
	}, nil
}

func kubeconfigPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home := homedir.HomeDir()
		if home == "" {
			return path
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return os.ExpandEnv(path)
}

func ResolveResource(mapper meta.RESTMapper, apiVersion, kind, resource string) (schema.GroupVersionResource, bool, error) {
	if mapper == nil {
		return schema.GroupVersionResource{}, false, errors.New("missing rest mapper")
	}
	if resource != "" {
		groupResource := schema.ParseGroupResource(resource)
		var gvr schema.GroupVersionResource
		if apiVersion != "" {
			gv, err := schema.ParseGroupVersion(apiVersion)
			if err == nil {
				gvr = schema.GroupVersionResource{Group: groupResource.Group, Version: gv.Version, Resource: groupResource.Resource}
			}
		} else {
			gvr = schema.GroupVersionResource{Group: groupResource.Group, Resource: groupResource.Resource}
		}
		resolved, err := mapper.ResourceFor(gvr)
		if err == nil {
			gvk, err := mapper.KindFor(resolved)
			if err == nil {
				mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
				if err == nil {
					return mapping.Resource, mapping.Scope.Name() == meta.RESTScopeNameNamespace, nil
				}
			}
		}
	}
	if apiVersion == "" || kind == "" {
		return schema.GroupVersionResource{}, false, errors.New("apiVersion and kind required")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	return mapping.Resource, mapping.Scope.Name() == meta.RESTScopeNameNamespace, nil
}

func ResolveResourceBestEffort(mapper meta.RESTMapper, discoveryClient discovery.DiscoveryInterface, apiVersion, kind, resource, group string) (schema.GroupVersionResource, bool, error) {
	if mapper == nil {
		return schema.GroupVersionResource{}, false, errors.New("missing rest mapper")
	}
	if group == "" && resource != "" && strings.Contains(resource, ".") {
		parts := strings.SplitN(resource, ".", 2)
		if len(parts) == 2 {
			resource = parts[0]
			group = parts[1]
		}
	}
	if apiVersion != "" || resource != "" {
		gvr, namespaced, err := ResolveResource(mapper, apiVersion, kind, resource)
		if err == nil {
			return gvr, namespaced, nil
		}
		if apiVersion != "" {
			return schema.GroupVersionResource{}, false, err
		}
	}
	if kind == "" && resource == "" {
		return schema.GroupVersionResource{}, false, errors.New("kind or resource required")
	}
	if discoveryClient == nil {
		return schema.GroupVersionResource{}, false, errors.New("missing discovery client")
	}

	lists, err := discoveryClient.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return schema.GroupVersionResource{}, false, err
	}

	type candidate struct {
		gvr          schema.GroupVersionResource
		namespaced   bool
		groupVersion string
	}
	var candidates []candidate
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		if group != "" && gv.Group != group {
			continue
		}
		if apiVersion != "" && list.GroupVersion != apiVersion {
			continue
		}
		for _, res := range list.APIResources {
			if res.Name == "" || strings.Contains(res.Name, "/") {
				continue
			}
			if resource != "" {
				if res.Name != resource && res.SingularName != resource && !containsString(res.ShortNames, resource) {
					continue
				}
			} else if kind != "" && res.Kind != kind {
				continue
			}
			candidates = append(candidates, candidate{
				gvr:          schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: res.Name},
				namespaced:   res.Namespaced,
				groupVersion: list.GroupVersion,
			})
		}
	}
	if len(candidates) == 0 {
		if resource != "" {
			return schema.GroupVersionResource{}, false, fmt.Errorf("no matching resource found for %q", resource)
		}
		return schema.GroupVersionResource{}, false, fmt.Errorf("no matching resource found for kind %q", kind)
	}
	if len(candidates) > 1 {
		options := make([]string, 0, len(candidates))
		for _, item := range candidates {
			options = append(options, fmt.Sprintf("%s/%s", item.groupVersion, item.gvr.Resource))
		}
		sort.Strings(options)
		return schema.GroupVersionResource{}, false, fmt.Errorf("multiple matches found; specify apiVersion or group: %s", strings.Join(options, ", "))
	}
	return candidates[0].gvr, candidates[0].namespaced, nil
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
