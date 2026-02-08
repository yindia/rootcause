package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleRolloutStatusRestartAndUnsupported(t *testing.T) {
	replicas := int32(2)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	client := k8sfake.NewSimpleClientset(deploy)
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

	if _, err := toolset.handleRollout(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"action":    "status",
			"name":      "api",
			"namespace": "default",
			"kind":      "Deployment",
		},
	}); err != nil {
		t.Fatalf("handleRollout status: %v", err)
	}

	if _, err := toolset.handleRollout(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"action":    "restart",
			"name":      "api",
			"namespace": "default",
			"kind":      "Deployment",
		},
	}); err != nil {
		t.Fatalf("handleRollout restart: %v", err)
	}

	if _, err := toolset.handleRollout(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"action":    "pause",
			"name":      "api",
			"namespace": "default",
		},
	}); err == nil {
		t.Fatalf("expected unsupported action error")
	}
}
