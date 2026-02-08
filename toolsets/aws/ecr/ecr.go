package awsecr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	ecrClient func(context.Context, string) (*ecr.Client, string, error)
	toolsetID string
}

func ToolSpecs(ctx mcp.ToolsetContext, toolsetID string, ecrClient func(context.Context, string) (*ecr.Client, string, error)) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, ecrClient: ecrClient, toolsetID: toolsetID}
	return []mcp.ToolSpec{
		{
			Name:        "aws.ecr.list_repositories",
			Description: "List ECR repositories.",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRListRepositories(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListRepositories,
		},
		{
			Name:        "aws.ecr.describe_repository",
			Description: "Describe an ECR repository by name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRDescribeRepository(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleDescribeRepository,
		},
		{
			Name:        "aws.ecr.list_images",
			Description: "List image identifiers in an ECR repository.",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRListImages(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListImages,
		},
		{
			Name:        "aws.ecr.describe_images",
			Description: "Describe images in an ECR repository.",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRDescribeImages(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleDescribeImages,
		},
		{
			Name:        "aws.ecr.describe_registry",
			Description: "Describe the current ECR registry.",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRDescribeRegistry(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleDescribeRegistry,
		},
		{
			Name:        "aws.ecr.get_authorization_token",
			Description: "Get an ECR authorization token (confirm required).",
			ToolsetID:   toolsetID,
			InputSchema: schemaECRGetAuthorizationToken(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     svc.handleGetAuthorizationToken,
		},
	}
}

func (s *Service) handleListRepositories(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ecr.DescribeRepositoriesInput{}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	paginator := ecr.NewDescribeRepositoriesPaginator(client, input)
	var repos []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, repo := range out.Repositories {
			repos = append(repos, summarizeRepository(repo))
			if limit > 0 && len(repos) >= limit {
				repos = repos[:limit]
				return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
					"region":       usedRegion,
					"repositories": repos,
				})}, nil
			}
		}
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":       usedRegion,
		"repositories": repos,
	})}, nil
}

func (s *Service) handleDescribeRepository(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	repoName := strings.TrimSpace(toString(req.Arguments["repositoryName"]))
	if repoName == "" {
		return errorResult(errors.New("repositoryName is required")), errors.New("repositoryName is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repoName},
	})
	if err != nil {
		return errorResult(err), err
	}
	var repo map[string]any
	if len(out.Repositories) > 0 {
		repo = summarizeRepository(out.Repositories[0])
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":     usedRegion,
		"repository": repo,
	})}, nil
}

func (s *Service) handleListImages(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	repoName := strings.TrimSpace(toString(req.Arguments["repositoryName"]))
	if repoName == "" {
		return errorResult(errors.New("repositoryName is required")), errors.New("repositoryName is required")
	}
	tagStatus := toString(req.Arguments["tagStatus"])
	limit := toInt(req.Arguments["limit"], 100)
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ecr.ListImagesInput{
		RepositoryName: aws.String(repoName),
	}
	if tagStatus != "" {
		input.Filter = &ecrtypes.ListImagesFilter{TagStatus: ecrtypes.TagStatus(tagStatus)}
	}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	paginator := ecr.NewListImagesPaginator(client, input)
	var images []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, image := range out.ImageIds {
			images = append(images, summarizeImageID(image))
			if limit > 0 && len(images) >= limit {
				images = images[:limit]
				return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
					"region":         usedRegion,
					"repositoryName": repoName,
					"images":         images,
				})}, nil
			}
		}
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":         usedRegion,
		"repositoryName": repoName,
		"images":         images,
	})}, nil
}

