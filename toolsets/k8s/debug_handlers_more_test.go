package k8s

import (
	"context"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleSchedulingDebugQuotaAndEvents(t *testing.T) {
	namespace := "default"
	uid := types.UID("pending-uid")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: namespace, UID: uid},
		Spec: corev1.PodSpec{
			PriorityClassName: "high",
		},
		Status: corev1.PodStatus{
			Phase:             corev1.PodPending,
			NominatedNodeName: "node-1",
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"},
			},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-event", Namespace: namespace},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace,
			Name:      pod.Name,
			UID:       uid,
		},
		Reason:  "Preempted",
		Message: "exceeded quota",
		Type:    corev1.EventTypeWarning,
	}
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: namespace},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			Used: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		},
	}
	limit := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "lr", Namespace: namespace},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{Type: corev1.LimitTypeContainer, Min: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")}},
			},
		},
	}
	priority := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{Name: "high"},
		Value:      1000,
	}

	client := fake.NewSimpleClientset(pod, event, quota, limit, priority)
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

	if _, err := toolset.handleSchedulingDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleSchedulingDebug: %v", err)
	}
}

func TestHandleHPADebugBranches(t *testing.T) {
	namespace := "default"
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "hpa", Namespace: namespace},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MaxReplicas: 3,
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 1,
			DesiredReplicas: 2,
			Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{Type: autoscalingv2.ScalingActive, Status: corev1.ConditionFalse, Message: "no metrics"},
			},
		},
	}
	client := fake.NewSimpleClientset(hpa)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	if _, err := toolset.handleHPADebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "name": "hpa"},
	}); err != nil {
		t.Fatalf("handleHPADebug name: %v", err)
	}

	toolsetEmpty := New()
	_ = toolsetEmpty.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: fake.NewSimpleClientset()},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolsetEmpty.handleHPADebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleHPADebug empty: %v", err)
	}
}

func TestHandleNetworkAndPrivateLinkDebug(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "api"},
			Ports:    []corev1.ServicePort{{Port: 80}},
			Type:     corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: map[string]string{"app": "api"}},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	endpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace}}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}
	client := fake.NewSimpleClientset(service, pod, endpoints, netpol)
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

	if _, err := toolset.handleNetworkDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "service": "api"},
	}); err != nil {
		t.Fatalf("handleNetworkDebug: %v", err)
	}

	if _, err := toolset.handlePrivateLinkDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "service": "api"},
	}); err != nil {
		t.Fatalf("handlePrivateLinkDebug: %v", err)
	}
}
