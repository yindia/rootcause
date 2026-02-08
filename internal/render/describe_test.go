package render

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"rootcause/internal/redact"
)

type fakeCollector struct{}

func (f *fakeCollector) EventsForObject(ctx context.Context, obj *unstructured.Unstructured) ([]corev1.Event, error) {
	return []corev1.Event{{ObjectMeta: metav1.ObjectMeta{Name: "e1"}}}, nil
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
	return nil, nil
}

func (f *fakeCollector) ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string {
	return gvr.Resource + "/" + name
}

func TestDescribeAnalysisIncludesEvidence(t *testing.T) {
	secret := &unstructured.Unstructured{}
	secret.SetKind("Secret")
	secret.SetNamespace("default")
	secret.SetName("demo")
	secret.Object = map[string]any{
		"data": map[string]any{"token": "abcd"},
	}
	analysis := DescribeAnalysis(context.Background(), &fakeCollector{}, redact.New(), schema.GroupVersionResource{Resource: "secrets"}, secret)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected evidence")
	}
	if len(analysis.ResourcesExamined) == 0 {
		t.Fatalf("expected resource reference")
	}
}

func TestDescribeAnalysisNilObject(t *testing.T) {
	analysis := DescribeAnalysis(context.Background(), &fakeCollector{}, redact.New(), schema.GroupVersionResource{Resource: "pods"}, nil)
	if len(analysis.Evidence) == 0 || analysis.Evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence for nil object")
	}
}

func TestRedactObjectSecretStringData(t *testing.T) {
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"stringData": map[string]any{"password": "secret"},
	}}
	result := redactObject(nil, secret)
	if result["stringData"] != "[REDACTED]" {
		t.Fatalf("expected stringData redacted")
	}
}
