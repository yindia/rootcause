package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestDebugHelperBranches(t *testing.T) {
	event := corev1.Event{Reason: "Preempted", Message: "exceeded quota"}
	if !hasPreemptionEvent([]corev1.Event{event}) {
		t.Fatalf("expected preemption event")
	}
	if !hasQuotaEvent([]corev1.Event{event}) {
		t.Fatalf("expected quota event")
	}
	if hasQuotaEvent([]corev1.Event{{Message: "other"}}) {
		t.Fatalf("did not expect quota event")
	}

	pod := &corev1.Pod{Status: corev1.PodStatus{}}
	reason, _ := pendingReason(pod)
	if reason != "Pending" {
		t.Fatalf("expected Pending reason")
	}

	classes := []schedulingv1.PriorityClass{{ObjectMeta: metav1.ObjectMeta{Name: "high"}, Value: 1000}}
	if findPriorityClass(classes, "high") == nil {
		t.Fatalf("expected priority class match")
	}
	if findPriorityClass(classes, "") != nil {
		t.Fatalf("expected nil for empty name")
	}

	netpol := networkingv1.NetworkPolicy{
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"bad key": "value"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
		},
	}
	if selected := policiesSelectingPod([]networkingv1.NetworkPolicy{netpol}, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}}}); len(selected) != 0 {
		t.Fatalf("expected invalid selector to be skipped")
	}
	if ingressBlocked, egressBlocked := networkPolicyBlockStatus([]networkingv1.NetworkPolicy{netpol}); ingressBlocked || egressBlocked {
		t.Fatalf("expected blocked false when ingress rules exist")
	}

	toolset := New()
	cfg := config.DefaultConfig()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	cache := newGraphCache()
	cache.podsLoaded = true
	if pods, err := toolset.podsForSelector(context.Background(), "default", labels.Everything(), cache); err != nil || len(pods) != 0 {
		t.Fatalf("expected podsForSelector with empty cache to return none")
	}
}
