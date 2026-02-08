package karpenter

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHelperBranchCoverage(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"requirements": []any{
						map[string]any{"key": "zone", "operator": "In", "values": []any{"us-east-1a"}},
					},
					"taints": []any{
						map[string]any{"key": "workload", "value": "batch", "effect": "NoSchedule"},
					},
					"nodeClassRef": map[string]any{"name": "templ"},
				},
			},
			"limits": map[string]any{
				"resources": map[string]any{"cpu": "500"},
			},
			"ttlSecondsAfterEmpty":  int64(10),
			"ttlSecondsUntilExpired": int64(5),
		},
	}}
	if reqs := extractRequirements(obj); len(reqs) != 1 {
		t.Fatalf("expected template requirements")
	}
	if taints := extractTaints(obj); len(taints) != 1 {
		t.Fatalf("expected template taints")
	}
	if limits := extractLimits(obj); limits["cpu"] != "500" {
		t.Fatalf("unexpected limits: %#v", limits)
	}
	if disruption := extractDisruption(obj); disruption["ttlSecondsAfterEmpty"] == nil || disruption["ttlSecondsUntilExpired"] == nil {
		t.Fatalf("expected disruption ttls")
	}
	if ref := extractNodeClassRefFromTemplate(obj); ref["name"] != "templ" {
		t.Fatalf("expected template nodeClassRef")
	}
	if value, ok := nestedMap(nil, "spec"); value != nil || ok {
		t.Fatalf("expected nil nestedMap for nil obj")
	}
	if nestedString(nil, "spec", "foo") != "" {
		t.Fatalf("expected empty nestedString for nil obj")
	}
	if nestedInt(nil, "spec", "ttlSecondsAfterEmpty") != 0 {
		t.Fatalf("expected zero nestedInt for nil obj")
	}
	if mapKeys(map[string]any{}) != nil {
		t.Fatalf("expected nil mapKeys for empty input")
	}
	if extractProviderRef(obj) != nil {
		t.Fatalf("expected nil provider ref")
	}
	if extractNodeClassRef(obj) != nil {
		t.Fatalf("expected nil node class ref")
	}
}

func TestConditionHelpersFalseCases(t *testing.T) {
	if isConditionTrue(nil, []string{"Ready"}) {
		t.Fatalf("expected false for nil condition")
	}
	if isConditionTrue(map[string]any{"type": 123, "status": "True"}, []string{"Ready"}) {
		t.Fatalf("expected false for non-string type")
	}
	if isConditionTrue(map[string]any{"type": "Ready", "status": "False"}, []string{"Ready"}) {
		t.Fatalf("expected false for non-true status")
	}
	if isConditionFalse(map[string]any{"type": "Ready", "status": "Unknown"}, []string{"Ready"}) {
		t.Fatalf("expected false for non-false status")
	}
}

func TestPodHelpers(t *testing.T) {
	if podInList("default", "api", []string{"api"}) != true {
		t.Fatalf("expected podInList by name")
	}
	if podInList("default", "api", []string{"default/api"}) != true {
		t.Fatalf("expected podInList by namespace")
	}
	if podInList("default", "api", []string{"other"}) {
		t.Fatalf("expected podInList false")
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending"},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"},
			},
		},
	}
	reason, msg := pendingReason(pod)
	if reason != "Unschedulable" || msg != "no nodes" {
		t.Fatalf("unexpected pending reason: %s %s", reason, msg)
	}
	ready := &corev1.Pod{Status: corev1.PodStatus{Message: "waiting"}}
	reason, _ = pendingReason(ready)
	if reason == "" {
		t.Fatalf("expected fallback reason")
	}
}