func (s *Service) handleDescribeImages(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	repoName := strings.TrimSpace(toString(req.Arguments["repositoryName"]))
	if repoName == "" {
		return errorResult(errors.New("repositoryName is required")), errors.New("repositoryName is required")
	}
	tagStatus := toString(req.Arguments["tagStatus"])
	limit := toInt(req.Arguments["limit"], 100)
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	ids := toImageIdentifiers(toStringSlice(req.Arguments["imageTags"]), toStringSlice(req.Arguments["imageDigests"]))
	input := &ecr.DescribeImagesInput{RepositoryName: aws.String(repoName)}
	if len(ids) > 0 {
		input.ImageIds = ids
	}
	if tagStatus != "" {
		input.Filter = &ecrtypes.DescribeImagesFilter{TagStatus: ecrtypes.TagStatus(tagStatus)}
	}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	paginator := ecr.NewDescribeImagesPaginator(client, input)
	var images []map[string]any
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResult(err), err
		}
		for _, detail := range out.ImageDetails {
			images = append(images, summarizeImageDetail(detail))
			if limit > 0 && len(images) >= limit {
				images = images[:limit]
				return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
					"region":         usedRegion,
					"repositoryName": repoName,
					"images":         images,
				})}, nil
			}
		}
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":         usedRegion,
		"repositoryName": repoName,
		"images":         images,
	})}, nil
}

func (s *Service) handleDescribeRegistry(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeRegistry(ctx, &ecr.DescribeRegistryInput{})
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":                   usedRegion,
		"registryId":               aws.ToString(out.RegistryId),
		"replicationConfiguration": out.ReplicationConfiguration,
	})}, nil
}

func (s *Service) handleGetAuthorizationToken(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := requireConfirm(req.Arguments); err != nil {
		return errorResult(err), err
	}
	registryIDs := toStringSlice(req.Arguments["registryIds"])
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ecrClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ecr.GetAuthorizationTokenInput{}
	if len(registryIDs) > 0 {
		input.RegistryIds = registryIDs
	}
	out, err := client.GetAuthorizationToken(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	tokens := make([]map[string]any, 0, len(out.AuthorizationData))
	for _, auth := range out.AuthorizationData {
		tokens = append(tokens, map[string]any{
			"authorizationToken": aws.ToString(auth.AuthorizationToken),
			"proxyEndpoint":      aws.ToString(auth.ProxyEndpoint),
			"expiresAt":          aws.ToTime(auth.ExpiresAt),
		})
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(map[string]any{
		"region":      usedRegion,
		"tokens":      tokens,
		"registryIds": registryIDs,
	})}, nil
}

func summarizeRepository(repo ecrtypes.Repository) map[string]any {
	out := map[string]any{
		"repositoryName": aws.ToString(repo.RepositoryName),
		"repositoryArn":  aws.ToString(repo.RepositoryArn),
		"registryId":     aws.ToString(repo.RegistryId),
		"repositoryUri":  aws.ToString(repo.RepositoryUri),
		"createdAt":      aws.ToTime(repo.CreatedAt),
	}
	if repo.ImageTagMutability != "" {
		out["imageTagMutability"] = string(repo.ImageTagMutability)
	}
	if repo.ImageScanningConfiguration != nil {
		out["scanOnPush"] = repo.ImageScanningConfiguration.ScanOnPush
	}
	return out
}

func summarizeImageID(id ecrtypes.ImageIdentifier) map[string]any {
	return map[string]any{
		"imageDigest": aws.ToString(id.ImageDigest),
		"imageTag":    aws.ToString(id.ImageTag),
	}
}

func summarizeImageDetail(detail ecrtypes.ImageDetail) map[string]any {
	return map[string]any{
		"imageDigest":       aws.ToString(detail.ImageDigest),
		"imageTags":         detail.ImageTags,
		"imagePushedAt":     aws.ToTime(detail.ImagePushedAt),
		"imageSizeInBytes":  detail.ImageSizeInBytes,
		"artifactMediaType": aws.ToString(detail.ArtifactMediaType),
	}
}

func toImageIdentifiers(tags []string, digests []string) []ecrtypes.ImageIdentifier {
	ids := make([]ecrtypes.ImageIdentifier, 0, len(tags)+len(digests))
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			continue
		}
		ids = append(ids, ecrtypes.ImageIdentifier{ImageTag: aws.String(tag)})
	}
	for _, digest := range digests {
		if strings.TrimSpace(digest) == "" {
			continue
		}
		ids = append(ids, ecrtypes.ImageIdentifier{ImageDigest: aws.String(digest)})
	}
	return ids
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

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
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
