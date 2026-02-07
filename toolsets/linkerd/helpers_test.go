package linkerd

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
	if _, ok := reg.Get("linkerd.health"); !ok {
		t.Fatalf("expected linkerd.health to be registered")
	}
}

func TestPodHelpers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "linkerd-proxy"}},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	if !hasLinkerdProxy(pod) {
		t.Fatalf("expected linkerd proxy container")
	}
	if !isPodReady(pod) {
		t.Fatalf("expected pod ready")
	}
	if got := toString(12); got != "12" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := sliceIf("ns"); len(got) != 1 || got[0] != "ns" {
		t.Fatalf("unexpected sliceIf: %#v", got)
	}
	if _, err := toUnstructured(pod); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}
