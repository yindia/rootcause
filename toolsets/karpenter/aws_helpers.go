package karpenter

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

type awsNodeClassSelectors struct {
	subnetIDs          []string
	subnetTagTerms     []map[string]string
	securityGroupIDs   []string
	securityGroupTerms []map[string]string
	roleName           string
	instanceProfile    string
}

func (t *Toolset) addAWSNodeClassEvidence(ctx context.Context, req mcp.ToolRequest, analysis *render.Analysis, match resourceMatch, obj *unstructured.Unstructured) {
	if !isAWSNodeClass(match, obj) {
		return
	}
	selectors := extractAWSNodeClassSelectors(obj)
	if selectors.isEmpty() {
		return
	}
	analysis.AddEvidence(fmt.Sprintf("AWS selectors %s", obj.GetName()), selectors.summary())

	if t.ctx.Registry == nil {
		analysis.AddEvidence("awsToolset", "tool registry unavailable")
		return
	}

	if len(selectors.subnetIDs) > 0 || len(selectors.subnetTagTerms) > 0 {
		if _, ok := t.ctx.Registry.Get("aws.vpc.list_subnets"); !ok {
			analysis.AddEvidence("awsSubnets", "aws toolset not enabled")
		} else {
			subnetEvidence := make([]map[string]any, 0)
			if len(selectors.subnetIDs) > 0 {
				result, err := t.ctx.CallTool(ctx, req.User, "aws.vpc.list_subnets", map[string]any{
					"subnetIds": selectors.subnetIDs,
				})
				if err != nil {
					analysis.AddCause("AWS subnet lookup failed", err.Error(), "medium")
					analysis.AddEvidence("awsSubnetError", err.Error())
				} else {
					subnetEvidence = append(subnetEvidence, map[string]any{
						"selector": map[string]any{"subnetIds": selectors.subnetIDs},
						"result":   result.Data,
					})
				}
			}
			for _, term := range selectors.subnetTagTerms {
				result, err := t.ctx.CallTool(ctx, req.User, "aws.vpc.list_subnets", map[string]any{
					"tagFilters": term,
				})
				if err != nil {
					analysis.AddCause("AWS subnet lookup failed", err.Error(), "medium")
					analysis.AddEvidence("awsSubnetError", err.Error())
					continue
				}
				subnetEvidence = append(subnetEvidence, map[string]any{
					"selector": map[string]any{"tags": term},
					"result":   result.Data,
				})
			}
			if len(subnetEvidence) > 0 {
				analysis.AddEvidence(fmt.Sprintf("AWS subnets %s", obj.GetName()), subnetEvidence)
			}
		}
	}

	if len(selectors.securityGroupIDs) > 0 || len(selectors.securityGroupTerms) > 0 {
		if _, ok := t.ctx.Registry.Get("aws.vpc.list_security_groups"); !ok {
			analysis.AddEvidence("awsSecurityGroups", "aws toolset not enabled")
		} else {
			sgEvidence := make([]map[string]any, 0)
			if len(selectors.securityGroupIDs) > 0 {
				result, err := t.ctx.CallTool(ctx, req.User, "aws.vpc.list_security_groups", map[string]any{
					"groupIds": selectors.securityGroupIDs,
				})
				if err != nil {
					analysis.AddCause("AWS security group lookup failed", err.Error(), "medium")
					analysis.AddEvidence("awsSecurityGroupError", err.Error())
				} else {
					sgEvidence = append(sgEvidence, map[string]any{
						"selector": map[string]any{"groupIds": selectors.securityGroupIDs},
						"result":   result.Data,
					})
				}
			}
			for _, term := range selectors.securityGroupTerms {
				result, err := t.ctx.CallTool(ctx, req.User, "aws.vpc.list_security_groups", map[string]any{
					"tagFilters": term,
				})
				if err != nil {
					analysis.AddCause("AWS security group lookup failed", err.Error(), "medium")
					analysis.AddEvidence("awsSecurityGroupError", err.Error())
					continue
				}
				sgEvidence = append(sgEvidence, map[string]any{
					"selector": map[string]any{"tags": term},
					"result":   result.Data,
				})
			}
			if len(sgEvidence) > 0 {
				analysis.AddEvidence(fmt.Sprintf("AWS security groups %s", obj.GetName()), sgEvidence)
			}
		}
	}

	if selectors.roleName != "" {
		if _, ok := t.ctx.Registry.Get("aws.iam.get_role"); !ok {
			analysis.AddEvidence("awsIamRole", "aws toolset not enabled")
		} else {
			result, err := t.ctx.CallTool(ctx, req.User, "aws.iam.get_role", map[string]any{
				"roleName":        selectors.roleName,
				"includePolicies": true,
			})
			if err != nil {
				analysis.AddCause("AWS IAM role lookup failed", err.Error(), "medium")
				analysis.AddEvidence("awsIamRoleError", err.Error())
			} else {
				analysis.AddEvidence(fmt.Sprintf("AWS IAM role %s", selectors.roleName), result.Data)
				analysis.AddResource(fmt.Sprintf("iam/role/%s", selectors.roleName))
			}
		}
	}

	if selectors.instanceProfile != "" {
		if _, ok := t.ctx.Registry.Get("aws.iam.get_instance_profile"); !ok {
			analysis.AddEvidence("awsInstanceProfile", "aws toolset not enabled")
		} else {
			result, err := t.ctx.CallTool(ctx, req.User, "aws.iam.get_instance_profile", map[string]any{
				"instanceProfileName": selectors.instanceProfile,
			})
			if err != nil {
				analysis.AddCause("AWS instance profile lookup failed", err.Error(), "medium")
				analysis.AddEvidence("awsInstanceProfileError", err.Error())
			} else {
				analysis.AddEvidence(fmt.Sprintf("AWS instance profile %s", selectors.instanceProfile), result.Data)
				analysis.AddResource(fmt.Sprintf("iam/instance-profile/%s", selectors.instanceProfile))
			}
		}
	}
}

