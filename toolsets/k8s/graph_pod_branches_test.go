package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestAddPodGraphOwnerBranches(t *testing.T) {
	namespace := "default"
	labels := map[string]string{"app": "demo"}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: labels, Ports: []corev1.ServicePort{{Port: 80}}},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-rs",
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "demo-deploy",
			}},
		},
		Spec: appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}},
	}
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-owner", Namespace: namespace, Labels: labels},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rs-owner",
				Namespace: namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "ReplicaSet",
					Name: "demo-rs",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ss-owner",
				Namespace: namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "StatefulSet",
					Name: "demo-ss",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ds-owner",
				Namespace: namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "DaemonSet",
					Name: "demo-ds",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deploy-owner",
				Namespace: namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Deployment",
					Name: "demo-deploy",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-owner",
				Namespace: namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Job",
					Name: "demo-job",
				}},
			},
		},
	}
	objects := []runtime.Object{service, rs}
	for _, pod := range pods {
		objects = append(objects, pod)
	}
	client := k8sfake.NewSimpleClientset(objects...)
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
	graph := newGraphBuilder()
	for _, pod := range pods {
		if _, err := toolset.addPodGraph(context.Background(), graph, namespace, pod.Name, nil); err != nil {
			t.Fatalf("addPodGraph %s: %v", pod.Name, err)
		}
	}
}
