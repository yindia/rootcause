package linkerd

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleProxyStatus(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "linkerd-proxy"},
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	client := fake.NewSimpleClientset(pod)
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

	result, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{
			"namespace": "default",
		},
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence in output")
	}
}
