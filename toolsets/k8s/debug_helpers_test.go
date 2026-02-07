package k8s

import (
	"context"
	"testing"

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

func TestEventHelpers(t *testing.T) {
	events := []corev1.Event{{Reason: "Preempted", Message: "preempted"}, {Message: "exceeded quota"}}
	if len(summarizeEvents(events)) != 2 {
		t.Fatalf("expected summarized events")
	}
	if !hasPreemptionEvent(events) {
		t.Fatalf("expected preemption event")
	}
	if !hasQuotaEvent(events) {
		t.Fatalf("expected quota event")
	}
}

func TestPodEvents(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default", UID: "pod-uid"}}
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "default"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}}
	client := k8sfake.NewSimpleClientset(event)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	list, err := podEvents(context.Background(), toolset, pod)
	if err != nil {
		t.Fatalf("podEvents: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected events")
	}
}
