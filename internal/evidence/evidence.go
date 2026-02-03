package evidence

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"rootcause/internal/kube"
)

type Collector interface {
	EventsForObject(ctx context.Context, obj *unstructured.Unstructured) ([]corev1.Event, error)
	OwnerChain(ctx context.Context, obj *unstructured.Unstructured) ([]string, error)
	PodStatusSummary(pod *corev1.Pod) map[string]any
	RelatedPods(ctx context.Context, namespace string, selector labels.Selector) ([]corev1.Pod, error)
	EndpointsForService(ctx context.Context, namespace, name string) (*corev1.Endpoints, error)
	ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string
}

type KubeCollector struct {
	clients *kube.Clients
}

func NewCollector(clients *kube.Clients) *KubeCollector {
	return &KubeCollector{clients: clients}
}

func (c *KubeCollector) EventsForObject(ctx context.Context, obj *unstructured.Unstructured) ([]corev1.Event, error) {
	if obj == nil {
		return nil, nil
	}
	uid := string(obj.GetUID())
	ns := obj.GetNamespace()
	if uid == "" || ns == "" {
		return nil, nil
	}
	list, err := c.clients.Typed.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s", uid),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *KubeCollector) OwnerChain(ctx context.Context, obj *unstructured.Unstructured) ([]string, error) {
	var chain []string
	current := obj
	for i := 0; i < 4; i++ {
		if current == nil {
			break
		}
		owners := current.GetOwnerReferences()
		if len(owners) == 0 {
			break
		}
		owner := owners[0]
		chain = append(chain, fmt.Sprintf("%s/%s", owner.Kind, owner.Name))
		gvr, _, err := kube.ResolveResource(c.clients.Mapper, owner.APIVersion, owner.Kind, "")
		if err != nil {
			break
		}
		resource, err := c.clients.Dynamic.Resource(gvr).Namespace(current.GetNamespace()).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		current = resource
	}
	return chain, nil
}

func (c *KubeCollector) PodStatusSummary(pod *corev1.Pod) map[string]any {
	summary := map[string]any{}
	if pod == nil {
		return summary
	}
	summary["phase"] = pod.Status.Phase
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			summary["ready"] = condition.Status
			break
		}
	}
	var containerStates []map[string]any
	for _, cs := range pod.Status.ContainerStatuses {
		state := map[string]any{"name": cs.Name, "ready": cs.Ready}
		if cs.State.Waiting != nil {
			state["waiting"] = cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil {
			state["terminated"] = cs.State.Terminated.Reason
		}
		containerStates = append(containerStates, state)
	}
	summary["containers"] = containerStates
	return summary
}

func (c *KubeCollector) RelatedPods(ctx context.Context, namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	list, err := c.clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *KubeCollector) EndpointsForService(ctx context.Context, namespace, name string) (*corev1.Endpoints, error) {
	endpoints, err := c.clients.Typed.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

func (c *KubeCollector) ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", gvr.Resource, name)
	}
	return fmt.Sprintf("%s/%s/%s", gvr.Resource, namespace, name)
}

func StatusFromUnstructured(obj *unstructured.Unstructured) map[string]any {
	if obj == nil {
		return map[string]any{}
	}
	status, ok := obj.Object["status"]
	if !ok {
		return map[string]any{}
	}
	if m, ok := status.(map[string]any); ok {
		return m
	}
	return map[string]any{"raw": status}
}
