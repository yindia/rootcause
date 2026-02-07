package k8s

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

func TestPodMatchesKeyword(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Labels: map[string]string{"app": "demo"}}}
	if !podMatchesKeyword(pod, "api") {
		t.Fatalf("expected name match")
	}
	if !podMatchesKeyword(pod, "demo") {
		t.Fatalf("expected label match")
	}
	if podMatchesKeyword(pod, "missing") {
		t.Fatalf("expected no match")
	}
}

func TestHandleDiagnose(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"}}},
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

	result, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"keyword": "api",
		},
	})
	if err != nil {
		t.Fatalf("handleDiagnose: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["evidence"] == nil {
		t.Fatalf("expected evidence output")
	}
}

func TestHandleDiagnoseNoMatches(t *testing.T) {
	client := fake.NewSimpleClientset()
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

	_, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"keyword": "missing",
		},
	})
	if err != nil {
		t.Fatalf("handleDiagnose no matches: %v", err)
	}
}

func TestHandleDiagnoseMissingKeyword(t *testing.T) {
	client := fake.NewSimpleClientset()
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

	_, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatalf("expected error for missing keyword")
	}
}
