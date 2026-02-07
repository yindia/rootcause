package helm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmtime "helm.sh/helm/v3/pkg/time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type helmDiscovery struct {
	resources []*metav1.APIResourceList
}

func (d *helmDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{}, nil
}

func (d *helmDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, list := range d.resources {
		if list.GroupVersion == groupVersion {
			return list, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (d *helmDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return []*metav1.APIGroup{}, d.resources, nil
}

func (d *helmDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *helmDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *helmDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{GitVersion: "v1.27.0", Major: "1", Minor: "27"}, nil
}

func (d *helmDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *helmDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *helmDiscovery) RESTClient() rest.Interface {
	return nil
}

func (d *helmDiscovery) Fresh() bool {
	return true
}

func (d *helmDiscovery) Invalidate() {}

func (d *helmDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &helmDiscovery{}

type staticRESTClientGetter struct {
	restConfig *rest.Config
	discovery  discovery.CachedDiscoveryInterface
	mapper     meta.RESTMapper
}

func (g *staticRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.restConfig, nil
}

func (g *staticRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return g.discovery, nil
}

func (g *staticRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return g.mapper, nil
}

func (g *staticRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return clientcmd.NewDefaultClientConfig(clientcmdapi.Config{}, &clientcmd.ConfigOverrides{})
}

func TestReadValuesMerge(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "values1.yaml")
	file2 := filepath.Join(dir, "values2.yaml")
	if err := os.WriteFile(file1, []byte("nested:\n  foo: file1\n"), 0o644); err != nil {
		t.Fatalf("write values1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("nested:\n  baz: file2\nother: true\n"), 0o644); err != nil {
		t.Fatalf("write values2: %v", err)
	}
	args := map[string]any{
		"values": map[string]any{
			"nested": map[string]any{"foo": "values"},
		},
		"valuesYAML": "nested:\n  foo: yaml\n  bar: yamlbar\n",
		"valuesFiles": []any{
			file1,
			file2,
		},
	}
	values, err := readValues(args)
	if err != nil {
		t.Fatalf("readValues: %v", err)
	}
	nested, ok := values["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %#v", values["nested"])
	}
	if nested["foo"] != "file1" || nested["bar"] != "yamlbar" || nested["baz"] != "file2" {
		t.Fatalf("unexpected merged values: %#v", nested)
	}
}

func TestDecodeManifestAndSummaries(t *testing.T) {
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\n\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n"
	objects, err := decodeManifest(manifest)
	if err != nil {
		t.Fatalf("decodeManifest: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
	rel := &release.Release{
		Name:      "demo",
		Namespace: "default",
		Version:   1,
		Info: &release.Info{
			Status:       release.StatusDeployed,
			LastDeployed: helmtime.Time{Time: time.Now()},
			Notes:        "ok",
		},
		Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "demo", Version: "0.1.0", AppVersion: "1.0.0"}},
	}
	summary := summarizeRelease(rel)
	if summary["name"] != "demo" || summary["status"] != "deployed" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	list := summarizeReleases([]*release.Release{rel})
	if len(list) != 1 || list[0]["name"] != "demo" {
		t.Fatalf("unexpected summarizeReleases: %#v", list)
	}
}

func TestHandleRepoListAndUpdate(t *testing.T) {
	dir := t.TempDir()
	repoFile := filepath.Join(dir, "repositories.yaml")
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	file := repo.NewFile()
	file.Add(&repo.Entry{Name: "b", URL: "https://example.com/b"})
	file.Add(&repo.Entry{Name: "a", URL: "https://example.com/a"})
	if err := file.WriteFile(repoFile, 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}

	cfg := config.DefaultConfig()
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg}}
	result, err := toolset.handleRepoList(context.Background(), mcp.ToolRequest{})
	if err != nil {
		t.Fatalf("handleRepoList: %v", err)
	}
	repos := result.Data.(map[string]any)["repositories"].([]map[string]any)
	if len(repos) != 2 || repos[0]["name"] != "a" {
		t.Fatalf("unexpected repo list: %#v", repos)
	}

	_, err = toolset.handleRepoUpdate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"name": "missing"}})
	if err == nil {
		t.Fatalf("expected repo update error for missing entry")
	}
}

