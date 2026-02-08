package awseks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	eksClient func(context.Context, string) (*eks.Client, string, error)
	ec2Client func(context.Context, string) (*ec2.Client, string, error)
	asgClient func(context.Context, string) (*autoscaling.Client, string, error)
	ecrClient func(context.Context, string) (*ecr.Client, string, error)
	kmsClient func(context.Context, string) (*kms.Client, string, error)
	stsClient func(context.Context, string) (*sts.Client, string, error)
	toolsetID string
}

func ToolSpecs(
	ctx mcp.ToolsetContext,
	toolsetID string,
	eksClient func(context.Context, string) (*eks.Client, string, error),
	ec2Client func(context.Context, string) (*ec2.Client, string, error),
	asgClient func(context.Context, string) (*autoscaling.Client, string, error),
	ecrClient func(context.Context, string) (*ecr.Client, string, error),
	kmsClient func(context.Context, string) (*kms.Client, string, error),
	stsClient func(context.Context, string) (*sts.Client, string, error),
) []mcp.ToolSpec {
	svc := &Service{
		ctx:       ctx,
		eksClient: eksClient,
		ec2Client: ec2Client,
		asgClient: asgClient,
		ecrClient: ecrClient,
		kmsClient: kmsClient,
		stsClient: stsClient,
		toolsetID: toolsetID,
	}
	return []mcp.ToolSpec{
		{Name: "aws.eks.list_clusters", Description: "List EKS clusters.", ToolsetID: toolsetID, InputSchema: schemaEKSListClusters(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListClusters},
		{Name: "aws.eks.get_cluster", Description: "Get an EKS cluster by name.", ToolsetID: toolsetID, InputSchema: schemaEKSGetCluster(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetCluster},
		{Name: "aws.eks.list_nodegroups", Description: "List EKS nodegroups for a cluster.", ToolsetID: toolsetID, InputSchema: schemaEKSListNodegroups(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListNodegroups},
		{Name: "aws.eks.get_nodegroup", Description: "Get an EKS nodegroup by name.", ToolsetID: toolsetID, InputSchema: schemaEKSGetNodegroup(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetNodegroup},
		{Name: "aws.eks.list_addons", Description: "List EKS addons for a cluster.", ToolsetID: toolsetID, InputSchema: schemaEKSListAddons(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListAddons},
		{Name: "aws.eks.get_addon", Description: "Get an EKS addon by name.", ToolsetID: toolsetID, InputSchema: schemaEKSGetAddon(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetAddon},
		{Name: "aws.eks.list_fargate_profiles", Description: "List EKS fargate profiles for a cluster.", ToolsetID: toolsetID, InputSchema: schemaEKSListFargateProfiles(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListFargateProfiles},
		{Name: "aws.eks.get_fargate_profile", Description: "Get an EKS fargate profile by name.", ToolsetID: toolsetID, InputSchema: schemaEKSGetFargateProfile(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetFargateProfile},
		{Name: "aws.eks.list_identity_provider_configs", Description: "List EKS identity provider configs.", ToolsetID: toolsetID, InputSchema: schemaEKSListIdentityProviderConfigs(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListIdentityProviderConfigs},
		{Name: "aws.eks.get_identity_provider_config", Description: "Get an EKS identity provider config.", ToolsetID: toolsetID, InputSchema: schemaEKSGetIdentityProviderConfig(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetIdentityProviderConfig},
		{Name: "aws.eks.list_updates", Description: "List EKS updates for a cluster or nodegroup.", ToolsetID: toolsetID, InputSchema: schemaEKSListUpdates(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListUpdates},
		{Name: "aws.eks.get_update", Description: "Get an EKS update by id.", ToolsetID: toolsetID, InputSchema: schemaEKSGetUpdate(), Safety: mcp.SafetyReadOnly, Handler: svc.handleGetUpdate},
		{Name: "aws.eks.list_nodes", Description: "List EC2 instances backing EKS nodegroups.", ToolsetID: toolsetID, InputSchema: schemaEKSListNodes(), Safety: mcp.SafetyReadOnly, Handler: svc.handleListNodes},
		{Name: "aws.eks.debug", Description: "Debug an EKS cluster with optional STS/KMS/ECR checks.", ToolsetID: toolsetID, InputSchema: schemaEKSDebug(), Safety: mcp.SafetyReadOnly, Handler: svc.handleDebug},
	}
}

func (s *Service) handleListClusters(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListClustersInput{}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var clusters []string
	for {
		out, err := client.ListClusters(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		clusters = append(clusters, out.Clusters...)
		if limit > 0 && len(clusters) >= limit {
			clusters = clusters[:limit]
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"clusters": clusters,
		"count":    len(clusters),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	clusterName := strings.TrimSpace(toString(req.Arguments["clusterName"]))
	if clusterName == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	region := toString(req.Arguments["region"])
	includeSts := true
	if val, ok := req.Arguments["includeSts"].(bool); ok {
		includeSts = val
	}
	includeKms := true
	if val, ok := req.Arguments["includeKms"].(bool); ok {
		includeKms = val
	}
	includeEcr := false
	if val, ok := req.Arguments["includeEcr"].(bool); ok {
		includeEcr = val
	}
	repoName := strings.TrimSpace(toString(req.Arguments["repositoryName"]))
	if repoName != "" {
		includeEcr = true
	}
	imageLimit := toInt(req.Arguments["imageLimit"], 50)
	imageTags := toStringSlice(req.Arguments["imageTags"])
	imageDigests := toStringSlice(req.Arguments["imageDigests"])

	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		return errorResult(err), err
	}

	diagnostics := map[string]any{}
	var warnings []string

	if includeSts {
		if s.stsClient == nil {
			warnings = append(warnings, "sts client not configured")
		} else {
			stsClient, _, err := s.stsClient(ctx, usedRegion)
			if err != nil {
				warnings = append(warnings, err.Error())
			} else {
				identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
				if err != nil {
					warnings = append(warnings, err.Error())
				} else {
					diagnostics["sts"] = map[string]any{
						"arn":     aws.ToString(identity.Arn),
						"account": aws.ToString(identity.Account),
						"userId":  aws.ToString(identity.UserId),
					}
				}
			}
		}
	}

	if includeKms {
		keys := []map[string]any{}
		if out.Cluster != nil {
			for _, enc := range out.Cluster.EncryptionConfig {
				if enc.Provider == nil || strings.TrimSpace(aws.ToString(enc.Provider.KeyArn)) == "" {
					continue
				}
				keyArn := aws.ToString(enc.Provider.KeyArn)
				if s.kmsClient == nil {
					warnings = append(warnings, "kms client not configured")
					break
				}
				kmsClient, _, err := s.kmsClient(ctx, usedRegion)
				if err != nil {
					warnings = append(warnings, err.Error())
					break
				}
				desc, err := kmsClient.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keyArn)})
				if err != nil {
					warnings = append(warnings, err.Error())
					continue
				}
				keys = append(keys, summarizeKMSKey(desc.KeyMetadata))
			}
		}
		diagnostics["kmsKeys"] = keys
	}

	if includeEcr {
		if s.ecrClient == nil {
			warnings = append(warnings, "ecr client not configured")
		} else {
			ecrClient, _, err := s.ecrClient(ctx, usedRegion)
			if err != nil {
				warnings = append(warnings, err.Error())
			} else {
				registry, err := ecrClient.DescribeRegistry(ctx, &ecr.DescribeRegistryInput{})
				if err != nil {
					warnings = append(warnings, err.Error())
				} else {
					diagnostics["ecrRegistry"] = map[string]any{
						"registryId": aws.ToString(registry.RegistryId),
					}
				}
				if repoName != "" {
					repoOut, err := ecrClient.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
						RepositoryNames: []string{repoName},
					})
					if err != nil {
						warnings = append(warnings, err.Error())
					} else if len(repoOut.Repositories) > 0 {
						diagnostics["ecrRepository"] = summarizeECRRepository(repoOut.Repositories[0])
					}
					if len(imageTags) > 0 || len(imageDigests) > 0 {
						imageOut, err := ecrClient.DescribeImages(ctx, &ecr.DescribeImagesInput{
							RepositoryName: aws.String(repoName),
							ImageIds:       toECRImageIdentifiers(imageTags, imageDigests),
							MaxResults:     aws.Int32(int32(imageLimit)),
						})
						if err != nil {
							warnings = append(warnings, err.Error())
						} else {
							images := make([]map[string]any, 0, len(imageOut.ImageDetails))
							for _, detail := range imageOut.ImageDetails {
								images = append(images, summarizeECRImageDetail(detail))
							}
							diagnostics["ecrImages"] = images
						}
					} else {
						imageOut, err := ecrClient.ListImages(ctx, &ecr.ListImagesInput{
							RepositoryName: aws.String(repoName),
							MaxResults:     aws.Int32(int32(imageLimit)),
						})
						if err != nil {
							warnings = append(warnings, err.Error())
						} else {
							images := make([]map[string]any, 0, len(imageOut.ImageIds))
							for _, img := range imageOut.ImageIds {
								images = append(images, summarizeECRImageID(img))
							}
							diagnostics["ecrImages"] = images
						}
					}
				}
			}
		}
	}

	data := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"cluster":     summarizeCluster(*out.Cluster),
		"diagnostics": diagnostics,
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetCluster(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	name := toString(req.Arguments["name"])
	if name == "" {
		return errorResult(errors.New("name is required")), errors.New("name is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
	if err != nil {
		return errorResult(err), err
	}
	if out.Cluster == nil {
		return errorResult(fmt.Errorf("cluster %s not found", name)), fmt.Errorf("cluster %s not found", name)
	}
	result := map[string]any{
		"region":  regionOrDefault(usedRegion),
		"cluster": summarizeCluster(*out.Cluster),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/cluster/%s", name)},
		},
	}, nil
}

func (s *Service) handleListNodegroups(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListNodegroupsInput{ClusterName: aws.String(cluster)}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var groups []string
	for {
		out, err := client.ListNodegroups(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		groups = append(groups, out.Nodegroups...)
		if limit > 0 && len(groups) >= limit {
			groups = groups[:limit]
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"nodegroups": groups,
		"count":      len(groups),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetNodegroup(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	name := toString(req.Arguments["nodegroupName"])
	if cluster == "" || name == "" {
		return errorResult(errors.New("clusterName and nodegroupName are required")), errors.New("clusterName and nodegroupName are required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(cluster),
		NodegroupName: aws.String(name),
	})
	if err != nil {
		return errorResult(err), err
	}
	if out.Nodegroup == nil {
		return errorResult(fmt.Errorf("nodegroup %s not found", name)), fmt.Errorf("nodegroup %s not found", name)
	}
	result := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"nodegroup": summarizeNodegroup(*out.Nodegroup),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/nodegroup/%s/%s", cluster, name)},
		},
	}, nil
}

func (s *Service) handleListAddons(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListAddonsInput{ClusterName: aws.String(cluster)}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var addons []string
	for {
		out, err := client.ListAddons(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		addons = append(addons, out.Addons...)
		if limit > 0 && len(addons) >= limit {
			addons = addons[:limit]
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region": regionOrDefault(usedRegion),
		"addons": addons,
		"count":  len(addons),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetAddon(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	name := toString(req.Arguments["addonName"])
	if cluster == "" || name == "" {
		return errorResult(errors.New("clusterName and addonName are required")), errors.New("clusterName and addonName are required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeAddon(ctx, &eks.DescribeAddonInput{
		ClusterName: aws.String(cluster),
		AddonName:   aws.String(name),
	})
	if err != nil {
		return errorResult(err), err
	}
	if out.Addon == nil {
		return errorResult(fmt.Errorf("addon %s not found", name)), fmt.Errorf("addon %s not found", name)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"addon":  summarizeAddon(*out.Addon),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/addon/%s/%s", cluster, name)},
		},
	}, nil
}

func (s *Service) handleListFargateProfiles(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListFargateProfilesInput{ClusterName: aws.String(cluster)}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var profiles []string
	for {
		out, err := client.ListFargateProfiles(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		profiles = append(profiles, out.FargateProfileNames...)
		if limit > 0 && len(profiles) >= limit {
			profiles = profiles[:limit]
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"fargateProfiles": profiles,
		"count":           len(profiles),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetFargateProfile(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	name := toString(req.Arguments["profileName"])
	if cluster == "" || name == "" {
		return errorResult(errors.New("clusterName and profileName are required")), errors.New("clusterName and profileName are required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeFargateProfile(ctx, &eks.DescribeFargateProfileInput{
		ClusterName:        aws.String(cluster),
		FargateProfileName: aws.String(name),
	})
	if err != nil {
		return errorResult(err), err
	}
	if out.FargateProfile == nil {
		return errorResult(fmt.Errorf("fargate profile %s not found", name)), fmt.Errorf("fargate profile %s not found", name)
	}
	result := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"fargateProfile": summarizeFargateProfile(*out.FargateProfile),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/fargate/%s/%s", cluster, name)},
		},
	}, nil
}

func (s *Service) handleListIdentityProviderConfigs(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListIdentityProviderConfigsInput{ClusterName: aws.String(cluster)}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var configs []map[string]any
	for {
		out, err := client.ListIdentityProviderConfigs(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, cfg := range out.IdentityProviderConfigs {
			configs = append(configs, map[string]any{
				"type": aws.ToString(cfg.Type),
				"name": aws.ToString(cfg.Name),
			})
			if limit > 0 && len(configs) >= limit {
				break
			}
		}
		if limit > 0 && len(configs) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":                  regionOrDefault(usedRegion),
		"identityProviderConfigs": configs,
		"count":                   len(configs),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetIdentityProviderConfig(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	idpType := toString(req.Arguments["type"])
	name := toString(req.Arguments["name"])
	if cluster == "" || idpType == "" || name == "" {
		return errorResult(errors.New("clusterName, type, and name are required")), errors.New("clusterName, type, and name are required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeIdentityProviderConfig(ctx, &eks.DescribeIdentityProviderConfigInput{
		ClusterName: aws.String(cluster),
		IdentityProviderConfig: &ekstypes.IdentityProviderConfig{
			Type: aws.String(idpType),
			Name: aws.String(name),
		},
	})
	if err != nil {
		return errorResult(err), err
	}
	if out.IdentityProviderConfig == nil {
		return errorResult(fmt.Errorf("identity provider config %s not found", name)), fmt.Errorf("identity provider config %s not found", name)
	}
	result := map[string]any{
		"region":                 regionOrDefault(usedRegion),
		"identityProviderConfig": summarizeIdentityProviderConfig(*out.IdentityProviderConfig),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/idp/%s/%s", cluster, name)},
		},
	}, nil
}

func (s *Service) handleListUpdates(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	nodegroup := toString(req.Arguments["nodegroupName"])
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.ListUpdatesInput{Name: aws.String(cluster)}
	if nodegroup != "" {
		input.NodegroupName = aws.String(nodegroup)
	}
	if limit > 0 {
		input.MaxResults = aws.Int32(int32(limit))
	}
	var updates []string
	for {
		out, err := client.ListUpdates(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		updates = append(updates, out.UpdateIds...)
		if limit > 0 && len(updates) >= limit {
			updates = updates[:limit]
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":  regionOrDefault(usedRegion),
		"updates": updates,
		"count":   len(updates),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetUpdate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	updateID := toString(req.Arguments["updateId"])
	if cluster == "" || updateID == "" {
		return errorResult(errors.New("clusterName and updateId are required")), errors.New("clusterName and updateId are required")
	}
	nodegroup := toString(req.Arguments["nodegroupName"])
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &eks.DescribeUpdateInput{Name: aws.String(cluster), UpdateId: aws.String(updateID)}
	if nodegroup != "" {
		input.NodegroupName = aws.String(nodegroup)
	}
	out, err := client.DescribeUpdate(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if out.Update == nil {
		return errorResult(fmt.Errorf("update %s not found", updateID)), fmt.Errorf("update %s not found", updateID)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"update": summarizeUpdate(*out.Update),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("eks/update/%s", updateID)},
		},
	}, nil
}

func (s *Service) handleListNodes(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	cluster := toString(req.Arguments["clusterName"])
	if cluster == "" {
		return errorResult(errors.New("clusterName is required")), errors.New("clusterName is required")
	}
	nodegroup := toString(req.Arguments["nodegroupName"])
	region := toString(req.Arguments["region"])
	limit := toInt(req.Arguments["limit"], 100)
	eksClient, usedRegion, err := s.eksClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	asgClient, _, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	ec2Client, _, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	nodegroups := []string{}
	if nodegroup != "" {
		nodegroups = []string{nodegroup}
	} else {
		listOut, err := eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(cluster)})
		if err != nil {
			return errorResult(err), err
		}
		nodegroups = append(nodegroups, listOut.Nodegroups...)
	}
	var instances []map[string]any
	var warnings []string
	for _, ng := range nodegroups {
		descOut, err := eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(cluster),
			NodegroupName: aws.String(ng),
		})
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if descOut.Nodegroup == nil || descOut.Nodegroup.Resources == nil {
			continue
		}
		asgNames := []string{}
		for _, asg := range descOut.Nodegroup.Resources.AutoScalingGroups {
			if asg.Name != nil {
				asgNames = append(asgNames, *asg.Name)
			}
		}
		if len(asgNames) == 0 {
			continue
		}
		asgOut, err := asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgNames,
		})
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		var instanceIDs []string
		for _, group := range asgOut.AutoScalingGroups {
			for _, inst := range group.Instances {
				if inst.InstanceId != nil {
					instanceIDs = append(instanceIDs, *inst.InstanceId)
				}
			}
		}
		if len(instanceIDs) == 0 {
			continue
		}
		ec2Out, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: instanceIDs})
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		for _, res := range ec2Out.Reservations {
			for _, inst := range res.Instances {
				instances = append(instances, summarizeInstance(inst, ng))
				if limit > 0 && len(instances) >= limit {
					break
				}
			}
			if limit > 0 && len(instances) >= limit {
				break
			}
		}
		if limit > 0 && len(instances) >= limit {
			break
		}
	}
	data := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"cluster":   cluster,
		"nodegroup": nodegroup,
		"instances": instances,
		"count":     len(instances),
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func summarizeCluster(cluster ekstypes.Cluster) map[string]any {
	logging := map[string]any{}
	if cluster.Logging != nil {
		logging["clusterLogging"] = cluster.Logging.ClusterLogging
	}
	return map[string]any{
		"name":                    aws.ToString(cluster.Name),
		"arn":                     aws.ToString(cluster.Arn),
		"version":                 aws.ToString(cluster.Version),
		"status":                  cluster.Status,
		"endpoint":                aws.ToString(cluster.Endpoint),
		"roleArn":                 aws.ToString(cluster.RoleArn),
		"createdAt":               cluster.CreatedAt,
		"platformVersion":         aws.ToString(cluster.PlatformVersion),
		"vpcConfig":               cluster.ResourcesVpcConfig,
		"kubernetesNetworkConfig": cluster.KubernetesNetworkConfig,
		"identity":                cluster.Identity,
		"logging":                 logging,
		"tags":                    cluster.Tags,
	}
}

func summarizeNodegroup(group ekstypes.Nodegroup) map[string]any {
	return map[string]any{
		"name":           aws.ToString(group.NodegroupName),
		"arn":            aws.ToString(group.NodegroupArn),
		"status":         group.Status,
		"version":        aws.ToString(group.Version),
		"capacityType":   group.CapacityType,
		"amiType":        group.AmiType,
		"releaseVersion": aws.ToString(group.ReleaseVersion),
		"nodeRole":       aws.ToString(group.NodeRole),
		"subnets":        group.Subnets,
		"instanceTypes":  group.InstanceTypes,
		"labels":         group.Labels,
		"taints":         group.Taints,
		"scalingConfig":  group.ScalingConfig,
		"remoteAccess":   group.RemoteAccess,
		"resources":      group.Resources,
		"tags":           group.Tags,
	}
}

func summarizeAddon(addon ekstypes.Addon) map[string]any {
	var issues []map[string]any
	for _, issue := range addon.Health.Issues {
		issues = append(issues, map[string]any{
			"code":        issue.Code,
			"message":     aws.ToString(issue.Message),
			"resourceIds": issue.ResourceIds,
		})
	}
	return map[string]any{
		"name":                  aws.ToString(addon.AddonName),
		"version":               aws.ToString(addon.AddonVersion),
		"status":                addon.Status,
		"serviceAccountRoleArn": aws.ToString(addon.ServiceAccountRoleArn),
		"health":                issues,
		"configurationValues":   addon.ConfigurationValues,
	}
}

func summarizeFargateProfile(profile ekstypes.FargateProfile) map[string]any {
	return map[string]any{
		"name":             aws.ToString(profile.FargateProfileName),
		"arn":              aws.ToString(profile.FargateProfileArn),
		"status":           profile.Status,
		"selectors":        profile.Selectors,
		"subnets":          profile.Subnets,
		"podExecutionRole": aws.ToString(profile.PodExecutionRoleArn),
		"createdAt":        profile.CreatedAt,
		"tags":             profile.Tags,
	}
}

func summarizeIdentityProviderConfig(cfg ekstypes.IdentityProviderConfigResponse) map[string]any {
	out := map[string]any{
		"type": "oidc",
	}
	if cfg.Oidc != nil {
		out["oidc"] = cfg.Oidc
	}
	return out
}

func summarizeUpdate(update ekstypes.Update) map[string]any {
	return map[string]any{
		"id":        aws.ToString(update.Id),
		"type":      update.Type,
		"status":    update.Status,
		"params":    update.Params,
		"errors":    update.Errors,
		"createdAt": update.CreatedAt,
	}
}

func summarizeInstance(inst ec2types.Instance, nodegroup string) map[string]any {
	var sgIDs []string
	for _, sg := range inst.SecurityGroups {
		sgIDs = append(sgIDs, aws.ToString(sg.GroupId))
	}
	return map[string]any{
		"id":               aws.ToString(inst.InstanceId),
		"state":            inst.State,
		"type":             inst.InstanceType,
		"vpcId":            aws.ToString(inst.VpcId),
		"subnetId":         aws.ToString(inst.SubnetId),
		"availabilityZone": aws.ToString(inst.Placement.AvailabilityZone),
		"privateIp":        aws.ToString(inst.PrivateIpAddress),
		"publicIp":         aws.ToString(inst.PublicIpAddress),
		"nodegroup":        nodegroup,
		"securityGroupIds": sgIDs,
		"launchTime":       inst.LaunchTime,
		"tags":             tagMap(inst.Tags),
	}
}

func summarizeKMSKey(meta *kmstypes.KeyMetadata) map[string]any {
	if meta == nil {
		return nil
	}
	return map[string]any{
		"keyId":        aws.ToString(meta.KeyId),
		"arn":          aws.ToString(meta.Arn),
		"accountId":    aws.ToString(meta.AWSAccountId),
		"description":  aws.ToString(meta.Description),
		"keyState":     string(meta.KeyState),
		"keyUsage":     string(meta.KeyUsage),
		"origin":       string(meta.Origin),
		"multiRegion":  meta.MultiRegion,
		"creationDate": aws.ToTime(meta.CreationDate),
		"enabled":      meta.Enabled,
	}
}

func summarizeECRRepository(repo ecrtypes.Repository) map[string]any {
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

func summarizeECRImageID(id ecrtypes.ImageIdentifier) map[string]any {
	return map[string]any{
		"imageDigest": aws.ToString(id.ImageDigest),
		"imageTag":    aws.ToString(id.ImageTag),
	}
}

func summarizeECRImageDetail(detail ecrtypes.ImageDetail) map[string]any {
	return map[string]any{
		"imageDigest":      aws.ToString(detail.ImageDigest),
		"imageTags":        detail.ImageTags,
		"imagePushedAt":    aws.ToTime(detail.ImagePushedAt),
		"imageSizeInBytes": detail.ImageSizeInBytes,
	}
}

func toECRImageIdentifiers(tags []string, digests []string) []ecrtypes.ImageIdentifier {
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

func tagMap(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		if key == "" {
			continue
		}
		out[key] = aws.ToString(tag.Value)
	}
	return out
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

func regionOrDefault(region string) string {
	if strings.TrimSpace(region) == "" {
		return "us-east-1"
	}
	return region
}
