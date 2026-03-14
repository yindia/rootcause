package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

const (
	irsaRoleAnnotation = "eks.amazonaws.com/role-arn"
)

func (t *Toolset) handlePermissionDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	podName := toString(req.Arguments["pod"])
	serviceAccount := toString(req.Arguments["serviceAccount"])
	awsRegion := toString(req.Arguments["awsRegion"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	analysis := render.NewAnalysis()
	cloud := detectCloud(t.ctx.Clients)
	addCloudEvidence(&analysis, cloud)
	var pod *corev1.Pod
	if podName != "" {
		obj, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		pod = obj
		if serviceAccount == "" {
			serviceAccount = pod.Spec.ServiceAccountName
		}
		analysis.AddEvidence("pod", t.ctx.Evidence.PodStatusSummary(pod))
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, podName))
	}
	if serviceAccount == "" {
		serviceAccount = "default"
		analysis.AddEvidence("serviceAccount", "default (no pod/serviceAccount specified)")
	}

	sa, err := t.ctx.Clients.Typed.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{})
	if err != nil {
		return errorResult(err), err
	}
	analysis.AddEvidence("serviceAccount", map[string]any{
		"name":                 sa.Name,
		"namespace":            sa.Namespace,
		"annotations":          sa.Annotations,
		"automountToken":       sa.AutomountServiceAccountToken,
		"imagePullSecrets":     sa.ImagePullSecrets,
		"secrets":              sa.Secrets,
		"serviceAccountTokens": len(sa.Secrets),
	})
	analysis.AddResource(fmt.Sprintf("serviceaccounts/%s/%s", namespace, serviceAccount))

	roleBindings, roleRefs, err := t.roleBindingsForServiceAccount(ctx, namespace, serviceAccount)
	if err != nil {
		analysis.AddEvidence("roleBindingError", err.Error())
	} else {
		analysis.AddEvidence("roleBindings", roleBindings)
		if len(roleBindings) == 0 {
			analysis.AddCause("No RoleBindings found", "ServiceAccount has no namespace RoleBindings", "high")
			analysis.AddNextCheck("Create a RoleBinding/ClusterRoleBinding for the ServiceAccount")
		}
	}

	clusterBindings := []map[string]any{}
	if req.User.Role == policy.RoleCluster {
		clusterBindings, roleRefs = t.clusterRoleBindingsForServiceAccount(ctx, namespace, serviceAccount, roleRefs)
		analysis.AddEvidence("clusterRoleBindings", clusterBindings)
		if len(clusterBindings) == 0 {
			analysis.AddEvidence("clusterRoleBindingsNote", "no cluster role bindings for ServiceAccount")
		}
	} else {
		analysis.AddEvidence("clusterRoleBindings", "skipped: requires cluster role")
	}

	roleEvidence, roleWarnings := t.fetchRoleRefs(ctx, namespace, roleRefs)
	if len(roleEvidence) > 0 {
		analysis.AddEvidence("roleRules", roleEvidence)
	}
	if len(roleWarnings) > 0 {
		for _, warn := range roleWarnings {
			analysis.AddEvidence("roleWarning", warn)
		}
	}

	if sa.AutomountServiceAccountToken != nil && !*sa.AutomountServiceAccountToken {
		analysis.AddCause("ServiceAccount token disabled", "automountServiceAccountToken=false may block auth", "medium")
	}

	annotation := strings.TrimSpace(sa.Annotations[irsaRoleAnnotation])
	if annotation != "" {
		analysis.AddEvidence("irsaRoleArn", annotation)
		roleName, err := roleNameFromARN(annotation)
		if err != nil {
			analysis.AddCause("Invalid IAM role ARN", err.Error(), "high")
		} else {
			analysis.AddEvidence("irsaRoleName", roleName)
			t.addAWSRoleEvidence(ctx, req, &analysis, roleName, awsRegion, namespace, serviceAccount)
		}
	}

	if len(analysis.LikelyRootCauses) == 0 {
		analysis.AddEvidence("status", "no explicit RBAC errors found")
	}
	if annotation != "" {
		analysis.AddNextCheck("Verify IAM policy permissions and trust policy for the IRSA role")
	}
	analysis.AddNextCheck("Check pod events/logs for Forbidden or AccessDenied messages")
	if !isAWSCloud(cloud.provider) {
		addCloudHints(&analysis, cloud.provider, "auth")
	}

	return mcp.ToolResult{
		Data: t.ctx.Renderer.Render(analysis),
		Metadata: mcp.ToolMetadata{
			Namespaces: []string{namespace},
		},
	}, nil
}

