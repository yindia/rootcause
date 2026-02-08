package awssts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	stsClient func(context.Context, string) (*sts.Client, string, error)
	toolsetID string
}

func ToolSpecs(ctx mcp.ToolsetContext, toolsetID string, stsClient func(context.Context, string) (*sts.Client, string, error)) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, stsClient: stsClient, toolsetID: toolsetID}
	return []mcp.ToolSpec{
		{
			Name:        "aws.sts.get_caller_identity",
			Description: "Get AWS account and caller identity.",
			ToolsetID:   toolsetID,
			InputSchema: schemaSTSGetCallerIdentity(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetCallerIdentity,
		},
		{
			Name:        "aws.sts.assume_role",
			Description: "Assume an IAM role and return temporary credentials (confirm required).",
			ToolsetID:   toolsetID,
			InputSchema: schemaSTSAssumeRole(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     svc.handleAssumeRole,
		},
	}
}

func (s *Service) handleGetCallerIdentity(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.stsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":  usedRegion,
		"arn":     aws.ToString(out.Arn),
		"account": aws.ToString(out.Account),
		"userId":  aws.ToString(out.UserId),
	})}, nil
}

func (s *Service) handleAssumeRole(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	roleArn := strings.TrimSpace(toString(req.Arguments["roleArn"]))
	if roleArn == "" {
		return errorResult(errors.New("roleArn is required")), errors.New("roleArn is required")
	}
	sessionName := strings.TrimSpace(toString(req.Arguments["sessionName"]))
	if sessionName == "" {
		return errorResult(errors.New("sessionName is required")), errors.New("sessionName is required")
	}
	region := toString(req.Arguments["region"])
	duration := toInt(req.Arguments["durationSeconds"], 0)
	externalID := strings.TrimSpace(toString(req.Arguments["externalId"]))
	policy := strings.TrimSpace(toString(req.Arguments["policy"]))
	client, usedRegion, err := s.stsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	}
	if duration > 0 {
		input.DurationSeconds = aws.Int32(int32(duration))
	}
	if externalID != "" {
		input.ExternalId = aws.String(externalID)
	}
	if policy != "" {
		input.Policy = aws.String(policy)
	}
	out, err := client.AssumeRole(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":           usedRegion,
		"assumedRoleUser":  summarizeAssumedRoleUser(out.AssumedRoleUser),
		"credentials":      summarizeCredentials(out.Credentials),
		"sourceIdentity":   aws.ToString(out.SourceIdentity),
		"packedPolicySize": out.PackedPolicySize,
	})}, nil
}

func summarizeAssumedRoleUser(user *ststypes.AssumedRoleUser) map[string]any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"arn":           aws.ToString(user.Arn),
		"assumedRoleId": aws.ToString(user.AssumedRoleId),
	}
}

func summarizeCredentials(creds *ststypes.Credentials) map[string]any {
	if creds == nil {
		return nil
	}
	return map[string]any{
		"accessKeyId":     aws.ToString(creds.AccessKeyId),
		"secretAccessKey": aws.ToString(creds.SecretAccessKey),
		"sessionToken":    aws.ToString(creds.SessionToken),
		"expiration":      aws.ToTime(creds.Expiration),
	}
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

func requireConfirm(args map[string]any) error {
	if val, ok := args["confirm"].(bool); ok && val {
		return nil
	}
	return errors.New("confirmation required: set confirm=true to proceed")
}
