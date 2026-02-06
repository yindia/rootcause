package awsiam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	iamClient func(context.Context, string) (*iam.Client, string, error)
	toolsetID string
}

func ToolSpecs(ctx mcp.ToolsetContext, toolsetID string, iamClient func(context.Context, string) (*iam.Client, string, error)) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, iamClient: iamClient, toolsetID: toolsetID}
	return []mcp.ToolSpec{
		{
			Name:        "aws.iam.list_roles",
			Description: "List IAM roles (path prefix, limit supported).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMListRoles(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleIAMListRoles,
		},
		{
			Name:        "aws.iam.get_role",
			Description: "Get an IAM role with attached/inline policies.",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMGetRole(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleIAMGetRole,
		},
		{
			Name:        "aws.iam.get_instance_profile",
			Description: "Get an IAM instance profile and its roles.",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMGetInstanceProfile(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleIAMGetInstanceProfile,
		},
		{
			Name:        "aws.iam.update_role",
			Description: "Update IAM role description or assume-role policy (confirm required).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMUpdateRole(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     svc.handleIAMUpdateRole,
		},
		{
			Name:        "aws.iam.delete_role",
			Description: "Delete IAM role (confirm required; optional force detach).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMDeleteRole(),
			Safety:      mcp.SafetyDestructive,
			Handler:     svc.handleIAMDeleteRole,
		},
		{
			Name:        "aws.iam.list_policies",
			Description: "List IAM policies (scope/attached/limit supported).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMListPolicies(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleIAMListPolicies,
		},
		{
			Name:        "aws.iam.get_policy",
			Description: "Get IAM policy and default policy document.",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMGetPolicy(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleIAMGetPolicy,
		},
		{
			Name:        "aws.iam.update_policy",
			Description: "Create a new IAM policy version (confirm required).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMUpdatePolicy(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     svc.handleIAMUpdatePolicy,
		},
		{
			Name:        "aws.iam.delete_policy",
			Description: "Delete IAM policy (confirm required; optional force).",
			ToolsetID:   toolsetID,
			InputSchema: schemaIAMDeletePolicy(),
			Safety:      mcp.SafetyDestructive,
			Handler:     svc.handleIAMDeletePolicy,
		},
	}
}

func (s *Service) handleIAMListRoles(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	pathPrefix := toString(req.Arguments["pathPrefix"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &iam.ListRolesInput{}
	if pathPrefix != "" {
		input.PathPrefix = aws.String(pathPrefix)
	}
	paginator := iam.NewListRolesPaginator(client, input)
	var roles []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, role := range out.Roles {
			roles = append(roles, summarizeRole(role))
			if limit > 0 && len(roles) >= limit {
				break
			}
		}
		if limit > 0 && len(roles) >= limit {
			break
		}
	}
	data := map[string]any{
		"region": regionOrDefault(usedRegion),
		"roles":  roles,
		"count":  len(roles),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleIAMGetRole(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	roleName := toString(req.Arguments["roleName"])
	includePolicies := toBool(req.Arguments["includePolicies"], true)
	if roleName == "" {
		return errorResult(errors.New("roleName is required")), errors.New("roleName is required")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return errorResult(err), err
	}
	role := out.Role
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"role":   summarizeRole(*role),
	}
	assumeDoc := decodePolicyDocument(aws.ToString(role.AssumeRolePolicyDocument))
	if strings.TrimSpace(assumeDoc) != "" {
		result["assumeRolePolicy"] = parseJSONOrString(assumeDoc)
	}
	if includePolicies {
		attached, err := listAttachedRolePolicies(ctx, client, roleName)
		if err != nil {
			return errorResult(err), err
		}
		inline, err := listInlineRolePolicies(ctx, client, roleName)
		if err != nil {
			return errorResult(err), err
		}
		instanceProfiles, err := listInstanceProfiles(ctx, client, roleName)
		if err != nil {
			return errorResult(err), err
		}
		result["attachedPolicies"] = attached
		result["inlinePolicies"] = inline
		result["instanceProfiles"] = instanceProfiles
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/role/%s", roleName)},
		},
	}, nil
}

func (s *Service) handleIAMGetInstanceProfile(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	profileName := toString(req.Arguments["instanceProfileName"])
	if profileName == "" {
		return errorResult(errors.New("instanceProfileName is required")), errors.New("instanceProfileName is required")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: aws.String(profileName)})
	if err != nil {
		return errorResult(err), err
	}
	result := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"instanceProfile": summarizeInstanceProfile(out.InstanceProfile),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/instance-profile/%s", profileName)},
		},
	}, nil
}

