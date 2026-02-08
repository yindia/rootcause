package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestHandleDiagnoseCrashloopEvents(t *testing.T) {
	namespace := "default"
	crashUID := types.UID("crash-uid")
	pendingUID := types.UID("pending-uid")
	crashPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-crash", Namespace: namespace, UID: crashUID, Labels: map[string]string{"app": "api"}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
		},
	}
	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pending", Namespace: namespace, UID: pendingUID, Labels: map[string]string{"app": "api"}},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"},
			},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "crash-event", Namespace: namespace},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace,
			Name:      crashPod.Name,
			UID:       crashUID,
		},
		Reason:  "BackOff",
		Message: "Back-off restarting failed container",
		Type:    corev1.EventTypeWarning,
	}

	client := fake.NewSimpleClientset(crashPod, pendingPod, event)
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
			"keyword":   "api",
			"namespace": namespace,
		},
	})
	if err != nil {
		t.Fatalf("handleDiagnose crashloop: %v", err)
	}
}

func TestHandleDiagnoseNamespaceDenied(t *testing.T) {
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

	if _, err := toolset.handleDiagnose(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"other"}},
		Arguments: map[string]any{
			"keyword":   "api",
			"namespace": "default",
		},
	}); err == nil {
		t.Fatalf("expected namespace enforcement error")
	}
}
