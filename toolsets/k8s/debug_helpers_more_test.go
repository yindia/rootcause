package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDebugHelpers(t *testing.T) {
	classes := []schedulingv1.PriorityClass{
		{ObjectMeta: metav1.ObjectMeta{Name: "high"}},
	}
	if findPriorityClass(classes, "high") == nil {
		t.Fatalf("expected priority class")
	}
	if findPriorityClass(classes, "missing") != nil {
		t.Fatalf("expected no priority class")
	}
	events := []corev1.Event{
		{Reason: "Preempted", Message: "pod was preempted"},
	}
	if !hasPreemptionEvent(events) {
		t.Fatalf("expected preemption event")
	}
	if hasPreemptionEvent([]corev1.Event{{Reason: "Scheduled", Message: "ok"}}) {
		t.Fatalf("expected no preemption event")
	}
}
