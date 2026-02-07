package karpenter

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	if _, ok := reg.Get("karpenter.status"); !ok {
		t.Fatalf("expected karpenter.status to be registered")
	}
}

func TestConditionHelpers(t *testing.T) {
	condFalse := map[string]any{"type": "Ready", "status": "False"}
	if !isConditionFalse(condFalse, []string{"Ready"}) {
		t.Fatalf("expected condition false")
	}
	condTrue := map[string]any{"type": "Ready", "status": "True"}
	if !isConditionTrue(condTrue, []string{"Ready"}) {
		t.Fatalf("expected condition true")
	}
}

func TestAWSNodeClassMatch(t *testing.T) {
	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	if !isAWSNodeClass(match, nil) {
		t.Fatalf("expected aws nodeclass match")
	}
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("karpenter.k8s.aws/v1beta1")
	if !isAWSNodeClass(resourceMatch{}, obj) {
		t.Fatalf("expected aws nodeclass match by apiVersion")
	}
}

func TestTypeHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toStringSlice([]any{"a", "b"}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	if _, err := toUnstructured(pod); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}
