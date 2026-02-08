package evidence

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
	k8stesting "k8s.io/client-go/testing"

	"rootcause/internal/kube"
)

func TestEventsForObject(t *testing.T) {
	ctx := context.Background()
	uid := "12345"
	obj := &unstructured.Unstructured{}
	obj.SetNamespace("default")
	obj.SetUID(types.UID(uid))

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "e1", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{
			UID: types.UID(uid),
		},
	}
	client := fake.NewSimpleClientset(event)
	collector := NewCollector(&kube.Clients{Typed: client})
	events, err := collector.EventsForObject(ctx, obj)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestEventsForObjectNilAndMissingFields(t *testing.T) {
	ctx := context.Background()
	collector := NewCollector(&kube.Clients{Typed: fake.NewSimpleClientset()})
	events, err := collector.EventsForObject(ctx, nil)
	if err != nil {
		t.Fatalf("events nil: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for nil obj")
	}
	obj := &unstructured.Unstructured{}
	obj.SetNamespace("default")
	events, err = collector.EventsForObject(ctx, obj)
	if err != nil {
		t.Fatalf("events missing uid: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for missing uid")
	}
	obj = &unstructured.Unstructured{}
	obj.SetUID(types.UID("1"))
	events, err = collector.EventsForObject(ctx, obj)
	if err != nil {
		t.Fatalf("events missing namespace: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for missing namespace")
	}
}

func TestOwnerChain(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
	}
	deployU := &unstructured.Unstructured{}
	deployU.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	deployU.SetAPIVersion("apps/v1")
	deployU.SetKind("Deployment")

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "DeploymentList",
	}, deployU)

	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "deployments", Kind: "Deployment", Namespaced: true}},
			},
		},
	})

	pod := &unstructured.Unstructured{}
	pod.SetNamespace("default")
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "demo",
		},
	})

	collector := NewCollector(&kube.Clients{Dynamic: dynamicClient, Mapper: mapper})
	chain, err := collector.OwnerChain(ctx, pod)
	if err != nil {
		t.Fatalf("owner chain: %v", err)
	}
	if len(chain) != 1 || chain[0] != "Deployment/demo" {
		t.Fatalf("unexpected chain: %#v", chain)
	}
}

func TestOwnerChainMissingMapper(t *testing.T) {
	ctx := context.Background()
	pod := &unstructured.Unstructured{}
	pod.SetNamespace("default")
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: "demo"},
	})
	collector := NewCollector(&kube.Clients{})
	chain, err := collector.OwnerChain(ctx, pod)
	if err != nil {
		t.Fatalf("owner chain: %v", err)
	}
	if len(chain) != 1 {
		t.Fatalf("expected chain entry even when mapper missing")
	}
}

func TestOwnerChainNoOwners(t *testing.T) {
	ctx := context.Background()
	pod := &unstructured.Unstructured{}
	collector := NewCollector(&kube.Clients{})
	chain, err := collector.OwnerChain(ctx, pod)
	if err != nil {
		t.Fatalf("owner chain: %v", err)
	}
	if len(chain) != 0 {
		t.Fatalf("expected empty chain, got %#v", chain)
	}
}

func TestPodStatusSummary(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: true,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				},
			},
		},
	}
	collector := NewCollector(nil)
	summary := collector.PodStatusSummary(pod)
	if summary["phase"] != corev1.PodRunning {
		t.Fatalf("expected phase running")
	}
	if summary["ready"] != corev1.ConditionTrue {
		t.Fatalf("expected ready true")
	}
}

func TestPodStatusSummaryNil(t *testing.T) {
	collector := NewCollector(nil)
	summary := collector.PodStatusSummary(nil)
	if len(summary) != 0 {
		t.Fatalf("expected empty summary for nil")
	}
}

func TestRelatedPodsAndEndpoints(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default", Labels: map[string]string{"app": "demo"}},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
	}
	client := fake.NewSimpleClientset(pod, endpoints)
	collector := NewCollector(&kube.Clients{Typed: client})
	selector := labels.SelectorFromSet(labels.Set{"app": "demo"})

	pods, err := collector.RelatedPods(ctx, "default", selector)
	if err != nil {
		t.Fatalf("related pods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	ep, err := collector.EndpointsForService(ctx, "default", "svc")
	if err != nil {
		t.Fatalf("endpoints: %v", err)
	}
	if ep.Name != "svc" {
		t.Fatalf("expected endpoints svc, got %s", ep.Name)
	}
}

func TestRelatedPodsAndEndpointsErrors(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("list fail")
	})
	client.PrependReactor("get", "endpoints", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("get fail")
	})
	collector := NewCollector(&kube.Clients{Typed: client})
	selector := labels.SelectorFromSet(labels.Set{"app": "demo"})
	if _, err := collector.RelatedPods(ctx, "default", selector); err == nil {
		t.Fatalf("expected related pods error")
	}
	if _, err := collector.EndpointsForService(ctx, "default", "svc"); err == nil {
		t.Fatalf("expected endpoints error")
	}
}

func TestResourceRefAndStatusFromUnstructured(t *testing.T) {
	collector := NewCollector(nil)
	ref := collector.ResourceRef(schema.GroupVersionResource{Resource: "pods"}, "default", "p1")
	if ref != "pods/default/p1" {
		t.Fatalf("unexpected ref: %s", ref)
	}
	ref = collector.ResourceRef(schema.GroupVersionResource{Resource: "pods"}, "", "p1")
	if ref != "pods/p1" {
		t.Fatalf("unexpected ref for cluster: %s", ref)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"phase": "Running"}}}
	status := StatusFromUnstructured(obj)
	if status["phase"] != "Running" {
		t.Fatalf("expected status phase Running")
	}
	status = StatusFromUnstructured(nil)
	if len(status) != 0 {
		t.Fatalf("expected empty status for nil")
	}
	obj3 := &unstructured.Unstructured{Object: map[string]any{}}
	status = StatusFromUnstructured(obj3)
	if len(status) != 0 {
		t.Fatalf("expected empty status for missing status")
	}
	obj2 := &unstructured.Unstructured{Object: map[string]any{"status": "ok"}}
	status2 := StatusFromUnstructured(obj2)
	if status2["raw"] != "ok" {
		t.Fatalf("expected raw status")
	}
}