func (t *Toolset) roleBindingsForServiceAccount(ctx context.Context, namespace, serviceAccount string) ([]map[string]any, map[string]rbacv1.RoleRef, error) {
	list, err := t.ctx.Clients.Typed.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	roleRefs := map[string]rbacv1.RoleRef{}
	var bindings []map[string]any
	for _, binding := range list.Items {
		if !bindingMatchesServiceAccount(binding.Subjects, namespace, serviceAccount) {
			continue
		}
		refsKey := fmt.Sprintf("%s/%s/%s", binding.RoleRef.Kind, namespace, binding.RoleRef.Name)
		roleRefs[refsKey] = binding.RoleRef
		bindings = append(bindings, map[string]any{
			"name":     binding.Name,
			"roleRef":  map[string]string{"kind": binding.RoleRef.Kind, "name": binding.RoleRef.Name},
			"subjects": summarizeSubjects(binding.Subjects),
		})
	}
	sort.Slice(bindings, func(i, j int) bool {
		return toString(bindings[i]["name"]) < toString(bindings[j]["name"])
	})
	return bindings, roleRefs, nil
}

func (t *Toolset) clusterRoleBindingsForServiceAccount(ctx context.Context, namespace, serviceAccount string, roleRefs map[string]rbacv1.RoleRef) ([]map[string]any, map[string]rbacv1.RoleRef) {
	list, err := t.ctx.Clients.Typed.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []map[string]any{{"error": err.Error()}}, roleRefs
	}
	var bindings []map[string]any
	for _, binding := range list.Items {
		if !bindingMatchesServiceAccount(binding.Subjects, namespace, serviceAccount) {
			continue
		}
		refsKey := fmt.Sprintf("%s/%s/%s", binding.RoleRef.Kind, "cluster", binding.RoleRef.Name)
		roleRefs[refsKey] = binding.RoleRef
		bindings = append(bindings, map[string]any{
			"name":     binding.Name,
			"roleRef":  map[string]string{"kind": binding.RoleRef.Kind, "name": binding.RoleRef.Name},
			"subjects": summarizeSubjects(binding.Subjects),
		})
	}
	sort.Slice(bindings, func(i, j int) bool {
		return toString(bindings[i]["name"]) < toString(bindings[j]["name"])
	})
	return bindings, roleRefs
}

func (t *Toolset) fetchRoleRefs(ctx context.Context, namespace string, roleRefs map[string]rbacv1.RoleRef) ([]map[string]any, []string) {
	if len(roleRefs) == 0 {
		return nil, nil
	}
	var evidence []map[string]any
	var warnings []string
	for _, ref := range roleRefs {
		switch ref.Kind {
		case "Role":
			role, err := t.ctx.Clients.Typed.RbacV1().Roles(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("role %s/%s not found: %v", namespace, ref.Name, err))
				continue
			}
			if len(role.Rules) == 0 {
				warnings = append(warnings, fmt.Sprintf("role %s/%s has no rules", namespace, ref.Name))
			}
			evidence = append(evidence, map[string]any{
				"kind":  "Role",
				"name":  role.Name,
				"rules": summarizePolicyRules(role.Rules),
			})
		case "ClusterRole":
			role, err := t.ctx.Clients.Typed.RbacV1().ClusterRoles().Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("clusterrole %s not found: %v", ref.Name, err))
				continue
			}
			if len(role.Rules) == 0 {
				warnings = append(warnings, fmt.Sprintf("clusterrole %s has no rules", ref.Name))
			}
			evidence = append(evidence, map[string]any{
				"kind":  "ClusterRole",
				"name":  role.Name,
				"rules": summarizePolicyRules(role.Rules),
			})
		default:
			warnings = append(warnings, fmt.Sprintf("unsupported roleRef kind: %s", ref.Kind))
		}
	}
	return evidence, warnings
}

