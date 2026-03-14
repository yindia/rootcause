package k8s

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/mcp"
)

func ingressesForService(ctx context.Context, t *Toolset, namespace, serviceName string) []map[string]any {
	if t == nil || t.ctx.Clients == nil || t.ctx.Clients.Typed == nil {
		return nil
	}
	list, err := t.ctx.Clients.Typed.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	var out []map[string]any
	for _, ing := range list.Items {
		if !ingressTargetsService(&ing, serviceName) {
			continue
		}
		out = append(out, summarizeIngress(ing))
	}
	return out
}

func ingressTargetsService(ing *networkingv1.Ingress, serviceName string) bool {
	if ing == nil {
		return false
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil && ing.Spec.DefaultBackend.Service.Name == serviceName {
		return true
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil && path.Backend.Service.Name == serviceName {
				return true
			}
		}
	}
	return false
}

func summarizeIngress(ing networkingv1.Ingress) map[string]any {
	var hosts []string
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	annotations := map[string]string{}
	for _, key := range []string{
		"kubernetes.io/ingress.class",
		"alb.ingress.kubernetes.io/load-balancer-name",
		"alb.ingress.kubernetes.io/load-balancer-arn",
	} {
		if val, ok := ing.Annotations[key]; ok {
			annotations[key] = val
		}
	}
	return map[string]any{
		"name":        ing.Name,
		"class":       ing.Spec.IngressClassName,
		"hosts":       hosts,
		"annotations": annotations,
		"ingress":     ing.Status.LoadBalancer.Ingress,
	}
}

func (t *Toolset) awsLoadBalancerEvidence(ctx context.Context, req mcp.ToolRequest, service *corev1.Service, ingresses []map[string]any, awsRegion string) ([]map[string]any, []string) {
	var warnings []string
	if t == nil || t.ctx.Registry == nil {
		return nil, []string{"tool registry unavailable"}
	}
	if service == nil {
		return nil, nil
	}
	hostnames, names, arns := extractLBIdentifiers(service, ingresses)
	if awsRegion == "" {
		for _, host := range hostnames {
			if region := regionFromHost(host); region != "" {
				awsRegion = region
				break
			}
		}
	}

	args := map[string]any{
		"region": awsRegion,
		"limit":  100,
	}
	if len(arns) > 0 {
		args["loadBalancerArns"] = arns
	} else if len(names) > 0 {
		args["names"] = names
	}

	result, err := t.ctx.CallTool(ctx, req.User, "aws.ec2.list_load_balancers", args)
	if err != nil {
		return nil, []string{err.Error()}
	}
	loadBalancers := loadBalancersFromResult(result.Data)
	if len(hostnames) > 0 {
		loadBalancers = filterLoadBalancersByHost(loadBalancers, hostnames)
	}
	if len(loadBalancers) == 0 {
		return nil, []string{"no matching load balancers found"}
	}

	var evidence []map[string]any
	for i, lb := range loadBalancers {
		if i >= 3 {
			break
		}
		item := map[string]any{"loadBalancer": lb}
		lbArn := toString(lb["arn"])
		if lbArn != "" {
			tgResult, err := t.ctx.CallTool(ctx, req.User, "aws.ec2.list_target_groups", map[string]any{
				"loadBalancerArn": lbArn,
				"region":          awsRegion,
				"limit":           20,
			})
			if err != nil {
				item["targetGroupError"] = err.Error()
			} else {
				targetGroups := targetGroupsFromResult(tgResult.Data)
				item["targetGroups"] = targetGroups
				var targetHealth []map[string]any
				for _, group := range targetGroups {
					groupArn := toString(group["arn"])
					if groupArn == "" {
						continue
					}
					health, err := t.ctx.CallTool(ctx, req.User, "aws.ec2.get_target_health", map[string]any{
						"targetGroupArn": groupArn,
						"region":         awsRegion,
					})
					if err != nil {
						targetHealth = append(targetHealth, map[string]any{"targetGroupArn": groupArn, "error": err.Error()})
						continue
					}
					targetHealth = append(targetHealth, map[string]any{
						"targetGroupArn": groupArn,
						"health":         health.Data,
					})
				}
				if len(targetHealth) > 0 {
					item["targetHealth"] = targetHealth
				}
			}
		}
		evidence = append(evidence, item)
	}
	return evidence, warnings
}

func extractLBIdentifiers(service *corev1.Service, ingresses []map[string]any) (hosts, names, arns []string) {
	if service == nil {
		return nil, nil, nil
	}
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" {
			hosts = append(hosts, ingress.Hostname)
		}
	}
	if ann := service.Annotations; ann != nil {
		if name := strings.TrimSpace(ann["service.beta.kubernetes.io/aws-load-balancer-name"]); name != "" {
			names = append(names, name)
		}
		if arn := strings.TrimSpace(ann["service.beta.kubernetes.io/aws-load-balancer-arn"]); arn != "" {
			arns = append(arns, arn)
		}
	}
	for _, ing := range ingresses {
		ann, ok := ing["annotations"].(map[string]string)
		if ok {
			if name := strings.TrimSpace(ann["alb.ingress.kubernetes.io/load-balancer-name"]); name != "" {
				names = append(names, name)
			}
			if arn := strings.TrimSpace(ann["alb.ingress.kubernetes.io/load-balancer-arn"]); arn != "" {
				arns = append(arns, arn)
			}
		}
		if lb, ok := ing["ingress"].([]corev1.LoadBalancerIngress); ok {
			for _, entry := range lb {
				if entry.Hostname != "" {
					hosts = append(hosts, entry.Hostname)
				}
			}
		}
	}
	return uniqueStrings(hosts), uniqueStrings(names), uniqueStrings(arns)
}

func loadBalancersFromResult(data any) []map[string]any {
	payload, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	items, ok := payload["loadBalancers"].([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func filterLoadBalancersByHost(loadBalancers []map[string]any, hosts []string) []map[string]any {
	if len(hosts) == 0 {
		return loadBalancers
	}
	var out []map[string]any
	for _, lb := range loadBalancers {
		dns := strings.ToLower(toString(lb["dnsName"]))
		for _, host := range hosts {
			if dns != "" && strings.Contains(dns, strings.ToLower(host)) {
				out = append(out, lb)
				break
			}
		}
	}
	return out
}

func targetGroupsFromResult(data any) []map[string]any {
	payload, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	items, ok := payload["targetGroups"].([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
