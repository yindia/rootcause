package k8s

import (
	"context"
	"errors"
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

func TestHandleNodeManagementDrainSkipsAndForce(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	daemonPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "daemon",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet"},
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-1"},
	}
	mirrorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mirror",
			Namespace:   "default",
			Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "true"},
		},
		Spec: corev1.PodSpec{NodeName: "node-1"},
	}
	workloadPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}

	client := fake.NewSimpleClientset(node, daemonPod, mirrorPod, workloadPod)
	client.PrependReactor("create", "evictions", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &policyv1.Eviction{}, errors.New("eviction failed")
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Typed: client},
		Policy:  policy.NewAuthorizer(),
	})

	if _, err := toolset.handleNodeManagement(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"action":   "drain",
			"nodeName": "node-1",
			"force":    true,
		},
	}); err != nil {
		t.Fatalf("handleNodeManagement drain: %v", err)
	}
}
