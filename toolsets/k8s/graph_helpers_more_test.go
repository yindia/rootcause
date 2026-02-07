package k8s

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestGraphNestedHelpers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"selector": map[string]any{"app": "demo"},
			"hosts":    []any{"api.default.svc"},
			"parentRefs": []any{
				map[string]any{"name": "gw"},
				map[string]any{"name": "ignored", "kind": "Service"},
			},
			"rules": []any{
				map[string]any{"backendRefs": []any{
					map[string]any{"name": "api", "kind": "Service"},
					map[string]any{"name": "skip", "kind": "Deployment"},
				}},
			},
		},
	}}
	if got := nestedString(obj, "spec", "missing"); got != "" {
		t.Fatalf("expected empty nestedString")
	}
	if hosts := nestedStringSlice(obj, "spec", "hosts"); len(hosts) != 1 {
		t.Fatalf("expected nestedStringSlice hosts")
	}
	if m := nestedStringMap(obj, "spec", "selector"); m["app"] != "demo" {
		t.Fatalf("expected nestedStringMap selector")
	}
	if refs := nestedParentRefs(obj); len(refs) != 1 || refs[0] != "gw" {
		t.Fatalf("expected nestedParentRefs")
	}
	if backends := nestedBackendRefs(obj); len(backends) != 1 || backends[0] != "api" {
		t.Fatalf("expected nestedBackendRefs")
	}
}

func TestGraphPolicyApplies(t *testing.T) {
	if policyAppliesIngress(nil) || policyAppliesEgress(nil) {
		t.Fatalf("expected nil policy to return false")
	}
	policy := &networkingv1.NetworkPolicy{}
	if !policyAppliesIngress(policy) {
		t.Fatalf("expected default ingress applies")
	}
	policy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}
	if policyAppliesIngress(policy) {
		t.Fatalf("expected ingress not to apply when only egress")
	}
	if !policyAppliesEgress(policy) {
		t.Fatalf("expected egress to apply")
	}
}

func TestLinkIstioAuthorizationPolicyToServiceAccounts(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"rules": []any{
				map[string]any{
					"from": []any{
						map[string]any{"source": map[string]any{"principals": []any{"spiffe://cluster.local/ns/default/sa/app"}}},
					},
				},
			},
		},
	}}
	graph := newGraphBuilder()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{},
		Policy:  policy.NewAuthorizer(),
	})
	res := groupResource{Kind: "AuthorizationPolicy", Group: "security.istio.io"}
	_ = toolset.linkIstioAuthorizationPolicyToServiceAccounts(graph, obj, res, "default")
	if len(graph.edges) == 0 {
		t.Fatalf("expected authorization policy edge")
	}
}
