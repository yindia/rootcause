package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

type ecosystemResourceSpec struct {
	Group    string
	Resource string
}

type ecosystemSpec struct {
	Name          string
	Groups        []string
	Resources     []ecosystemResourceSpec
	EventKeywords []string
}

var (
	argocdSpec = ecosystemSpec{
		Name:   "argocd",
		Groups: []string{"argoproj.io"},
		Resources: []ecosystemResourceSpec{
			{Group: "argoproj.io", Resource: "applications"},
			{Group: "argoproj.io", Resource: "applicationsets"},
			{Group: "argoproj.io", Resource: "appprojects"},
		},
		EventKeywords: []string{"argocd", "application", "sync", "comparisonerror", "invalidspecerror"},
	}
	fluxSpec = ecosystemSpec{
		Name:   "flux",
		Groups: []string{"source.toolkit.fluxcd.io", "kustomize.toolkit.fluxcd.io", "helm.toolkit.fluxcd.io", "notification.toolkit.fluxcd.io"},
		Resources: []ecosystemResourceSpec{
			{Group: "kustomize.toolkit.fluxcd.io", Resource: "kustomizations"},
			{Group: "helm.toolkit.fluxcd.io", Resource: "helmreleases"},
			{Group: "source.toolkit.fluxcd.io", Resource: "gitrepositories"},
			{Group: "source.toolkit.fluxcd.io", Resource: "helmrepositories"},
		},
		EventKeywords: []string{"flux", "reconcile", "kustomization", "helmrelease", "source"},
	}
	certManagerSpec = ecosystemSpec{
		Name:   "cert-manager",
		Groups: []string{"cert-manager.io", "acme.cert-manager.io"},
		Resources: []ecosystemResourceSpec{
			{Group: "cert-manager.io", Resource: "certificates"},
			{Group: "cert-manager.io", Resource: "certificaterequests"},
			{Group: "cert-manager.io", Resource: "issuers"},
			{Group: "cert-manager.io", Resource: "clusterissuers"},
			{Group: "acme.cert-manager.io", Resource: "orders"},
			{Group: "acme.cert-manager.io", Resource: "challenges"},
		},
		EventKeywords: []string{"cert", "certificate", "issuer", "challenge", "order"},
	}
	kyvernoSpec = ecosystemSpec{
		Name:   "kyverno",
		Groups: []string{"kyverno.io", "wgpolicyk8s.io", "reports.kyverno.io"},
		Resources: []ecosystemResourceSpec{
			{Group: "kyverno.io", Resource: "clusterpolicies"},
			{Group: "kyverno.io", Resource: "policies"},
			{Group: "wgpolicyk8s.io", Resource: "policyreports"},
			{Group: "wgpolicyk8s.io", Resource: "clusterpolicyreports"},
		},
		EventKeywords: []string{"kyverno", "policy", "violation", "admission"},
	}
	ciliumSpec = ecosystemSpec{
		Name:   "cilium",
		Groups: []string{"cilium.io"},
		Resources: []ecosystemResourceSpec{
			{Group: "cilium.io", Resource: "ciliumnetworkpolicies"},
			{Group: "cilium.io", Resource: "ciliumclusterwidenetworkpolicies"},
			{Group: "cilium.io", Resource: "ciliumendpoints"},
			{Group: "cilium.io", Resource: "ciliumnodes"},
		},
		EventKeywords: []string{"cilium", "networkpolicy", "ipam", "bpf"},
	}
)

func (t *Toolset) handleArgoCDDetect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDetect(ctx, req, argocdSpec)
}

func (t *Toolset) handleFluxDetect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDetect(ctx, req, fluxSpec)
}

func (t *Toolset) handleCertManagerDetect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDetect(ctx, req, certManagerSpec)
}

func (t *Toolset) handleKyvernoDetect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDetect(ctx, req, kyvernoSpec)
}

func (t *Toolset) handleCiliumDetect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDetect(ctx, req, ciliumSpec)
}

func (t *Toolset) handleDiagnoseArgoCD(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDiagnose(ctx, req, argocdSpec)
}

func (t *Toolset) handleDiagnoseFlux(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDiagnose(ctx, req, fluxSpec)
}