func (s *Service) handleIAMUpdateRole(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	region := toString(req.Arguments["region"])
	roleName := toString(req.Arguments["roleName"])
	desc := toString(req.Arguments["description"])
	assumeDoc := toString(req.Arguments["assumeRolePolicyDocument"])
	maxSession := toInt(req.Arguments["maxSessionDurationSeconds"], 0)
	if roleName == "" {
		return errorResult(errors.New("roleName is required")), errors.New("roleName is required")
	}
	if assumeDoc != "" && !json.Valid([]byte(assumeDoc)) {
		return errorResult(errors.New("assumeRolePolicyDocument must be valid JSON")), errors.New("assumeRolePolicyDocument must be valid JSON")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	updates := map[string]any{}
	if desc != "" || maxSession > 0 {
		input := &iam.UpdateRoleInput{RoleName: aws.String(roleName)}
		if desc != "" {
			input.Description = aws.String(desc)
		}
		if maxSession > 0 {
			input.MaxSessionDuration = aws.Int32(int32(maxSession))
		}
		if _, err := client.UpdateRole(ctx, input); err != nil {
			return errorResult(err), err
		}
		updates["role"] = map[string]any{"description": desc, "maxSessionDurationSeconds": maxSession}
	}
	if assumeDoc != "" {
		if _, err := client.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
			RoleName:       aws.String(roleName),
			PolicyDocument: aws.String(assumeDoc),
		}); err != nil {
			return errorResult(err), err
		}
		updates["assumeRolePolicyDocument"] = "updated"
	}
	if len(updates) == 0 {
		return errorResult(errors.New("no updates specified")), errors.New("no updates specified")
	}
	result := map[string]any{
		"region":  regionOrDefault(usedRegion),
		"role":    roleName,
		"updated": updates,
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/role/%s", roleName)},
		},
	}, nil
}

func (s *Service) handleIAMDeleteRole(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	region := toString(req.Arguments["region"])
	roleName := toString(req.Arguments["roleName"])
	force := toBool(req.Arguments["force"], false)
	if roleName == "" {
		return errorResult(errors.New("roleName is required")), errors.New("roleName is required")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	attached, err := listAttachedRolePolicies(ctx, client, roleName)
	if err != nil {
		return errorResult(err), err
	}
	inline, err := listInlineRolePolicies(ctx, client, roleName)
	if err != nil {
		return errorResult(err), err
	}
	instanceProfiles, err := listInstanceProfiles(ctx, client, roleName)
	if err != nil {
		return errorResult(err), err
	}
	if (!force) && (len(attached) > 0 || len(inline) > 0 || len(instanceProfiles) > 0) {
		return errorResult(fmt.Errorf("role has attached policies or instance profiles; set force=true to detach")), fmt.Errorf("role has attached policies or instance profiles; set force=true to detach")
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"role":   roleName,
	}
	if force {
		detached, err := detachRolePolicies(ctx, client, roleName, attached)
		if err != nil {
			return errorResult(err), err
		}
		deletedInline, err := deleteInlineRolePolicies(ctx, client, roleName, inline)
		if err != nil {
			return errorResult(err), err
		}
		removedProfiles, err := removeRoleFromProfiles(ctx, client, roleName, instanceProfiles)
		if err != nil {
			return errorResult(err), err
		}
		result["detachedPolicies"] = detached
		result["deletedInlinePolicies"] = deletedInline
		result["removedInstanceProfiles"] = removedProfiles
	}
	if _, err := client.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(roleName)}); err != nil {
		return errorResult(err), err
	}
	result["deleted"] = true
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/role/%s", roleName)},
		},
	}, nil
}

func (s *Service) handleIAMListPolicies(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	scope := toString(req.Arguments["scope"])
	onlyAttached := toBool(req.Arguments["onlyAttached"], false)
	pathPrefix := toString(req.Arguments["pathPrefix"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &iam.ListPoliciesInput{}
	if scope != "" {
		input.Scope = iamtypes.PolicyScopeType(scope)
	} else {
		input.Scope = iamtypes.PolicyScopeTypeLocal
	}
	if onlyAttached {
		input.OnlyAttached = true
	}
	if pathPrefix != "" {
		input.PathPrefix = aws.String(pathPrefix)
	}
	paginator := iam.NewListPoliciesPaginator(client, input)
	var policies []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, policy := range out.Policies {
			policies = append(policies, summarizePolicy(policy))
			if limit > 0 && len(policies) >= limit {
				break
			}
		}
		if limit > 0 && len(policies) >= limit {
			break
		}
	}
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"policies": policies,
		"count":    len(policies),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleIAMGetPolicy(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	policyArn := toString(req.Arguments["policyArn"])
	versionID := toString(req.Arguments["versionId"])
	includeDoc := toBool(req.Arguments["includeDocument"], true)
	if policyArn == "" {
		return errorResult(errors.New("policyArn is required")), errors.New("policyArn is required")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: aws.String(policyArn)})
	if err != nil {
		return errorResult(err), err
	}
	policy := out.Policy
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"policy": summarizePolicy(*policy),
	}
	if includeDoc {
		if versionID == "" {
			versionID = aws.ToString(policy.DefaultVersionId)
		}
		versionOut, err := client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: aws.String(versionID),
		})
		if err != nil {
			return errorResult(err), err
		}
		doc := aws.ToString(versionOut.PolicyVersion.Document)
		result["policyDocument"] = parseJSONOrString(decodePolicyDocument(doc))
		result["policyVersion"] = versionID
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/policy/%s", policyArn)},
		},
	}, nil
}

