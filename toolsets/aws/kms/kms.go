package awskms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	kmsClient func(context.Context, string) (*kms.Client, string, error)
	toolsetID string
}

func ToolSpecs(ctx mcp.ToolsetContext, toolsetID string, kmsClient func(context.Context, string) (*kms.Client, string, error)) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, kmsClient: kmsClient, toolsetID: toolsetID}
	return []mcp.ToolSpec{
		{
			Name:        "aws.kms.list_keys",
			Description: "List KMS keys.",
			ToolsetID:   toolsetID,
			InputSchema: schemaKMSListKeys(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListKeys,
		},
		{
			Name:        "aws.kms.list_aliases",
			Description: "List KMS aliases.",
			ToolsetID:   toolsetID,
			InputSchema: schemaKMSListAliases(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListAliases,
		},
		{
			Name:        "aws.kms.describe_key",
			Description: "Describe a KMS key by id or ARN.",
			ToolsetID:   toolsetID,
			InputSchema: schemaKMSDescribeKey(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleDescribeKey,
		},
		{
			Name:        "aws.kms.get_key_policy",
			Description: "Get a KMS key policy (default policy name if omitted).",
			ToolsetID:   toolsetID,
			InputSchema: schemaKMSGetKeyPolicy(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetKeyPolicy,
		},
	}
}

func (s *Service) handleListKeys(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.kmsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &kms.ListKeysInput{}
	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}
	paginator := kms.NewListKeysPaginator(client, input)
	var keys []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, key := range out.Keys {
			keys = append(keys, summarizeKeyListEntry(key))
			if limit > 0 && len(keys) >= limit {
				keys = keys[:limit]
				return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
					"region": usedRegion,
					"keys":   keys,
				})}, nil
			}
		}
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region": usedRegion,
		"keys":   keys,
	})}, nil
}

func (s *Service) handleListAliases(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.kmsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &kms.ListAliasesInput{}
	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}
	paginator := kms.NewListAliasesPaginator(client, input)
	var aliases []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, alias := range out.Aliases {
			aliases = append(aliases, summarizeAlias(alias))
			if limit > 0 && len(aliases) >= limit {
				aliases = aliases[:limit]
				return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
					"region":  usedRegion,
					"aliases": aliases,
				})}, nil
			}
		}
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":  usedRegion,
		"aliases": aliases,
	})}, nil
}

func (s *Service) handleDescribeKey(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	keyID := strings.TrimSpace(toString(req.Arguments["keyId"]))
	if keyID == "" {
		return errorResult(errors.New("keyId is required")), errors.New("keyId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.kmsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region": usedRegion,
		"key":    summarizeKeyMetadata(out.KeyMetadata),
	})}, nil
}

func (s *Service) handleGetKeyPolicy(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	keyID := strings.TrimSpace(toString(req.Arguments["keyId"]))
	if keyID == "" {
		return errorResult(errors.New("keyId is required")), errors.New("keyId is required")
	}
	policyName := strings.TrimSpace(toString(req.Arguments["policyName"]))
	if policyName == "" {
		policyName = "default"
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.kmsClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
		KeyId:      aws.String(keyID),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":     usedRegion,
		"keyId":      keyID,
		"policyName": policyName,
		"policy":     aws.ToString(out.Policy),
	})}, nil
}

func summarizeKeyListEntry(entry kmstypes.KeyListEntry) map[string]any {
	return map[string]any{
		"keyId":  aws.ToString(entry.KeyId),
		"keyArn": aws.ToString(entry.KeyArn),
	}
}

func summarizeAlias(alias kmstypes.AliasListEntry) map[string]any {
	return map[string]any{
		"aliasName":   aws.ToString(alias.AliasName),
		"aliasArn":    aws.ToString(alias.AliasArn),
		"targetKeyId": aws.ToString(alias.TargetKeyId),
	}
}

func summarizeKeyMetadata(meta *kmstypes.KeyMetadata) map[string]any {
	if meta == nil {
		return nil
	}
	return map[string]any{
		"keyId":        aws.ToString(meta.KeyId),
		"arn":          aws.ToString(meta.Arn),
		"awsAccountId": aws.ToString(meta.AWSAccountId),
		"description":  aws.ToString(meta.Description),
		"keyState":     string(meta.KeyState),
		"keyUsage":     string(meta.KeyUsage),
		"origin":       string(meta.Origin),
		"multiRegion":  meta.MultiRegion,
		"creationDate": aws.ToTime(meta.CreationDate),
		"enabled":      meta.Enabled,
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
