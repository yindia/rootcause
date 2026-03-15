package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleEventsTimelineSorted(t *testing.T) {
	older := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	newer := metav1.NewTime(time.Now())
	client := k8sfake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: "e1", Namespace: "default"},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "back-off restarting container",
			LastTimestamp:  newer,
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "pod-a", Namespace: "default"},
		},
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: "e2", Namespace: "default"},
			Type:           "Warning",
			Reason:         "FailedScheduling",
			Message:        "0/3 nodes available",
			LastTimestamp:  older,
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "pod-b", Namespace: "default"},
		},
	)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	result, err := toolset.handleEventsTimeline(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}},
		Arguments: map[string]any{
			"namespace": "default",
			"limit":     10,
		},
	})
	if err != nil {
		t.Fatalf("handleEventsTimeline: %v", err)
	}
	timeline := result.Data.(map[string]any)["timeline"].([]map[string]any)
	if len(timeline) != 2 {
		t.Fatalf("expected 2 timeline entries, got %d", len(timeline))
	}
	if timeline[0]["reason"] != "FailedScheduling" {
		t.Fatalf("expected oldest event first, got %#v", timeline[0])
	}
}
