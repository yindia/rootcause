package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleDiagnoseCrashLoopAndPending(t *testing.T) {
	crashPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-crash",
			Namespace: "default",
			UID:       "uid-crash",
			Labels:    map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{
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
			Name:      "api-pending",
			Namespace: "default",
			UID:       "uid-pending",
			Labels:    map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: "no nodes available",
				},
			},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-crash-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{UID: crashPod.UID},
		Reason:         "BackOff",
		Message:        "Back-off restarting failed container",
	}

	client := k8sfake.NewSimpleClientset(crashPod, pendingPod, event)
	cfg := config.DefaultConfig()
	clients := &kube.Clients{Typed: client}
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
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"keyword": "api"},
	})
	if err != nil {
		t.Fatalf("handleDiagnose: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["evidence"] == nil {
		t.Fatalf("expected evidence output, got %#v", result.Data)
	}
}
