package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleRestartSafetyCheck(t *testing.T) {
	replicas := int32(3)
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ReadyReplicas: 3}},
		&policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "default"}, Status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 3, DesiredHealthy: 2, DisruptionsAllowed: 1}},
		&autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "api-hpa", Namespace: "default"}, Spec: autoscalingv2.HorizontalPodAutoscalerSpec{ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "api"}, MaxReplicas: 6}, Status: autoscalingv2.HorizontalPodAutoscalerStatus{CurrentReplicas: 3, DesiredReplicas: 3}},
	)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: client}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	result, err := toolset.handleRestartSafetyCheck(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}}, Arguments: map[string]any{"name": "api", "namespace": "default"}})
	if err != nil {
		t.Fatalf("handleRestartSafetyCheck: %v", err)
	}
	root := result.Data.(map[string]any)
	if safe, _ := root["safe"].(bool); !safe {
		t.Fatalf("expected safe=true, got %#v", root)
	}
}
