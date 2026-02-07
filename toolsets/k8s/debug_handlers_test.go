package k8s

import (
	"context"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newDebugToolset(objects ...runtime.Object) *Toolset {
	client := fake.NewSimpleClientset(objects...)
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
	return toolset
}

func TestDebugHandlers(t *testing.T) {
	namespace := "default"
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}

	crashPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crash", Namespace: namespace},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending",
			Namespace: namespace,
			Labels:    map[string]string{"app": "demo"},
		},
		Spec: corev1.PodSpec{
			PriorityClassName: "high",
			PreemptionPolicy:  func() *corev1.PreemptionPolicy { v := corev1.PreemptLowerPriority; return &v }(),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"},
			},
		},
	}
	readyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ready",
			Namespace: namespace,
			Labels:    map[string]string{"app": "demo"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: namespace},
		Reason:     "Preempted",
		Message:    "exceeded quota",
	}

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: namespace},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("1")},
			Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("1")},
		},
	}
	limit := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "lr", Namespace: namespace},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{Type: corev1.LimitTypeContainer, Min: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}},
			},
		},
	}
	priority := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{Name: "high"},
		Value:      1000,
	}
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "hpa", Namespace: namespace},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 1,
			DesiredReplicas: 2,
			Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{Type: autoscalingv2.ScalingActive, Status: corev1.ConditionFalse, Message: "no metrics"},
			},
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "demo"},
			Type:     corev1.ServiceTypeLoadBalancer,
		},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: namespace},
	}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}

	toolset := newDebugToolset(ns, node, crashPod, pendingPod, readyPod, event, quota, limit, priority, hpa, service, endpoints, netpol)
	user := policy.User{Role: policy.RoleCluster}

	if _, err := toolset.handleOverview(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if _, err := toolset.handleCrashloopDebug(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleCrashloopDebug: %v", err)
	}
	if _, err := toolset.handleSchedulingDebug(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleSchedulingDebug: %v", err)
	}
	if _, err := toolset.handleHPADebug(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleHPADebug: %v", err)
	}
	if _, err := toolset.handleNetworkDebug(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": namespace, "service": "demo"},
	}); err != nil {
		t.Fatalf("handleNetworkDebug: %v", err)
	}
	if _, err := toolset.handlePrivateLinkDebug(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": namespace, "service": "demo"},
	}); err != nil {
		t.Fatalf("handlePrivateLinkDebug: %v", err)
	}
}