func summarizePolicyRules(rules []rbacv1.PolicyRule) []map[string]any {
	var out []map[string]any
	for _, rule := range rules {
		out = append(out, map[string]any{
			"apiGroups":       rule.APIGroups,
			"resources":       rule.Resources,
			"resourceNames":   rule.ResourceNames,
			"verbs":           rule.Verbs,
			"nonResourceURLs": rule.NonResourceURLs,
		})
	}
	return out
}

func summarizeSubjects(subjects []rbacv1.Subject) []map[string]string {
	var out []map[string]string
	for _, subject := range subjects {
		out = append(out, map[string]string{
			"kind":      subject.Kind,
			"name":      subject.Name,
			"namespace": subject.Namespace,
		})
	}
	return out
}

func bindingMatchesServiceAccount(subjects []rbacv1.Subject, namespace, name string) bool {
	for _, subject := range subjects {
		if subject.Kind != "ServiceAccount" {
			continue
		}
		if subject.Name != name {
			continue
		}
		if subject.Namespace == "" || subject.Namespace == namespace {
			return true
		}
	}
	return false
}

func roleNameFromARN(arn string) (string, error) {
	if !strings.Contains(arn, ":role/") {
		return "", fmt.Errorf("unsupported role ARN: %s", arn)
	}
	parts := strings.SplitN(arn, ":role/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unsupported role ARN: %s", arn)
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return "", fmt.Errorf("role ARN missing name: %s", arn)
	}
	if strings.Contains(name, "/") {
		segments := strings.Split(name, "/")
		name = segments[len(segments)-1]
	}
	if name == "" {
		return "", fmt.Errorf("role ARN missing name: %s", arn)
	}
	return name, nil
}

func (t *Toolset) addAWSRoleEvidence(ctx context.Context, req mcp.ToolRequest, analysis *render.Analysis, roleName, region, namespace, serviceAccount string) {
	if t.ctx.Registry == nil {
		analysis.AddEvidence("awsIam", "tool registry unavailable")
		return
	}
	if _, ok := t.ctx.Registry.Get("aws.iam.get_role"); !ok {
		analysis.AddEvidence("awsIam", "aws toolset not enabled")
		return
	}
	if t.ctx.Clients == nil {
		analysis.AddEvidence("cloud", "unknown (missing kube clients)")
		analysis.AddEvidence("awsIam", "skipped: missing kube clients")
		return
	}
	cloud, reason := kube.DetectCloud(t.ctx.Clients.RestConfig)
	if cloud != kube.CloudAWS {
		analysis.AddEvidence("cloud", fmt.Sprintf("%s (%s)", cloud, reason))
		analysis.AddEvidence("awsIam", "skipped: non-aws cluster")
		return
	}
	result, err := t.ctx.CallTool(ctx, req.User, "aws.iam.get_role", map[string]any{
		"roleName":        roleName,
		"region":          region,
		"includePolicies": true,
	})
	if err != nil {
		analysis.AddCause("IAM role lookup failed", err.Error(), "high")
		analysis.AddEvidence("awsIamError", err.Error())
		return
	}
	analysis.AddEvidence("awsIamRole", result.Data)
	analysis.AddResource(fmt.Sprintf("iam/role/%s", roleName))
	if !assumePolicyMentionsServiceAccount(result.Data, namespace, serviceAccount) {
		analysis.AddCause("IAM trust policy mismatch", "Assume role policy does not reference the ServiceAccount", "high")
	}
	t.addAWSOIDCTrustEvidence(ctx, req, analysis, result.Data, namespace, serviceAccount)
}

func assumePolicyMentionsServiceAccount(data any, namespace, serviceAccount string) bool {
	needle := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount)
	return assumePolicyContains(data, needle)
}

