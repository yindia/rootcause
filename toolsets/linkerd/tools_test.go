package linkerd

import (
	"context"
	"io"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/audit"
	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type fakeCollector struct {
	eventsCalled bool
}

func (f *fakeCollector) EventsForObject(ctx context.Context, obj *unstructured.Unstructured) ([]corev1.Event, error) {
	f.eventsCalled = true
	return nil, nil
}

func (f *fakeCollector) OwnerChain(ctx context.Context, obj *unstructured.Unstructured) ([]string, error) {
	return []string{"Deployment/demo"}, nil
}

func (f *fakeCollector) PodStatusSummary(pod *corev1.Pod) map[string]any {
	return map[string]any{"phase": pod.Status.Phase}
}

func (f *fakeCollector) RelatedPods(ctx context.Context, namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	return nil, nil
}

func (f *fakeCollector) EndpointsForService(ctx context.Context, namespace, name string) (*corev1.Endpoints, error) {
	return &corev1.Endpoints{}, nil
}

func (f *fakeCollector) ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string {
	return "pods/" + namespace + "/" + name
}

func TestProxyStatusUsesSharedEvidence(t *testing.T) {
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "linkerd-proxy"}}},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		},
	}
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, pod)

	collector := &fakeCollector{}
	renderer := render.NewRenderer()
	cfg := config.DefaultConfig()
	authorizer := policy.NewAuthorizer()
	toolCtx := mcp.ToolContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   authorizer,
		Evidence: collector,
		Renderer: renderer,
		Redactor: redact.New(),
		Audit:    audit.NewLogger(io.Discard),
	}
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext(toolCtx)); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	result, err := toolset.handleProxyStatus(ctx, mcp.ToolRequest{
		Arguments: map[string]any{"namespace": "default"},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus failed: %v", err)
	}
	if !collector.eventsCalled {
		t.Fatalf("expected EventsForObject to be called via shared describe helper")
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result.Data)
	}
	if _, ok := data["likelyRootCauses"]; !ok {
		t.Fatalf("expected likelyRootCauses in output")
	}
}
