package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"rootcause/internal/cache"
	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleDiagnoseNoKeywordAndNoMatches(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
	})
	clients := &kube.Clients{Typed: client}
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

	if _, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected error for missing keyword")
	}
	if _, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"keyword": "api", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleDiagnose no matches: %v", err)
	}
}

func TestHandleConfigDebugSecretAndUnsupportedKind(t *testing.T) {
	namespace := "default"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-secret", Namespace: namespace},
		Data:       map[string][]byte{"token": []byte("value")},
	}
	client := k8sfake.NewSimpleClientset(secret)
	clients := &kube.Clients{Typed: client}
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

	if _, err := toolset.handleConfigDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":    namespace,
			"kind":         "secret",
			"name":         "app-secret",
			"requiredKeys": []any{"token", "missing"},
		},
	}); err != nil {
		t.Fatalf("handleConfigDebug secret: %v", err)
	}

	if _, err := toolset.handleConfigDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": namespace,
			"kind":      "unknown",
			"name":      "x",
		},
	}); err == nil {
		t.Fatalf("expected unsupported kind error")
	}
}

func TestHandleVPADebugMetricsUnavailable(t *testing.T) {
	namespace := "default"
	labels := map[string]string{"app": "api"}
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: labels}}
	vpa := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "autoscaling.k8s.io/v1",
		"kind":       "VerticalPodAutoscaler",
		"metadata":   map[string]any{"name": "api-vpa", "namespace": namespace},
		"spec": map[string]any{
			"targetRef": map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "name": "api"},
			"updatePolicy": map[string]any{
				"updateMode": "Off",
			},
		},
	}}

	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: "VerticalPodAutoscalerList"}, vpa)
	resources := []*metav1.APIResourceList{{
		GroupVersion: "autoscaling.k8s.io/v1",
		APIResources: []metav1.APIResource{{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
	}}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{Name: "autoscaling.k8s.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
			},
		},
	})

	client := k8sfake.NewSimpleClientset(deploy, pod)
	clients := &kube.Clients{Typed: client, Dynamic: dyn, Discovery: cached, Mapper: mapper}
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

	if _, err := toolset.handleVPADebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleVPADebug metrics unavailable: %v", err)
	}
}

func TestHandleStorageDebugNoPVCs(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client}
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

	if _, err := toolset.handleStorageDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleStorageDebug no pvcs: %v", err)
	}
}

func TestHandleGraphCache(t *testing.T) {
	toolset := newGraphToolset()
	toolset.ctx.Cache = cache.NewStore()
	toolset.ctx.Config.Cache.GraphTTLSeconds = 60

	request := mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"kind":      "service",
			"name":      "api",
			"namespace": "default",
		},
	}
	if _, err := toolset.handleGraph(context.Background(), request); err != nil {
		t.Fatalf("handleGraph cache fill: %v", err)
	}
	if _, err := toolset.handleGraph(context.Background(), request); err != nil {
		t.Fatalf("handleGraph cache hit: %v", err)
	}
}
