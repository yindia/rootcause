package kube

import (
	"context"
	"errors"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

func GroupsPresent(discoveryClient discovery.DiscoveryInterface, groupNames []string) (bool, []string, error) {
	if discoveryClient == nil {
		return false, nil, errors.New("missing discovery client")
	}
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, nil, err
	}
	found := map[string]struct{}{}
	for _, group := range groups.Groups {
		for _, name := range groupNames {
			if group.Name == name {
				found[group.Name] = struct{}{}
			}
		}
	}
	list := make([]string, 0, len(found))
	for name := range found {
		list = append(list, name)
	}
	sort.Strings(list)
	return len(list) > 0, list, nil
}

func ControlPlaneNamespaces(ctx context.Context, clients *Clients, selectors []string) ([]string, error) {
	if clients == nil || clients.Typed == nil {
		return nil, errors.New("missing typed client")
	}
	found := map[string]struct{}{}
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		deployments, err := clients.Typed.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, err
		}
		for _, deploy := range deployments.Items {
			found[deploy.Namespace] = struct{}{}
		}
	}
	if len(found) == 0 {
		for _, selector := range selectors {
			if selector == "" {
				continue
			}
			pods, err := clients.Typed.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				return nil, err
			}
			for _, pod := range pods.Items {
				found[pod.Namespace] = struct{}{}
			}
		}
	}
	list := make([]string, 0, len(found))
	for ns := range found {
		list = append(list, ns)
	}
	sort.Strings(list)
	return list, nil
}
