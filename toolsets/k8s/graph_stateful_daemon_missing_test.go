package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddStatefulSetGraphMissingResources(t *testing.T) {
	namespace := "default"
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "missing-svc",
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "db"}},
			},
		},
	}
	cache := newGraphCache()
	cache.statefulsetsLoaded = true
	cache.statefulsets[ss.Name] = ss
	cache.podsLoaded = true
	cache.podList = []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: namespace, Labels: map[string]string{"app": "db"}}},
	}
	cache.servicesLoaded = true
	cache.endpointsLoaded = true

	toolset := New()
	graph := newGraphBuilder()
	warnings, err := toolset.addStatefulSetGraph(context.Background(), graph, namespace, "db", cache)
	if err != nil {
		t.Fatalf("add statefulset graph: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected warnings for missing resources")
	}
}

func TestAddDaemonSetGraphMissingPods(t *testing.T) {
	namespace := "default"
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: namespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "agent"}},
			},
		},
	}
	cache := newGraphCache()
	cache.daemonsetsLoaded = true
	cache.daemonsets[ds.Name] = ds
	cache.podsLoaded = true
	cache.podList = []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: namespace, Labels: map[string]string{"app": "agent"}}},
	}
	cache.servicesLoaded = true

	toolset := New()
	graph := newGraphBuilder()
	warnings, err := toolset.addDaemonSetGraph(context.Background(), graph, namespace, "agent", cache)
	if err != nil {
		t.Fatalf("add daemonset graph: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected warnings for missing pods")
	}
}