func (t *Toolset) handleDiagnoseCertManager(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDiagnose(ctx, req, certManagerSpec)
}

func (t *Toolset) handleDiagnoseKyverno(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDiagnose(ctx, req, kyvernoSpec)
}

func (t *Toolset) handleDiagnoseCilium(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleEcosystemDiagnose(ctx, req, ciliumSpec)
}

func (t *Toolset) handleEcosystemDetect(ctx context.Context, req mcp.ToolRequest, spec ecosystemSpec) (mcp.ToolResult, error) {
	if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
		return errorResult(err), err
	}
	present, foundGroups, err := kube.GroupsPresent(t.ctx.Clients.Discovery, spec.Groups)
	if err != nil {
		return errorResult(err), err
	}
	foundSet := map[string]struct{}{}
	for _, group := range foundGroups {
		foundSet[group] = struct{}{}
	}
	missingGroups := make([]string, 0)
	for _, group := range spec.Groups {
		if _, ok := foundSet[group]; !ok {
			missingGroups = append(missingGroups, group)
		}
	}
	resourceHits := t.detectResourceHits(ctx, spec)
	controlPlaneNamespaces := t.detectControlPlaneNamespaces(ctx, spec)
	return mcp.ToolResult{Data: map[string]any{
		"ecosystem":              spec.Name,
		"installed":              present,
		"detectedGroups":         foundGroups,
		"missingGroups":          missingGroups,
		"detectedResources":      resourceHits,
		"controlPlaneNamespaces": controlPlaneNamespaces,
	}}, nil
}

func (t *Toolset) handleEcosystemDiagnose(ctx context.Context, req mcp.ToolRequest, spec ecosystemSpec) (mcp.ToolResult, error) {
	namespace := strings.TrimSpace(toString(req.Arguments["namespace"]))
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return errorResult(err), err
		}
	}
	limit := toInt(req.Arguments["limit"], 50)
	if limit <= 0 {
		limit = 50
	}

	detect, err := t.handleEcosystemDetect(ctx, req, spec)
	if err != nil {
		return detect, err
	}
	detectData, _ := detect.Data.(map[string]any)
	installed, _ := detectData["installed"].(bool)
	if !installed {
		return mcp.ToolResult{Data: map[string]any{
			"ecosystem": spec.Name,
			"installed": false,
			"summary": map[string]any{
				"objects":         0,
				"degradedObjects": 0,
				"warningEvents":   0,
			},
			"findings":        []any{},
			"recommendations": []string{"Install the ecosystem CRDs/controllers before running diagnostics."},
		}}, nil
	}

	namespaces, err := t.allowedNamespaces(ctx, req.User)
	if err != nil {
		return errorResult(err), err
	}
	if namespace != "" {
		namespaces = []string{namespace}
	}

	findings := make([]map[string]any, 0)
	degraded := 0
	totalObjects := 0
	for _, target := range spec.Resources {
		gvr, namespaced, resolveErr := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "", target.Resource, target.Group)
		if resolveErr != nil {
			continue
		}
		if !namespaced {
			if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
				continue
			}
			list, listErr := t.ctx.Clients.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
			if listErr != nil {
				continue
			}
			for _, item := range list.Items {
				if len(findings) >= limit {
					break
				}
				totalObjects++
				status, issues := ecosystemHealthStatus(spec.Name, target.Resource, &item)
				if status != "healthy" {
					degraded++
				}
				findings = append(findings, map[string]any{
					"resource":  target.Resource,
					"name":      item.GetName(),
					"namespace": item.GetNamespace(),
					"status":    status,
					"issues":    issues,
				})
			}
			continue
		}
		for _, ns := range namespaces {
			list, listErr := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if listErr != nil {
				continue
			}
			for _, item := range list.Items {
				if len(findings) >= limit {
					break
				}
				totalObjects++
				status, issues := ecosystemHealthStatus(spec.Name, target.Resource, &item)
				if status != "healthy" {
					degraded++
				}
				findings = append(findings, map[string]any{
					"resource":  target.Resource,
					"name":      item.GetName(),
					"namespace": item.GetNamespace(),
					"status":    status,
					"issues":    issues,
				})
			}
		}
	}

	warningEvents, eventWarnings := t.collectEcosystemWarningEvents(ctx, namespaces, spec.EventKeywords)
	for _, warning := range eventWarnings {
		if len(findings) >= limit {
			break
		}
		findings = append(findings, map[string]any{
			"resource":  "events",
			"name":      warning["reason"],
			"namespace": warning["namespace"],
			"status":    "warning",
			"issues":    []string{warning["message"].(string)},
		})
	}

	return mcp.ToolResult{Data: map[string]any{
		"ecosystem": spec.Name,
		"installed": true,
		"summary": map[string]any{
			"objects":         totalObjects,
			"degradedObjects": degraded,
			"warningEvents":   warningEvents,
		},
		"findings":          findings,
		"recommendations":   ecosystemRecommendations(spec.Name, degraded, warningEvents),
		"detection":         detectData,
		"scannedNamespaces": namespaces,
	}}, nil
}