func (s *Service) handleIAMUpdatePolicy(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	region := toString(req.Arguments["region"])
	policyArn := toString(req.Arguments["policyArn"])
	document := toString(req.Arguments["document"])
	setDefault := toBool(req.Arguments["setDefault"], true)
	prune := toBool(req.Arguments["prune"], false)
	if policyArn == "" || document == "" {
		return errorResult(errors.New("policyArn and document are required")), errors.New("policyArn and document are required")
	}
	if !json.Valid([]byte(document)) {
		return errorResult(errors.New("document must be valid JSON")), errors.New("document must be valid JSON")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	if prune {
		if err := prunePolicyVersions(ctx, client, policyArn); err != nil {
			return errorResult(err), err
		}
	}
	out, err := client.CreatePolicyVersion(ctx, &iam.CreatePolicyVersionInput{
		PolicyArn:      aws.String(policyArn),
		PolicyDocument: aws.String(document),
		SetAsDefault:   setDefault,
	})
	if err != nil {
		return errorResult(err), err
	}
	result := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"policyArn":      policyArn,
		"versionId":      aws.ToString(out.PolicyVersion.VersionId),
		"setAsDefault":   setDefault,
		"createDate":     out.PolicyVersion.CreateDate,
		"isDefault":      out.PolicyVersion.IsDefaultVersion,
		"policyDocument": "updated",
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/policy/%s", policyArn)},
		},
	}, nil
}

func (s *Service) handleIAMDeletePolicy(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	region := toString(req.Arguments["region"])
	policyArn := toString(req.Arguments["policyArn"])
	force := toBool(req.Arguments["force"], false)
	if policyArn == "" {
		return errorResult(errors.New("policyArn is required")), errors.New("policyArn is required")
	}
	client, usedRegion, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	if force {
		if err := deleteNonDefaultPolicyVersions(ctx, client, policyArn); err != nil {
			return errorResult(err), err
		}
	}
	if _, err := client.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(policyArn)}); err != nil {
		return errorResult(err), err
	}
	result := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"policyArn": policyArn,
		"deleted":   true,
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("iam/policy/%s", policyArn)},
		},
	}, nil
}

func listAttachedRolePolicies(ctx context.Context, client *iam.Client, roleName string) ([]map[string]any, error) {
	paginator := iam.NewListAttachedRolePoliciesPaginator(client, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	var out []map[string]any
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, policy := range page.AttachedPolicies {
			out = append(out, map[string]any{"name": aws.ToString(policy.PolicyName), "arn": aws.ToString(policy.PolicyArn)})
		}
	}
	return out, nil
}

func listInlineRolePolicies(ctx context.Context, client *iam.Client, roleName string) ([]string, error) {
	paginator := iam.NewListRolePoliciesPaginator(client, &iam.ListRolePoliciesInput{RoleName: aws.String(roleName)})
	var out []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.PolicyNames...)
	}
	sort.Strings(out)
	return out, nil
}

func listInstanceProfiles(ctx context.Context, client *iam.Client, roleName string) ([]string, error) {
	paginator := iam.NewListInstanceProfilesForRolePaginator(client, &iam.ListInstanceProfilesForRoleInput{
		RoleName: aws.String(roleName),
	})
	var out []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, profile := range page.InstanceProfiles {
			out = append(out, aws.ToString(profile.InstanceProfileName))
		}
	}
	sort.Strings(out)
	return out, nil
}

func detachRolePolicies(ctx context.Context, client *iam.Client, roleName string, policies []map[string]any) ([]string, error) {
	var detached []string
	for _, policy := range policies {
		arn := toString(policy["arn"])
		if arn == "" {
			continue
		}
		if _, err := client.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: aws.String(arn),
		}); err != nil {
			return detached, err
		}
		detached = append(detached, arn)
	}
	return detached, nil
}

