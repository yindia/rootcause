package istio

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestListServicesBranches(t *testing.T) {
	serviceDefault := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}}
	serviceOther := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "other"}}
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
		serviceDefault,
		serviceOther,
	)
	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{Typed: client}
	if err := toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if services, namespaces, err := toolset.listServices(context.Background(), policy.User{Role: policy.RoleCluster}, ""); err != nil || len(services) != 2 || namespaces != nil {
		t.Fatalf("list services cluster: %v %#v %#v", err, services, namespaces)
	}
	if services, namespaces, err := toolset.listServices(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}, ""); err != nil || len(services) != 1 || len(namespaces) != 1 {
		t.Fatalf("list services namespace user: %v %#v %#v", err, services, namespaces)
	}
	if services, namespaces, err := toolset.listServices(context.Background(), policy.User{Role: policy.RoleCluster}, "default"); err != nil || len(services) != 1 || len(namespaces) != 1 {
		t.Fatalf("list services scoped: %v %#v %#v", err, services, namespaces)
	}
}

func TestGatewayServerHosts(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"servers": []any{
				map[string]any{"hosts": []any{"a.example.com", "b.example.com"}},
			},
		},
	}}
	if got := gatewayServerHosts(nil); got != nil {
		t.Fatalf("expected nil hosts")
	}
	if got := gatewayServerHosts(obj); len(got) != 2 {
		t.Fatalf("unexpected hosts: %#v", got)
	}
}
