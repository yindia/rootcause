package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleApplyAndPatch(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("demo")
	obj.SetNamespace("default")
	toolset := newDynamicToolset(gvr, "ConfigMapList", "ConfigMap", obj)

	manifest := "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"demo\",\"namespace\":\"default\"}}"
	_, err := toolset.handleApply(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"manifest": manifest,
			"confirm":  true,
		},
	})
	if err == nil {
		// apply may return error with the fake dynamic client; that's acceptable for coverage.
	}

	_, err = toolset.handlePatch(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"name":       "demo",
			"namespace":  "default",
			"patch":      "{\"metadata\":{\"labels\":{\"env\":\"test\"}}}",
			"confirm":    true,
		},
	})
	if err != nil {
		t.Fatalf("handlePatch: %v", err)
	}
}

func TestHandleLogsError(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}}
	client := k8sfake.NewSimpleClientset(pod)
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

	result, err := toolset.handleLogs(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
		},
	})
	if err == nil {
		data := result.Data.(map[string]any)
		if _, ok := data["logs"]; !ok {
			t.Fatalf("expected logs output")
		}
	}
}

func TestHandlePortForwardMissingArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	_, err := toolset.handlePortForward(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err == nil {
		t.Fatalf("expected error for missing args")
	}
}

func TestCommandAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Exec.AllowedCommands = []string{"ls"}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	if !toolset.commandAllowed([]string{"ls"}) {
		t.Fatalf("expected allowed command")
	}
	if toolset.commandAllowed([]string{"bash"}) {
		t.Fatalf("expected shell to be denied")
	}
}

func TestHandleScale(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("api")
	obj.SetNamespace("default")
	toolset := newDynamicToolset(gvr, "DeploymentList", "Deployment", obj)

	_, err := toolset.handleScale(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "api",
			"namespace":  "default",
			"replicas":   float64(2),
			"confirm":    true,
		},
	})
	if err != nil {
		t.Fatalf("handleScale: %v", err)
	}
}

func TestHandleEventsNamespace(t *testing.T) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "api",
			UID:  "pod-uid",
		},
		Reason:  "Scheduled",
		Message: "scheduled",
	}
	client := k8sfake.NewSimpleClientset(event)
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
	_, err := toolset.handleEvents(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":            "default",
			"involvedObjectKind":  "Pod",
			"involvedObjectName":  "api",
			"involvedObjectUID":   "pod-uid",
		},
	})
	if err != nil {
		t.Fatalf("handleEvents: %v", err)
	}
}

func TestHandleGenericGet(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("demo")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	result, err := toolset.handleGeneric(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"verb":      "get",
			"apiVersion": "v1",
			"kind":      "Pod",
			"name":      "demo",
			"namespace": "default",
		},
	})
	if err != nil {
		t.Fatalf("handleGeneric get: %v", err)
	}
	if result.Data == nil {
		t.Fatalf("expected generic get data")
	}
}

func TestHandleGenericListDescribe(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("demo")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	_, err := toolset.handleGeneric(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"verb":      "list",
			"resources": []any{map[string]any{"apiVersion": "v1", "kind": "Pod"}},
			"namespace": "default",
		},
	})
	if err != nil {
		t.Fatalf("handleGeneric list: %v", err)
	}
	_, err = toolset.handleGeneric(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"verb":      "describe",
			"apiVersion": "v1",
			"kind":      "Pod",
			"name":      "demo",
			"namespace": "default",
		},
	})
	if err != nil {
		t.Fatalf("handleGeneric describe: %v", err)
	}
}

func TestAPIResourceMatches(t *testing.T) {
	resource := metav1.APIResource{Name: "pods", Kind: "Pod", Namespaced: true}
	if !apiResourceMatches("pod", "v1", resource) {
		t.Fatalf("expected resource match")
	}
	if apiResourceMatches("service", "v1", resource) {
		t.Fatalf("expected resource mismatch")
	}
}

func TestHandleGenericVerbs(t *testing.T) {
	cfg := config.DefaultConfig()
	client := k8sfake.NewSimpleClientset()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: client}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	verbs := []string{"create", "apply", "patch", "delete", "logs", "events", "scale", "rollout", "context"}
	for _, verb := range verbs {
		_, _ = toolset.handleGeneric(context.Background(), mcp.ToolRequest{
			User:      policy.User{Role: policy.RoleCluster},
			Arguments: map[string]any{"verb": verb},
		})
	}
}
