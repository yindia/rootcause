package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
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
