package istio

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandlePodsByServiceBranchesMore(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{}},
	}
	client := fake.NewSimpleClientset(service)
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

	if _, err := toolset.handlePodsByService(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "service": "missing"},
	}); err != nil {
		t.Fatalf("handlePodsByService missing: %v", err)
	}

	if _, err := toolset.handlePodsByService(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "service": "api"},
	}); err != nil {
		t.Fatalf("handlePodsByService no selector: %v", err)
	}
}

func TestGatewayServerHostsAndUnstructured(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"servers": []any{
				map[string]any{"hosts": []any{"example.com"}},
				map[string]any{"hosts": []any{float64(123)}},
			},
		},
	}}
	hosts := gatewayServerHosts(obj)
	if len(hosts) != 1 || hosts[0] != "example.com" {
		t.Fatalf("unexpected hosts: %#v", hosts)
	}

	if _, err := toUnstructured(nil); err == nil {
		t.Fatalf("expected toUnstructured error for nil pod")
	}
}
