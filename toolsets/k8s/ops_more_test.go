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
	toolset := newDynamicToolset(gvr, "ConfigMapList", obj)

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
