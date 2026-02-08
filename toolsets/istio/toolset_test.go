package istio

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestToolsetInitAndRegister(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected error for missing clients")
	}
	ctx := mcp.ToolsetContext{Clients: &kube.Clients{}}
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("istio.health"); !ok {
		t.Fatalf("expected istio.health to be registered")
	}
}

func TestParseProxyPayload(t *testing.T) {
	if got := parseProxyPayload([]byte(" ")); got != "" {
		t.Fatalf("expected empty string, got %#v", got)
	}
	out := parseProxyPayload([]byte(`{"a":1}`))
	if m, ok := out.(map[string]any); !ok || m["a"] != float64(1) {
		t.Fatalf("unexpected json decode: %#v", out)
	}
	if got := parseProxyPayload([]byte("hello")); got != "hello" {
		t.Fatalf("unexpected raw output: %#v", got)
	}
}

func TestIstioHelpers(t *testing.T) {
	if !isGatewayAPIKind("Gateway", "") {
		t.Fatalf("expected gateway api kind")
	}
	if isGatewayAPIKind("VirtualService", "") {
		t.Fatalf("did not expect gateway api kind")
	}
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toInt("5", 1); got != 5 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := sliceIf(""); got != nil {
		t.Fatalf("expected nil slice")
	}
	if got := sliceIf("ns"); len(got) != 1 || got[0] != "ns" {
		t.Fatalf("unexpected sliceIf: %#v", got)
	}
}

func TestIstioPodHelpers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	if !isPodReady(pod) {
		t.Fatalf("expected pod ready")
	}
	if _, err := toUnstructured(pod); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}