func (t *Toolset) detectResourceHits(ctx context.Context, spec ecosystemSpec) []string {
	hits := make([]string, 0)
	for _, target := range spec.Resources {
		gvr, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "", target.Resource, target.Group)
		if err != nil {
			continue
		}
		hits = append(hits, fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group))
	}
	sort.Strings(hits)
	return hits
}

func (t *Toolset) detectControlPlaneNamespaces(ctx context.Context, spec ecosystemSpec) []string {
	selectors := map[string]string{
		"argocd":       "app.kubernetes.io/part-of=argocd,app.kubernetes.io/name=argocd-application-controller",
		"flux":         "app.kubernetes.io/part-of=flux",
		"cert-manager": "app.kubernetes.io/name=cert-manager",
		"kyverno":      "app.kubernetes.io/name=kyverno",
		"cilium":       "k8s-app=cilium",
	}
	selector := selectors[spec.Name]
	if selector == "" {
		return nil
	}
	namespaces, err := kube.ControlPlaneNamespaces(ctx, t.ctx.Clients, []string{selector})
	if err != nil {
		return nil
	}
	return namespaces
}

func (t *Toolset) collectEcosystemWarningEvents(ctx context.Context, namespaces []string, keywords []string) (int, []map[string]any) {
	keywordSet := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		trimmed := strings.ToLower(strings.TrimSpace(keyword))
		if trimmed != "" {
			keywordSet = append(keywordSet, trimmed)
		}
	}
	if len(keywordSet) == 0 {
		return 0, nil
	}
	warnings := make([]map[string]any, 0)
	count := 0
	for _, ns := range namespaces {
		list, err := t.ctx.Clients.Typed.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, event := range list.Items {
			if !strings.EqualFold(event.Type, string(corev1.EventTypeWarning)) {
				continue
			}
			blob := strings.ToLower(event.Reason + " " + event.Message + " " + event.ReportingController + " " + event.Source.Component)
			matched := false
			for _, keyword := range keywordSet {
				if strings.Contains(blob, keyword) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			count++
			if len(warnings) < 10 {
				warnings = append(warnings, map[string]any{
					"namespace": ns,
					"reason":    event.Reason,
					"message":   event.Message,
				})
			}
		}
	}
	return count, warnings
}