func (t *Toolset) addAWSOIDCTrustEvidence(ctx context.Context, req mcp.ToolRequest, analysis *render.Analysis, roleData any, namespace, serviceAccount string) {
	if analysis == nil {
		return
	}
	issuer, clusterName, err := t.findEKSOIDCIssuer(ctx, req)
	if err != nil {
		analysis.AddEvidence("eksOidc", err.Error())
		return
	}
	if issuer == "" {
		analysis.AddEvidence("eksOidc", "issuer not found")
		return
	}
	analysis.AddEvidence("eksOidcIssuer", issuer)
	if clusterName != "" {
		analysis.AddEvidence("eksCluster", clusterName)
	}
	if !assumePolicyContains(roleData, issuer) {
		analysis.AddCause("IAM trust policy missing OIDC issuer", "Assume role policy does not reference the cluster OIDC issuer", "high")
	}
	subject := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount)
	if !assumePolicyContains(roleData, subject) {
		analysis.AddCause("IAM trust policy missing subject", "Assume role policy does not allow the ServiceAccount subject", "high")
	}
	if !assumePolicyContains(roleData, "sts.amazonaws.com") {
		analysis.AddCause("IAM trust policy missing audience", "Assume role policy missing sts.amazonaws.com audience", "medium")
	}
}

func (t *Toolset) findEKSOIDCIssuer(ctx context.Context, req mcp.ToolRequest) (string, string, error) {
	if t.ctx.Registry == nil {
		return "", "", fmt.Errorf("tool registry unavailable")
	}
	if _, ok := t.ctx.Registry.Get("aws.eks.list_clusters"); !ok {
		return "", "", fmt.Errorf("aws eks toolset not enabled")
	}
	if t.ctx.Clients == nil || t.ctx.Clients.RestConfig == nil {
		return "", "", fmt.Errorf("missing rest config")
	}
	cloud := detectCloud(t.ctx.Clients)
	if cloud.provider != kube.CloudAWS {
		return "", "", fmt.Errorf("non-aws cluster (%s)", cloud.provider)
	}
	host := hostFromURL(t.ctx.Clients.RestConfig.Host)
	if host == "" {
		return "", "", fmt.Errorf("missing api server host")
	}

	listResult, err := t.ctx.CallTool(ctx, req.User, "aws.eks.list_clusters", map[string]any{"limit": 100})
	if err != nil {
		return "", "", err
	}
	clusters := toStringSliceFromData(listResult.Data, "clusters")
	for _, name := range clusters {
		clusterResult, err := t.ctx.CallTool(ctx, req.User, "aws.eks.get_cluster", map[string]any{"name": name})
		if err != nil {
			continue
		}
		clusterData, ok := clusterResult.Data.(map[string]any)
		if !ok {
			continue
		}
		cluster, ok := clusterData["cluster"].(map[string]any)
		if !ok {
			continue
		}
		endpoint := toString(cluster["endpoint"])
		if endpoint == "" {
			continue
		}
		if hostFromURL(endpoint) != host {
			continue
		}
		issuer := extractOIDCIssuer(cluster["identity"])
		if issuer != "" {
			return issuer, name, nil
		}
	}
	return "", "", fmt.Errorf("no matching EKS cluster for host %s", host)
}

func extractOIDCIssuer(identity any) string {
	if identity == nil {
		return ""
	}
	if data, ok := identity.(map[string]any); ok {
		if oidc, ok := data["oidc"]; ok {
			if oidcMap, ok := oidc.(map[string]any); ok {
				if issuer := toString(oidcMap["issuer"]); issuer != "" {
					return issuer
				}
			}
		}
	}
	if s, ok := identity.(string); ok && s != "" {
		return s
	}
	blob, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	return extractIssuerFromJSON(string(blob))
}

func extractIssuerFromJSON(raw string) string {
	matches := issuerRegexp.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

var issuerRegexp = regexp.MustCompile(`"issuer"\s*:\s*"?([^"\\}]+)"?`)

func assumePolicyContains(data any, needle string) bool {
	if data == nil {
		return false
	}
	payload, ok := data.(map[string]any)
	if !ok {
		return false
	}
	policy, ok := payload["assumeRolePolicy"]
	if !ok {
		return false
	}
	if s, ok := policy.(string); ok {
		return strings.Contains(s, needle)
	}
	blob, err := json.Marshal(policy)
	if err != nil {
		return strings.Contains(fmt.Sprintf("%v", policy), needle)
	}
	return strings.Contains(string(blob), needle)
}

func toStringSliceFromData(data any, key string) []string {
	payload, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	list, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
