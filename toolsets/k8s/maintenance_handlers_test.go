package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestHandleCleanupPods(t *testing.T) {
	namespace := "default"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: namespace},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
		},
	}
	client := fake.NewSimpleClientset(pod)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Typed: client},
		Policy:  policy.NewAuthorizer(),
	})
	_, err := toolset.handleCleanupPods(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace},
	})
	if err != nil {
		t.Fatalf("handleCleanupPods: %v", err)
	}
}

func TestHandleNodeManagementDrain(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}
	client := fake.NewSimpleClientset(node, pod)
	client.PrependReactor("create", "evictions", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &policyv1.Eviction{}, nil
	})
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Typed: client},
		Policy:  policy.NewAuthorizer(),
	})
	_, err := toolset.handleNodeManagement(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"action":             "drain",
			"nodeName":           "node-1",
			"gracePeriodSeconds": float64(1),
			"force":              true,
		},
	})
	if err != nil {
		t.Fatalf("handleNodeManagement drain: %v", err)
	}
}

func TestHandleNodeManagementCordon(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}}
	client := fake.NewSimpleClientset(node)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Typed: client},
		Policy:  policy.NewAuthorizer(),
	})
	_, err := toolset.handleNodeManagement(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"action":   "cordon",
			"nodeName": "node-2",
		},
	})
	if err != nil {
		t.Fatalf("handleNodeManagement cordon: %v", err)
	}
}