func deleteInlineRolePolicies(ctx context.Context, client *iam.Client, roleName string, policies []string) ([]string, error) {
	var deleted []string
	for _, name := range policies {
		if _, err := client.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(name),
		}); err != nil {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}

func removeRoleFromProfiles(ctx context.Context, client *iam.Client, roleName string, profiles []string) ([]string, error) {
	var removed []string
	for _, profile := range profiles {
		if _, err := client.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: aws.String(profile),
			RoleName:            aws.String(roleName),
		}); err != nil {
			return removed, err
		}
		removed = append(removed, profile)
	}
	return removed, nil
}

func prunePolicyVersions(ctx context.Context, client *iam.Client, policyArn string) error {
	versions, err := listPolicyVersions(ctx, client, policyArn)
	if err != nil {
		return err
	}
	if len(versions) < 5 {
		return nil
	}
	sort.Slice(versions, func(i, j int) bool {
		return policyVersionTime(versions[i]).Before(policyVersionTime(versions[j]))
	})
	for _, version := range versions {
		if version.IsDefaultVersion {
			continue
		}
		_, err := client.DeletePolicyVersion(ctx, &iam.DeletePolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: version.VersionId,
		})
		if err != nil {
			return err
		}
		versions, err = listPolicyVersions(ctx, client, policyArn)
		if err != nil {
			return err
		}
		if len(versions) < 5 {
			break
		}
	}
	return nil
}

func deleteNonDefaultPolicyVersions(ctx context.Context, client *iam.Client, policyArn string) error {
	versions, err := listPolicyVersions(ctx, client, policyArn)
	if err != nil {
		return err
	}
	for _, version := range versions {
		if version.IsDefaultVersion {
			continue
		}
		if _, err := client.DeletePolicyVersion(ctx, &iam.DeletePolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: version.VersionId,
		}); err != nil {
			return err
		}
	}
	return nil
}

func listPolicyVersions(ctx context.Context, client *iam.Client, policyArn string) ([]iamtypes.PolicyVersion, error) {
	out, err := client.ListPolicyVersions(ctx, &iam.ListPolicyVersionsInput{PolicyArn: aws.String(policyArn)})
	if err != nil {
		return nil, err
	}
	return out.Versions, nil
}

func policyVersionTime(version iamtypes.PolicyVersion) time.Time {
	if version.CreateDate == nil {
		return time.Time{}
	}
	return *version.CreateDate
}

func summarizeRole(role iamtypes.Role) map[string]any {
	return map[string]any{
		"name":        aws.ToString(role.RoleName),
		"arn":         aws.ToString(role.Arn),
		"path":        aws.ToString(role.Path),
		"id":          aws.ToString(role.RoleId),
		"description": aws.ToString(role.Description),
		"createDate":  role.CreateDate,
		"maxSession":  role.MaxSessionDuration,
	}
}

func summarizeInstanceProfile(profile *iamtypes.InstanceProfile) map[string]any {
	if profile == nil {
		return map[string]any{"status": "not found"}
	}
	var roles []map[string]any
	for _, role := range profile.Roles {
		roles = append(roles, map[string]any{
			"name": aws.ToString(role.RoleName),
			"arn":  aws.ToString(role.Arn),
			"path": aws.ToString(role.Path),
		})
	}
	return map[string]any{
		"name":    aws.ToString(profile.InstanceProfileName),
		"arn":     aws.ToString(profile.Arn),
		"path":    aws.ToString(profile.Path),
		"roles":   roles,
		"created": profile.CreateDate,
	}
}

func summarizePolicy(policy iamtypes.Policy) map[string]any {
	return map[string]any{
		"name":              aws.ToString(policy.PolicyName),
		"arn":               aws.ToString(policy.Arn),
		"id":                aws.ToString(policy.PolicyId),
		"path":              aws.ToString(policy.Path),
		"defaultVersionId":  aws.ToString(policy.DefaultVersionId),
		"attachmentCount":   policy.AttachmentCount,
		"isAttachable":      policy.IsAttachable,
		"createDate":        policy.CreateDate,
		"updateDate":        policy.UpdateDate,
		"permissionsBound":  policy.PermissionsBoundaryUsageCount,
		"description":       aws.ToString(policy.Description),
		"lastPolicyVersion": policy.DefaultVersionId,
	}
}

func parseJSONOrString(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	var out any
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out
	}
	return value
}

func decodePolicyDocument(value string) string {
	if value == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func requireConfirm(args map[string]any) error {
	if val, ok := args["confirm"].(bool); ok && val {
		return nil
	}
	return errors.New("confirmation required: set confirm=true to proceed")
}

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func toInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func toBool(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func regionOrDefault(region string) string {
	if strings.TrimSpace(region) == "" {
		return "us-east-1"
	}
	return region
}