func isAWSNodeClass(match resourceMatch, obj *unstructured.Unstructured) bool {
	if strings.Contains(strings.ToLower(match.Group), "karpenter.k8s.aws") {
		return true
	}
	if strings.EqualFold(match.Kind, "EC2NodeClass") {
		return true
	}
	if obj != nil && strings.Contains(strings.ToLower(obj.GetAPIVersion()), "karpenter.k8s.aws") {
		return true
	}
	return false
}

func extractAWSNodeClassSelectors(obj *unstructured.Unstructured) awsNodeClassSelectors {
	selectors := awsNodeClassSelectors{}
	if obj == nil {
		return selectors
	}
	roleValue := strings.TrimSpace(nestedString(obj, "spec", "role"))
	if roleValue != "" {
		selectors.roleName = awsNameFromARN(roleValue, ":role/")
	}
	profileValue := strings.TrimSpace(nestedString(obj, "spec", "instanceProfile"))
	if profileValue != "" {
		selectors.instanceProfile = awsNameFromARN(profileValue, ":instance-profile/")
	}

	selectors.subnetIDs, selectors.subnetTagTerms = extractSelectorInfo(obj, "subnetSelectorTerms", "subnetSelector",
		[]string{"id", "ids", "subnetId", "subnetIds"})
	selectors.securityGroupIDs, selectors.securityGroupTerms = extractSelectorInfo(obj, "securityGroupSelectorTerms", "securityGroupSelector",
		[]string{"id", "ids", "groupId", "groupIds", "securityGroupId", "securityGroupIds"})

	return selectors
}

func (s awsNodeClassSelectors) isEmpty() bool {
	return len(s.subnetIDs) == 0 && len(s.subnetTagTerms) == 0 &&
		len(s.securityGroupIDs) == 0 && len(s.securityGroupTerms) == 0 &&
		s.roleName == "" && s.instanceProfile == ""
}

func (s awsNodeClassSelectors) summary() map[string]any {
	return map[string]any{
		"subnetIds":          s.subnetIDs,
		"subnetTagTerms":     s.subnetTagTerms,
		"securityGroupIds":   s.securityGroupIDs,
		"securityGroupTerms": s.securityGroupTerms,
		"role":               s.roleName,
		"instanceProfile":    s.instanceProfile,
	}
}

func extractSelectorInfo(obj *unstructured.Unstructured, termsField, selectorField string, idKeys []string) ([]string, []map[string]string) {
	var ids []string
	var terms []map[string]string
	for _, term := range selectorTerms(obj, termsField) {
		ids = append(ids, selectorIDs(term, idKeys)...)
		if tags := selectorTags(term); len(tags) > 0 {
			terms = append(terms, tags)
		}
	}
	if tags, ok := nestedMap(obj, "spec", selectorField); ok {
		if mapped := stringMapFromAny(tags); len(mapped) > 0 {
			terms = append(terms, mapped)
		}
	}
	return uniqueStrings(ids), terms
}

func selectorTerms(obj *unstructured.Unstructured, field string) []map[string]any {
	items, found, _ := unstructured.NestedSlice(obj.Object, "spec", field)
	if !found {
		return nil
	}
	terms := make([]map[string]any, 0, len(items))
	for _, item := range items {
		term, ok := item.(map[string]any)
		if !ok {
			continue
		}
		terms = append(terms, term)
	}
	return terms
}

func selectorIDs(term map[string]any, keys []string) []string {
	var ids []string
	for _, key := range keys {
		value, ok := term[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				if val := strings.TrimSpace(fmt.Sprintf("%v", item)); val != "" {
					ids = append(ids, val)
				}
			}
		case []string:
			for _, item := range typed {
				if val := strings.TrimSpace(item); val != "" {
					ids = append(ids, val)
				}
			}
		default:
			if val := strings.TrimSpace(fmt.Sprintf("%v", typed)); val != "" {
				ids = append(ids, val)
			}
		}
	}
	return ids
}

func selectorTags(term map[string]any) map[string]string {
	if raw, ok := term["tags"]; ok {
		return stringMapFromAny(raw)
	}
	return nil
}

func stringMapFromAny(value any) map[string]string {
	out := map[string]string{}
	switch typed := value.(type) {
	case map[string]any:
		for key, raw := range typed {
			val := strings.TrimSpace(fmt.Sprintf("%v", raw))
			if key == "" || val == "" {
				continue
			}
			out[key] = val
		}
	case map[string]string:
		for key, val := range typed {
			if key == "" || strings.TrimSpace(val) == "" {
				continue
			}
			out[key] = strings.TrimSpace(val)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func awsNameFromARN(value, marker string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "arn:") {
		return value
	}
	idx := strings.Index(value, marker)
	if idx == -1 {
		return value
	}
	name := strings.TrimPrefix(value[idx+len(marker):], "/")
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		name = parts[len(parts)-1]
	}
	if name == "" {
		return value
	}
	return name
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