func TestHandleRepoAdd(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}
	index := "apiVersion: v1\nentries:\n  demo:\n  - version: 0.1.0\n    urls:\n    - demo-0.1.0.tgz\n"
	if err := os.WriteFile(filepath.Join(repoDir, "index.yaml"), []byte(index), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	repoFile := filepath.Join(dir, "repositories.yaml")
	if err := repo.NewFile().WriteFile(repoFile, 0o644); err != nil {
		t.Fatalf("write empty repo file: %v", err)
	}
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	t.Setenv("HELM_REPOSITORY_CACHE", filepath.Join(dir, "cache"))

	cfg := config.DefaultConfig()
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg}}
	_, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "demo", "url": "file://" + repoDir},
	})
	if err == nil {
		t.Fatalf("expected repo add error for unsupported file protocol")
	}
}

func TestHandleListAndStatus(t *testing.T) {
	rel := &release.Release{
		Name:      "demo",
		Namespace: "default",
		Version:   1,
		Info:      &release.Info{Status: release.StatusDeployed},
	}
	toolset := &Toolset{ctx: mcp.ToolsetContext{Policy: policy.NewAuthorizer()}}
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		mem := driver.NewMemory()
		mem.SetNamespace(namespace)
		cfg := &action.Configuration{
			Releases:     storage.Init(mem),
			Log:          func(string, ...interface{}) {},
			Capabilities: nil,
		}
		cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}
		_ = cfg.Releases.Create(rel)
		return cfg, nil
	}

	listResult, err := toolset.handleList(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	})
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if releases := listResult.Data.(map[string]any)["releases"].([]map[string]any); len(releases) != 1 {
		t.Fatalf("expected releases in list")
	}

	statusResult, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"release":   "demo",
		},
	})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if statusResult.Data.(map[string]any)["name"] != "demo" {
		t.Fatalf("unexpected status result: %#v", statusResult.Data)
	}
}

func TestHandleInstallUpgradeUninstall(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	template := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}-config\n  namespace: {{ .Release.Namespace }}\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}

	if _, err := toolset.handleInstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err != nil {
		t.Fatalf("handleInstall: %v", err)
	}

	if _, err := toolset.handleUpgrade(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err != nil {
		t.Fatalf("handleUpgrade: %v", err)
	}

	if _, err := toolset.handleUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"confirm":   true,
		},
	}); err != nil {
		t.Fatalf("handleUninstall: %v", err)
	}
}

func TestHandleTemplateUninstall(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}-config\n  namespace: {{ .Release.Namespace }}\ndata:\n  foo: bar\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "demo-config",
			"namespace": "default",
		},
	}}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, obj)
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	}
	discoveryClient := &helmDiscovery{resources: resources}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		Dynamic:    dynamicClient,
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		getter := &staticRESTClientGetter{
			restConfig: &rest.Config{Host: "https://example.com"},
			mapper:     mapper,
			discovery:  discoveryClient,
		}
		cfg := new(action.Configuration)
		if err := cfg.Init(getter, namespace, "memory", func(string, ...interface{}) {}); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	result, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	})
	if err != nil {
		t.Fatalf("handleTemplateUninstall: %v", err)
	}
	data := result.Data.(map[string]any)
	deleted := data["deleted"].([]string)
	if len(deleted) != 1 || deleted[0] != "configmaps/default/demo-config" {
		t.Fatalf("unexpected delete result: %#v", data)
	}
}

func TestSharedRESTClientGetterRawConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(cfgPath, []byte("apiVersion: v1\nkind: Config\nclusters: []\ncontexts: []\nusers: []\n"), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	getter := &sharedRESTClientGetter{kubeconfig: cfgPath}
	loader := getter.ToRawKubeConfigLoader()
	if _, err := loader.RawConfig(); err != nil {
		t.Fatalf("expected raw config: %v", err)
	}
	toolset := New()
	if toolset.Version() == "" {
		t.Fatalf("expected toolset version")
	}
}
