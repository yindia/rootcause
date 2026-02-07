package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/kube"
)

func TestParsePortSpec(t *testing.T) {
	local, remote, err := parsePortSpec("8080")
	if err != nil || local != "8080" || remote != "8080" {
		t.Fatalf("unexpected parse: %v %s %s", err, local, remote)
	}
	local, remote, err = parsePortSpec("8080:80")
	if err != nil || local != "8080" || remote != "80" {
		t.Fatalf("unexpected parse: %v %s %s", err, local, remote)
	}
	if _, _, err := parsePortSpec(""); err == nil {
		t.Fatalf("expected error for empty spec")
	}
}

func TestParsePortNumber(t *testing.T) {
	if value, ok := parsePortNumber("80"); !ok || value != 80 {
		t.Fatalf("expected numeric parse")
	}
	if _, ok := parsePortNumber("http"); ok {
		t.Fatalf("expected non-numeric to fail")
	}
}

func TestResolveTargetPort(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}}}}}
	port := corev1.ServicePort{Port: 80, TargetPort: intstr.FromString("http")}
	resolved, err := resolveTargetPort(port, pod)
	if err != nil || resolved != 8080 {
		t.Fatalf("expected named port resolution, got %d (%v)", resolved, err)
	}
	port = corev1.ServicePort{Port: 90, TargetPort: intstr.FromInt(9090)}
	resolved, err = resolveTargetPort(port, pod)
	if err != nil || resolved != 9090 {
		t.Fatalf("expected int port resolution, got %d (%v)", resolved, err)
	}
}

func TestResolveServicePortMappings(t *testing.T) {
	namespace := "default"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromString("http")}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}}}},
	}
	client := fake.NewSimpleClientset(svc, pod)
	toolset := New()
	toolset.ctx.Clients = &kube.Clients{Typed: client}

	resolved, mappings, err := toolset.resolveServicePortMappings(context.Background(), namespace, "api", "api-1", []string{"10443:http"})
	if err != nil {
		t.Fatalf("resolveServicePortMappings: %v", err)
	}
	if len(resolved) != 1 || resolved[0] != "10443:8080" {
		t.Fatalf("unexpected resolved ports: %#v", resolved)
	}
	if len(mappings) != 1 || mappings[0].ServicePort != 80 || mappings[0].TargetPort != 8080 {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestResolvePodForService(t *testing.T) {
	namespace := "default"
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1"}}},
		}},
	}
	client := fake.NewSimpleClientset(endpoints)
	toolset := New()
	toolset.ctx.Clients = &kube.Clients{Typed: client}

	podName, err := toolset.resolvePodForService(context.Background(), namespace, "api")
	if err != nil || podName != "api-1" {
		t.Fatalf("resolvePodForService: %v %s", err, podName)
	}
}