func ecosystemHealthStatus(ecosystem, resource string, obj *unstructured.Unstructured) (string, []string) {
	issues := make([]string, 0)
	switch ecosystem {
	case "argocd":
		if resource == "applications" {
			health, _, _ := unstructured.NestedString(obj.Object, "status", "health", "status")
			sync, _, _ := unstructured.NestedString(obj.Object, "status", "sync", "status")
			opPhase, _, _ := unstructured.NestedString(obj.Object, "status", "operationState", "phase")
			if health != "" && !strings.EqualFold(health, "Healthy") {
				issues = append(issues, "health status="+health)
			}
			if sync != "" && !strings.EqualFold(sync, "Synced") {
				issues = append(issues, "sync status="+sync)
			}
			if strings.EqualFold(opPhase, "Failed") || strings.EqualFold(opPhase, "Error") {
				issues = append(issues, "last operation phase="+opPhase)
			}
		}
	case "flux":
		issues = append(issues, collectNonReadyConditionIssues(obj)...)
		if resource == "helmreleases" {
			for _, field := range []string{"failures", "installFailures", "upgradeFailures"} {
				if value, _, _ := unstructured.NestedInt64(obj.Object, "status", field); value > 0 {
					issues = append(issues, fmt.Sprintf("status.%s=%d", field, value))
				}
			}
		}
	case "cert-manager":
		issues = append(issues, collectNonReadyConditionIssues(obj)...)
		if resource == "certificates" {
			notAfter, _, _ := unstructured.NestedString(obj.Object, "status", "notAfter")
			if notAfter != "" {
				if ts, err := time.Parse(time.RFC3339, notAfter); err == nil {
					if ts.Before(time.Now().Add(7 * 24 * time.Hour)) {
						issues = append(issues, "certificate expires within 7d")
					}
				}
			}
		}
	case "kyverno":
		issues = append(issues, collectNonReadyConditionIssues(obj)...)
		if ready, found, _ := unstructured.NestedBool(obj.Object, "status", "ready"); found && !ready {
			issues = append(issues, "status.ready=false")
		}
		if strings.Contains(resource, "policyreport") {
			if fail, found, _ := unstructured.NestedInt64(obj.Object, "summary", "fail"); found && fail > 0 {
				issues = append(issues, fmt.Sprintf("summary.fail=%d", fail))
			}
		}
	case "cilium":
		if resource == "ciliumendpoints" {
			state, _, _ := unstructured.NestedString(obj.Object, "status", "state")
			health, _, _ := unstructured.NestedString(obj.Object, "status", "health", "overallHealth")
			if state != "" && !strings.EqualFold(state, "ready") {
				issues = append(issues, "endpoint state="+state)
			}
			if health != "" && !strings.EqualFold(health, "OK") {
				issues = append(issues, "overall health="+health)
			}
		}
		if resource == "ciliumnetworkpolicies" || resource == "ciliumclusterwidenetworkpolicies" {
			if nodes, found, _ := unstructured.NestedMap(obj.Object, "status", "nodes"); found {
				for node, raw := range nodes {
					nodeMap, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					errorText, _ := nodeMap["error"].(string)
					if strings.TrimSpace(errorText) != "" {
						issues = append(issues, "node "+node+" error="+errorText)
					}
				}
			}
		}
		if resource == "ciliumnodes" {
			errorText, _, _ := unstructured.NestedString(obj.Object, "status", "ipam", "operator-status", "error")
			if strings.TrimSpace(errorText) != "" {
				issues = append(issues, "ipam operator error="+errorText)
			}
		}
	}
	if len(issues) == 0 {
		return "healthy", nil
	}
	return "degraded", issues
}

func collectNonReadyConditionIssues(obj *unstructured.Unstructured) []string {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return nil
	}
	issues := make([]string, 0)
	for _, raw := range conditions {
		cond, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeVal, _ := cond["type"].(string)
		statusVal, _ := cond["status"].(string)
		reason, _ := cond["reason"].(string)
		message, _ := cond["message"].(string)
		if strings.EqualFold(typeVal, "Ready") && !strings.EqualFold(statusVal, "True") {
			issue := "condition Ready=" + statusVal
			if reason != "" {
				issue += " reason=" + reason
			}
			if message != "" {
				issue += " message=" + message
			}
			issues = append(issues, issue)
		}
	}
	return issues
}

func ecosystemRecommendations(name string, degraded, warningEvents int) []string {
	base := []string{
		"Review controller logs in the detected control-plane namespace.",
		"Review warning events and reconcile failures before applying changes.",
	}
	if degraded == 0 && warningEvents == 0 {
		return []string{"No immediate ecosystem health issues detected."}
	}
	switch name {
	case "argocd":
		return append(base, "Prioritize applications with health!=Healthy or sync!=Synced.")
	case "flux":
		return append(base, "Investigate resources where Ready condition is not True.")
	case "cert-manager":
		return append(base, "Investigate not-ready issuers/certificates and ACME challenge failures.")
	case "kyverno":
		return append(base, "Review failing policy reports and admission denials before rollout.")
	case "cilium":
		return append(base, "Review CiliumEndpoint state and policy node-level errors.")
	default:
		return base
	}
}
