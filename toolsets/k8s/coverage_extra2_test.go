package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
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

func TestHandleDiagnoseWithEventsAndCrashLoop(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: "default",
			UID:       "pod-uid",
			Labels:    map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "api-event", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "api-1",
			UID:       "pod-uid",
		},
		Reason:  "Failed",
		Message: "boom",
	}
	client := k8sfake.NewSimpleClientset(pod, event)
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
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"keyword": "api",
		},
	}); err != nil {
		t.Fatalf("handleDiagnose with events: %v", err)
	}
}

func TestDiscoveryErrorIsPartialNil(t *testing.T) {
	if discoveryErrorIsPartial(nil) {
		t.Fatalf("expected false for nil error")
	}
}

func TestOwnedByFalse(t *testing.T) {
	meta := metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}}}
	if ownedBy(&meta, "StatefulSet", "db") {
		t.Fatalf("expected ownedBy false")
	}
}

func TestAddStatefulSetGraphServiceMissing(t *testing.T) {
	labels := map[string]string{"app": "db"}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "missing",
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template:    corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	client := k8sfake.NewSimpleClientset(sts, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: stubCollector{},
	})
	cache, _ := toolset.buildGraphCache(context.Background(), "default", true)
	graph := newGraphBuilder()
	if _, err := toolset.addStatefulSetGraph(context.Background(), graph, "default", "db", cache); err != nil {
		t.Fatalf("addStatefulSetGraph missing service: %v", err)
	}
}

func TestAddAWSRoleEvidenceSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	_ = reg.Add(mcp.ToolSpec{
		Name:      "aws.iam.get_role",
		ToolsetID: "aws",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{
				"assumeRolePolicy": map[string]any{
					"Statement": []any{
						map[string]any{"Principal": map[string]any{"AWS": "system:serviceaccount:default:app"}},
					},
				},
			}}, nil
		},
	})
	toolset := New()
	toolCtx := mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Registry: reg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}
	toolCtx.Invoker = mcp.NewToolInvoker(reg, mcp.ToolContext(toolCtx))
	_ = toolset.Init(toolCtx)

	analysis := render.NewAnalysis()
	toolset.addAWSRoleEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, "role", "us-east-1", "default", "app")
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected aws role evidence")
	}
}
