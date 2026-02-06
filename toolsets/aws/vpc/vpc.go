package awsvpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53resolver/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx            mcp.ToolsetContext
	ec2Client      func(context.Context, string) (*ec2.Client, string, error)
	resolverClient func(context.Context, string) (*route53resolver.Client, string, error)
	toolsetID      string
}

func ToolSpecs(
	ctx mcp.ToolsetContext,
	toolsetID string,
	ec2Client func(context.Context, string) (*ec2.Client, string, error),
	resolverClient func(context.Context, string) (*route53resolver.Client, string, error),
) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, ec2Client: ec2Client, resolverClient: resolverClient, toolsetID: toolsetID}
	return []mcp.ToolSpec{
		{
			Name:        "aws.vpc.list_vpcs",
			Description: "List VPCs by id or return all VPCs.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListVPCs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListVPCs,
		},
		{
			Name:        "aws.vpc.get_vpc",
			Description: "Get a VPC by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetVPC(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetVPC,
		},
		{
			Name:        "aws.vpc.list_subnets",
			Description: "List subnets (optional VPC or subnet id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListSubnets(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListSubnets,
		},
		{
			Name:        "aws.vpc.get_subnet",
			Description: "Get a subnet by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetSubnet(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetSubnet,
		},
		{
			Name:        "aws.vpc.list_route_tables",
			Description: "List route tables (optional VPC or route table id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListRouteTables(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListRouteTables,
		},
		{
			Name:        "aws.vpc.get_route_table",
			Description: "Get a route table by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetRouteTable(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetRouteTable,
		},
		{
			Name:        "aws.vpc.list_nat_gateways",
			Description: "List NAT gateways (optional VPC, subnet, or gateway id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListNatGateways(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListNatGateways,
		},
		{
			Name:        "aws.vpc.get_nat_gateway",
			Description: "Get a NAT gateway by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetNatGateway(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetNatGateway,
		},
		{
			Name:        "aws.vpc.list_security_groups",
			Description: "List security groups (optional VPC or group id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListSecurityGroups(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListSecurityGroups,
		},
		{
			Name:        "aws.vpc.get_security_group",
			Description: "Get a security group by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetSecurityGroup(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetSecurityGroup,
		},
		{
			Name:        "aws.vpc.list_network_acls",
			Description: "List network ACLs (optional VPC or ACL id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListNetworkAcls(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListNetworkAcls,
		},
		{
			Name:        "aws.vpc.get_network_acl",
			Description: "Get a network ACL by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetNetworkAcl(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetNetworkAcl,
		},
		{
			Name:        "aws.vpc.list_internet_gateways",
			Description: "List internet gateways (optional VPC or gateway id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListInternetGateways(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListInternetGateways,
		},
		{
			Name:        "aws.vpc.get_internet_gateway",
			Description: "Get an internet gateway by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetInternetGateway(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetInternetGateway,
		},
		{
			Name:        "aws.vpc.list_vpc_endpoints",
			Description: "List VPC endpoints (optional VPC or endpoint id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListEndpoints(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListEndpoints,
		},
		{
			Name:        "aws.vpc.get_vpc_endpoint",
			Description: "Get a VPC endpoint by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetEndpoint(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetEndpoint,
		},
		{
			Name:        "aws.vpc.list_network_interfaces",
			Description: "List network interfaces (optional VPC, subnet, or interface id filters).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListNetworkInterfaces(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListNetworkInterfaces,
		},
		{
			Name:        "aws.vpc.get_network_interface",
			Description: "Get a network interface by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetNetworkInterface(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetNetworkInterface,
		},
		{
			Name:        "aws.vpc.list_resolver_endpoints",
			Description: "List Route53 Resolver endpoints (optional VPC filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListResolverEndpoints(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListResolverEndpoints,
		},
		{
			Name:        "aws.vpc.get_resolver_endpoint",
			Description: "Get a Route53 Resolver endpoint by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetResolverEndpoint(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetResolverEndpoint,
		},
		{
			Name:        "aws.vpc.list_resolver_rules",
			Description: "List Route53 Resolver rules (optional resolver endpoint filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCListResolverRules(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListResolverRules,
		},
		{
			Name:        "aws.vpc.get_resolver_rule",
			Description: "Get a Route53 Resolver rule by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaVPCGetResolverRule(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetResolverRule,
		},
	}
}

func (s *Service) handleListVPCs(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["vpcIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeVpcsInput{}
	if len(ids) > 0 {
		input.VpcIds = ids
	}
	var vpcs []map[string]any
	for {
		out, err := client.DescribeVpcs(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, vpc := range out.Vpcs {
			vpcs = append(vpcs, summarizeVPC(vpc))
			if limit > 0 && len(vpcs) >= limit {
				break
			}
		}
		if limit > 0 && len(vpcs) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region": regionOrDefault(usedRegion),
		"vpcs":   vpcs,
		"count":  len(vpcs),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetVPC(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	vpcID := toString(req.Arguments["vpcId"])
	if vpcID == "" {
		return errorResult(errors.New("vpcId is required")), errors.New("vpcId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{vpcID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Vpcs) == 0 {
		return errorResult(fmt.Errorf("vpc %s not found", vpcID)), fmt.Errorf("vpc %s not found", vpcID)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"vpc":    summarizeVPC(out.Vpcs[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/vpc/%s", vpcID)},
		},
	}, nil
}

func (s *Service) handleListSubnets(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["subnetIds"])
	tagFilters := tagFiltersFromArgs(req.Arguments["tagFilters"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeSubnetsInput{}
	if len(ids) > 0 {
		input.SubnetIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	if len(tagFilters) > 0 {
		input.Filters = append(input.Filters, tagFilters...)
	}
	var subnets []map[string]any
	for {
		out, err := client.DescribeSubnets(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, subnet := range out.Subnets {
			subnets = append(subnets, summarizeSubnet(subnet))
			if limit > 0 && len(subnets) >= limit {
				break
			}
		}
		if limit > 0 && len(subnets) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":  regionOrDefault(usedRegion),
		"subnets": subnets,
		"count":   len(subnets),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetSubnet(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	subnetID := toString(req.Arguments["subnetId"])
	if subnetID == "" {
		return errorResult(errors.New("subnetId is required")), errors.New("subnetId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: []string{subnetID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Subnets) == 0 {
		return errorResult(fmt.Errorf("subnet %s not found", subnetID)), fmt.Errorf("subnet %s not found", subnetID)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"subnet": summarizeSubnet(out.Subnets[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/subnet/%s", subnetID)},
		},
	}, nil
}

func (s *Service) handleListRouteTables(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["routeTableIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeRouteTablesInput{}
	if len(ids) > 0 {
		input.RouteTableIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	var tables []map[string]any
	for {
		out, err := client.DescribeRouteTables(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, table := range out.RouteTables {
			tables = append(tables, summarizeRouteTable(table))
			if limit > 0 && len(tables) >= limit {
				break
			}
		}
		if limit > 0 && len(tables) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"routeTables": tables,
		"count":       len(tables),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetRouteTable(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	tableID := toString(req.Arguments["routeTableId"])
	if tableID == "" {
		return errorResult(errors.New("routeTableId is required")), errors.New("routeTableId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{RouteTableIds: []string{tableID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.RouteTables) == 0 {
		return errorResult(fmt.Errorf("route table %s not found", tableID)), fmt.Errorf("route table %s not found", tableID)
	}
	result := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"routeTable": summarizeRouteTable(out.RouteTables[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/route-table/%s", tableID)},
		},
	}, nil
}

func (s *Service) handleListNatGateways(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	subnetID := toString(req.Arguments["subnetId"])
	ids := toStringSlice(req.Arguments["natGatewayIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeNatGatewaysInput{}
	if len(ids) > 0 {
		input.NatGatewayIds = ids
	}
	if vpcID != "" {
		input.Filter = append(input.Filter, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	if subnetID != "" {
		input.Filter = append(input.Filter, ec2types.Filter{
			Name:   aws.String("subnet-id"),
			Values: []string{subnetID},
		})
	}
	var gateways []map[string]any
	for {
		out, err := client.DescribeNatGateways(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, gw := range out.NatGateways {
			gateways = append(gateways, summarizeNatGateway(gw))
			if limit > 0 && len(gateways) >= limit {
				break
			}
		}
		if limit > 0 && len(gateways) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"natGateways": gateways,
		"count":       len(gateways),
		"filtersUsed": summarizeNatFilters(vpcID, subnetID, ids),
		"note":        "NAT gateway list is eventually consistent; recently created gateways may take time to appear.",
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetNatGateway(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	natID := toString(req.Arguments["natGatewayId"])
	if natID == "" {
		return errorResult(errors.New("natGatewayId is required")), errors.New("natGatewayId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{natID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.NatGateways) == 0 {
		return errorResult(fmt.Errorf("nat gateway %s not found", natID)), fmt.Errorf("nat gateway %s not found", natID)
	}
	result := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"natGateway": summarizeNatGateway(out.NatGateways[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/nat-gateway/%s", natID)},
		},
	}, nil
}

func (s *Service) handleListSecurityGroups(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["groupIds"])
	tagFilters := tagFiltersFromArgs(req.Arguments["tagFilters"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeSecurityGroupsInput{}
	if len(ids) > 0 {
		input.GroupIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	if len(tagFilters) > 0 {
		input.Filters = append(input.Filters, tagFilters...)
	}
	var groups []map[string]any
	for {
		out, err := client.DescribeSecurityGroups(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, sg := range out.SecurityGroups {
			groups = append(groups, summarizeSecurityGroup(sg))
			if limit > 0 && len(groups) >= limit {
				break
			}
		}
		if limit > 0 && len(groups) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"securityGroups": groups,
		"count":          len(groups),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetSecurityGroup(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	groupID := toString(req.Arguments["groupId"])
	if groupID == "" {
		return errorResult(errors.New("groupId is required")), errors.New("groupId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{groupID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.SecurityGroups) == 0 {
		return errorResult(fmt.Errorf("security group %s not found", groupID)), fmt.Errorf("security group %s not found", groupID)
	}
	result := map[string]any{
		"region":        regionOrDefault(usedRegion),
		"securityGroup": summarizeSecurityGroup(out.SecurityGroups[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/security-group/%s", groupID)},
		},
	}, nil
}

func (s *Service) handleListNetworkAcls(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["networkAclIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeNetworkAclsInput{}
	if len(ids) > 0 {
		input.NetworkAclIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	var acls []map[string]any
	for {
		out, err := client.DescribeNetworkAcls(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, acl := range out.NetworkAcls {
			acls = append(acls, summarizeNetworkAcl(acl))
			if limit > 0 && len(acls) >= limit {
				break
			}
		}
		if limit > 0 && len(acls) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"networkAcls": acls,
		"count":       len(acls),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetNetworkAcl(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	aclID := toString(req.Arguments["networkAclId"])
	if aclID == "" {
		return errorResult(errors.New("networkAclId is required")), errors.New("networkAclId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{NetworkAclIds: []string{aclID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.NetworkAcls) == 0 {
		return errorResult(fmt.Errorf("network ACL %s not found", aclID)), fmt.Errorf("network ACL %s not found", aclID)
	}
	result := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"networkAcl": summarizeNetworkAcl(out.NetworkAcls[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/network-acl/%s", aclID)},
		},
	}, nil
}

func (s *Service) handleListInternetGateways(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["internetGatewayIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeInternetGatewaysInput{}
	if len(ids) > 0 {
		input.InternetGatewayIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("attachment.vpc-id"),
			Values: []string{vpcID},
		})
	}
	var gateways []map[string]any
	for {
		out, err := client.DescribeInternetGateways(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, gw := range out.InternetGateways {
			gateways = append(gateways, summarizeInternetGateway(gw))
			if limit > 0 && len(gateways) >= limit {
				break
			}
		}
		if limit > 0 && len(gateways) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":           regionOrDefault(usedRegion),
		"internetGateways": gateways,
		"count":            len(gateways),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetInternetGateway(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	igwID := toString(req.Arguments["internetGatewayId"])
	if igwID == "" {
		return errorResult(errors.New("internetGatewayId is required")), errors.New("internetGatewayId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{InternetGatewayIds: []string{igwID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.InternetGateways) == 0 {
		return errorResult(fmt.Errorf("internet gateway %s not found", igwID)), fmt.Errorf("internet gateway %s not found", igwID)
	}
	result := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"internetGateway": summarizeInternetGateway(out.InternetGateways[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/internet-gateway/%s", igwID)},
		},
	}, nil
}

func (s *Service) handleListEndpoints(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	ids := toStringSlice(req.Arguments["endpointIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeVpcEndpointsInput{}
	if len(ids) > 0 {
		input.VpcEndpointIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	var endpoints []map[string]any
	for {
		out, err := client.DescribeVpcEndpoints(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, ep := range out.VpcEndpoints {
			endpoints = append(endpoints, summarizeVpcEndpoint(ep))
			if limit > 0 && len(endpoints) >= limit {
				break
			}
		}
		if limit > 0 && len(endpoints) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"endpoints": endpoints,
		"count":     len(endpoints),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetEndpoint(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	endpointID := toString(req.Arguments["endpointId"])
	if endpointID == "" {
		return errorResult(errors.New("endpointId is required")), errors.New("endpointId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{VpcEndpointIds: []string{endpointID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.VpcEndpoints) == 0 {
		return errorResult(fmt.Errorf("vpc endpoint %s not found", endpointID)), fmt.Errorf("vpc endpoint %s not found", endpointID)
	}
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"endpoint": summarizeVpcEndpoint(out.VpcEndpoints[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/vpc-endpoint/%s", endpointID)},
		},
	}, nil
}

func (s *Service) handleListNetworkInterfaces(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	subnetID := toString(req.Arguments["subnetId"])
	ids := toStringSlice(req.Arguments["networkInterfaceIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeNetworkInterfacesInput{}
	if len(ids) > 0 {
		input.NetworkInterfaceIds = ids
	}
	if vpcID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		})
	}
	if subnetID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("subnet-id"),
			Values: []string{subnetID},
		})
	}
	var interfaces []map[string]any
	for {
		out, err := client.DescribeNetworkInterfaces(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, iface := range out.NetworkInterfaces {
			interfaces = append(interfaces, summarizeNetworkInterface(iface))
			if limit > 0 && len(interfaces) >= limit {
				break
			}
		}
		if limit > 0 && len(interfaces) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":            regionOrDefault(usedRegion),
		"networkInterfaces": interfaces,
		"count":             len(interfaces),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetNetworkInterface(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	ifaceID := toString(req.Arguments["networkInterfaceId"])
	if ifaceID == "" {
		return errorResult(errors.New("networkInterfaceId is required")), errors.New("networkInterfaceId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: []string{ifaceID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.NetworkInterfaces) == 0 {
		return errorResult(fmt.Errorf("network interface %s not found", ifaceID)), fmt.Errorf("network interface %s not found", ifaceID)
	}
	result := map[string]any{
		"region":           regionOrDefault(usedRegion),
		"networkInterface": summarizeNetworkInterface(out.NetworkInterfaces[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("ec2/network-interface/%s", ifaceID)},
		},
	}, nil
}

func (s *Service) handleListResolverEndpoints(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if s.resolverClient == nil {
		return errorResult(errors.New("resolver client not available")), errors.New("resolver client not available")
	}
	region := toString(req.Arguments["region"])
	vpcID := toString(req.Arguments["vpcId"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.resolverClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &route53resolver.ListResolverEndpointsInput{}
	if vpcID != "" {
		input.Filters = append(input.Filters, r53types.Filter{Name: aws.String("VpcId"), Values: []string{vpcID}})
	}
	var endpoints []map[string]any
	for {
		out, err := client.ListResolverEndpoints(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, ep := range out.ResolverEndpoints {
			endpoints = append(endpoints, summarizeResolverEndpoint(ep))
			if limit > 0 && len(endpoints) >= limit {
				break
			}
		}
		if limit > 0 && len(endpoints) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":            regionOrDefault(usedRegion),
		"resolverEndpoints": endpoints,
		"count":             len(endpoints),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetResolverEndpoint(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if s.resolverClient == nil {
		return errorResult(errors.New("resolver client not available")), errors.New("resolver client not available")
	}
	endpointID := toString(req.Arguments["resolverEndpointId"])
	if endpointID == "" {
		return errorResult(errors.New("resolverEndpointId is required")), errors.New("resolverEndpointId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.resolverClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetResolverEndpoint(ctx, &route53resolver.GetResolverEndpointInput{ResolverEndpointId: aws.String(endpointID)})
	if err != nil {
		return errorResult(err), err
	}
	result := map[string]any{
		"region":           regionOrDefault(usedRegion),
		"resolverEndpoint": summarizeResolverEndpoint(*out.ResolverEndpoint),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("route53resolver/endpoint/%s", endpointID)},
		},
	}, nil
}

func (s *Service) handleListResolverRules(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if s.resolverClient == nil {
		return errorResult(errors.New("resolver client not available")), errors.New("resolver client not available")
	}
	region := toString(req.Arguments["region"])
	endpointID := toString(req.Arguments["resolverEndpointId"])
	ruleType := toString(req.Arguments["ruleType"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.resolverClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &route53resolver.ListResolverRulesInput{}
	if endpointID != "" {
		input.Filters = append(input.Filters, r53types.Filter{Name: aws.String("ResolverEndpointId"), Values: []string{endpointID}})
	}
	if ruleType != "" {
		input.Filters = append(input.Filters, r53types.Filter{Name: aws.String("RuleType"), Values: []string{ruleType}})
	}
	var rules []map[string]any
	for {
		out, err := client.ListResolverRules(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, rule := range out.ResolverRules {
			rules = append(rules, summarizeResolverRule(rule))
			if limit > 0 && len(rules) >= limit {
				break
			}
		}
		if limit > 0 && len(rules) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":        regionOrDefault(usedRegion),
		"resolverRules": rules,
		"count":         len(rules),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetResolverRule(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if s.resolverClient == nil {
		return errorResult(errors.New("resolver client not available")), errors.New("resolver client not available")
	}
	ruleID := toString(req.Arguments["resolverRuleId"])
	if ruleID == "" {
		return errorResult(errors.New("resolverRuleId is required")), errors.New("resolverRuleId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.resolverClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.GetResolverRule(ctx, &route53resolver.GetResolverRuleInput{ResolverRuleId: aws.String(ruleID)})
	if err != nil {
		return errorResult(err), err
	}
	result := map[string]any{
		"region":       regionOrDefault(usedRegion),
		"resolverRule": summarizeResolverRule(*out.ResolverRule),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("route53resolver/rule/%s", ruleID)},
		},
	}, nil
}

func summarizeVPC(vpc ec2types.Vpc) map[string]any {
	return map[string]any{
		"id":          aws.ToString(vpc.VpcId),
		"cidrBlock":   aws.ToString(vpc.CidrBlock),
		"state":       vpc.State,
		"isDefault":   vpc.IsDefault,
		"dhcpOptions": aws.ToString(vpc.DhcpOptionsId),
		"tenancy":     vpc.InstanceTenancy,
		"ipv6Cidrs":   summarizeIpv6Cidr(vpc.Ipv6CidrBlockAssociationSet),
		"tags":        tagMap(vpc.Tags),
		"ownerId":     aws.ToString(vpc.OwnerId),
		"cidrBlocks":  summarizeCidrBlocks(vpc.CidrBlockAssociationSet),
	}
}

func summarizeSubnet(subnet ec2types.Subnet) map[string]any {
	return map[string]any{
		"id":                  aws.ToString(subnet.SubnetId),
		"vpcId":               aws.ToString(subnet.VpcId),
		"cidrBlock":           aws.ToString(subnet.CidrBlock),
		"availabilityZone":    aws.ToString(subnet.AvailabilityZone),
		"state":               subnet.State,
		"mapPublicIpOnLaunch": subnet.MapPublicIpOnLaunch,
		"ipv6Cidrs":           summarizeSubnetIpv6Cidr(subnet.Ipv6CidrBlockAssociationSet),
		"tags":                tagMap(subnet.Tags),
	}
}

func summarizeRouteTable(table ec2types.RouteTable) map[string]any {
	routes := make([]map[string]any, 0, len(table.Routes))
	for _, route := range table.Routes {
		routes = append(routes, summarizeRoute(route))
	}
	associations := make([]map[string]any, 0, len(table.Associations))
	for _, assoc := range table.Associations {
		associations = append(associations, map[string]any{
			"id":          aws.ToString(assoc.RouteTableAssociationId),
			"subnetId":    aws.ToString(assoc.SubnetId),
			"gatewayId":   aws.ToString(assoc.GatewayId),
			"main":        assoc.Main,
			"association": assoc.AssociationState,
		})
	}
	return map[string]any{
		"id":           aws.ToString(table.RouteTableId),
		"vpcId":        aws.ToString(table.VpcId),
		"routes":       routes,
		"associations": associations,
		"tags":         tagMap(table.Tags),
	}
}

func summarizeRoute(route ec2types.Route) map[string]any {
	return map[string]any{
		"destinationCidr":     aws.ToString(route.DestinationCidrBlock),
		"destinationIpv6Cidr": aws.ToString(route.DestinationIpv6CidrBlock),
		"destinationPrefix":   aws.ToString(route.DestinationPrefixListId),
		"gatewayId":           aws.ToString(route.GatewayId),
		"natGatewayId":        aws.ToString(route.NatGatewayId),
		"transitGatewayId":    aws.ToString(route.TransitGatewayId),
		"instanceId":          aws.ToString(route.InstanceId),
		"vpcPeeringId":        aws.ToString(route.VpcPeeringConnectionId),
		"state":               route.State,
		"origin":              route.Origin,
	}
}

func summarizeNatGateway(gw ec2types.NatGateway) map[string]any {
	addresses := make([]map[string]any, 0, len(gw.NatGatewayAddresses))
	for _, addr := range gw.NatGatewayAddresses {
		addresses = append(addresses, map[string]any{
			"allocationId":     aws.ToString(addr.AllocationId),
			"publicIp":         aws.ToString(addr.PublicIp),
			"privateIp":        aws.ToString(addr.PrivateIp),
			"networkInterface": aws.ToString(addr.NetworkInterfaceId),
		})
	}
	return map[string]any{
		"id":             aws.ToString(gw.NatGatewayId),
		"vpcId":          aws.ToString(gw.VpcId),
		"subnetId":       aws.ToString(gw.SubnetId),
		"state":          gw.State,
		"connectivity":   gw.ConnectivityType,
		"createTime":     gw.CreateTime,
		"addresses":      addresses,
		"tags":           tagMap(gw.Tags),
		"failureCode":    aws.ToString(gw.FailureCode),
		"failureMessage": aws.ToString(gw.FailureMessage),
		"provisioned":    gw.ProvisionedBandwidth,
		"deleteTime":     gw.DeleteTime,
	}
}

func summarizeSecurityGroup(sg ec2types.SecurityGroup) map[string]any {
	return map[string]any{
		"id":          aws.ToString(sg.GroupId),
		"name":        aws.ToString(sg.GroupName),
		"description": aws.ToString(sg.Description),
		"vpcId":       aws.ToString(sg.VpcId),
		"ownerId":     aws.ToString(sg.OwnerId),
		"inbound":     summarizePermissions(sg.IpPermissions),
		"outbound":    summarizePermissions(sg.IpPermissionsEgress),
		"tags":        tagMap(sg.Tags),
	}
}

func summarizePermissions(perms []ec2types.IpPermission) []map[string]any {
	var out []map[string]any
	for _, perm := range perms {
		entry := map[string]any{
			"protocol": perm.IpProtocol,
			"fromPort": perm.FromPort,
			"toPort":   perm.ToPort,
		}
		if len(perm.IpRanges) > 0 {
			var cidrs []string
			for _, cidr := range perm.IpRanges {
				if cidr.CidrIp != nil {
					cidrs = append(cidrs, aws.ToString(cidr.CidrIp))
				}
			}
			entry["ipv4Ranges"] = cidrs
		}
		if len(perm.Ipv6Ranges) > 0 {
			var cidrs []string
			for _, cidr := range perm.Ipv6Ranges {
				if cidr.CidrIpv6 != nil {
					cidrs = append(cidrs, aws.ToString(cidr.CidrIpv6))
				}
			}
			entry["ipv6Ranges"] = cidrs
		}
		if len(perm.UserIdGroupPairs) > 0 {
			var refs []map[string]any
			for _, pair := range perm.UserIdGroupPairs {
				refs = append(refs, map[string]any{
					"groupId":   aws.ToString(pair.GroupId),
					"groupName": aws.ToString(pair.GroupName),
					"userId":    aws.ToString(pair.UserId),
					"vpcId":     aws.ToString(pair.VpcId),
				})
			}
			entry["securityGroups"] = refs
		}
		if len(perm.PrefixListIds) > 0 {
			var prefixes []string
			for _, prefix := range perm.PrefixListIds {
				prefixes = append(prefixes, aws.ToString(prefix.PrefixListId))
			}
			entry["prefixListIds"] = prefixes
		}
		out = append(out, entry)
	}
	return out
}

func summarizeNetworkAcl(acl ec2types.NetworkAcl) map[string]any {
	entries := make([]map[string]any, 0, len(acl.Entries))
	for _, entry := range acl.Entries {
		item := map[string]any{
			"egress":     entry.Egress,
			"ruleNumber": entry.RuleNumber,
			"protocol":   entry.Protocol,
			"ruleAction": entry.RuleAction,
			"cidrBlock":  aws.ToString(entry.CidrBlock),
			"ipv6Cidr":   aws.ToString(entry.Ipv6CidrBlock),
			"icmp":       entry.IcmpTypeCode,
			"portRange":  entry.PortRange,
		}
		entries = append(entries, item)
	}
	associations := make([]map[string]any, 0, len(acl.Associations))
	for _, assoc := range acl.Associations {
		associations = append(associations, map[string]any{
			"id":       aws.ToString(assoc.NetworkAclAssociationId),
			"subnetId": aws.ToString(assoc.SubnetId),
		})
	}
	return map[string]any{
		"id":           aws.ToString(acl.NetworkAclId),
		"vpcId":        aws.ToString(acl.VpcId),
		"isDefault":    acl.IsDefault,
		"entries":      entries,
		"associations": associations,
		"tags":         tagMap(acl.Tags),
	}
}

func summarizeInternetGateway(gw ec2types.InternetGateway) map[string]any {
	attachments := make([]map[string]any, 0, len(gw.Attachments))
	for _, attachment := range gw.Attachments {
		attachments = append(attachments, map[string]any{
			"vpcId": aws.ToString(attachment.VpcId),
			"state": attachment.State,
		})
	}
	return map[string]any{
		"id":          aws.ToString(gw.InternetGatewayId),
		"attachments": attachments,
		"tags":        tagMap(gw.Tags),
	}
}

func summarizeVpcEndpoint(ep ec2types.VpcEndpoint) map[string]any {
	var sgIDs []string
	for _, group := range ep.Groups {
		sgIDs = append(sgIDs, aws.ToString(group.GroupId))
	}
	return map[string]any{
		"id":                  aws.ToString(ep.VpcEndpointId),
		"vpcId":               aws.ToString(ep.VpcId),
		"serviceName":         aws.ToString(ep.ServiceName),
		"type":                ep.VpcEndpointType,
		"state":               ep.State,
		"subnetIds":           ep.SubnetIds,
		"routeTableIds":       ep.RouteTableIds,
		"securityGroupIds":    sgIDs,
		"privateDnsEnabled":   ep.PrivateDnsEnabled,
		"networkInterfaceIds": ep.NetworkInterfaceIds,
		"tags":                tagMap(ep.Tags),
	}
}

func summarizeNetworkInterface(iface ec2types.NetworkInterface) map[string]any {
	var sgIDs []string
	for _, group := range iface.Groups {
		sgIDs = append(sgIDs, aws.ToString(group.GroupId))
	}
	var privateIPs []string
	for _, addr := range iface.PrivateIpAddresses {
		privateIPs = append(privateIPs, aws.ToString(addr.PrivateIpAddress))
	}
	attachment := map[string]any{}
	if iface.Attachment != nil {
		attachment["id"] = aws.ToString(iface.Attachment.AttachmentId)
		attachment["instanceId"] = aws.ToString(iface.Attachment.InstanceId)
		attachment["deviceIndex"] = iface.Attachment.DeviceIndex
		attachment["status"] = iface.Attachment.Status
	}
	return map[string]any{
		"id":               aws.ToString(iface.NetworkInterfaceId),
		"status":           iface.Status,
		"description":      aws.ToString(iface.Description),
		"type":             iface.InterfaceType,
		"vpcId":            aws.ToString(iface.VpcId),
		"subnetId":         aws.ToString(iface.SubnetId),
		"privateIp":        aws.ToString(iface.PrivateIpAddress),
		"privateIps":       privateIPs,
		"privateDnsName":   aws.ToString(iface.PrivateDnsName),
		"ownerId":          aws.ToString(iface.OwnerId),
		"securityGroupIds": sgIDs,
		"attachment":       attachment,
		"tags":             tagMap(iface.TagSet),
	}
}

func summarizeResolverEndpoint(endpoint r53types.ResolverEndpoint) map[string]any {
	return map[string]any{
		"id":               aws.ToString(endpoint.Id),
		"name":             aws.ToString(endpoint.Name),
		"direction":        endpoint.Direction,
		"status":           endpoint.Status,
		"vpcId":            aws.ToString(endpoint.HostVPCId),
		"securityGroups":   endpoint.SecurityGroupIds,
		"ipAddressCount":   endpoint.IpAddressCount,
		"creationTime":     aws.ToString(endpoint.CreationTime),
		"modificationTime": aws.ToString(endpoint.ModificationTime),
	}
}

func summarizeResolverRule(rule r53types.ResolverRule) map[string]any {
	var targets []map[string]any
	for _, target := range rule.TargetIps {
		targets = append(targets, map[string]any{
			"ip":   aws.ToString(target.Ip),
			"port": target.Port,
		})
	}
	return map[string]any{
		"id":                 aws.ToString(rule.Id),
		"name":               aws.ToString(rule.Name),
		"domainName":         aws.ToString(rule.DomainName),
		"ruleType":           rule.RuleType,
		"status":             rule.Status,
		"resolverEndpointId": aws.ToString(rule.ResolverEndpointId),
		"ownerId":            aws.ToString(rule.OwnerId),
		"targetIps":          targets,
	}
}

func summarizeCidrBlocks(blocks []ec2types.VpcCidrBlockAssociation) []map[string]any {
	var out []map[string]any
	for _, block := range blocks {
		out = append(out, map[string]any{
			"cidrBlock": aws.ToString(block.CidrBlock),
			"state":     block.CidrBlockState,
		})
	}
	return out
}

func summarizeIpv6Cidr(blocks []ec2types.VpcIpv6CidrBlockAssociation) []map[string]any {
	var out []map[string]any
	for _, block := range blocks {
		out = append(out, map[string]any{
			"ipv6CidrBlock": aws.ToString(block.Ipv6CidrBlock),
			"state":         block.Ipv6CidrBlockState,
		})
	}
	return out
}

func summarizeSubnetIpv6Cidr(blocks []ec2types.SubnetIpv6CidrBlockAssociation) []map[string]any {
	var out []map[string]any
	for _, block := range blocks {
		out = append(out, map[string]any{
			"ipv6CidrBlock": aws.ToString(block.Ipv6CidrBlock),
			"state":         block.Ipv6CidrBlockState,
		})
	}
	return out
}

func summarizeNatFilters(vpcID, subnetID string, ids []string) map[string]any {
	out := map[string]any{}
	if vpcID != "" {
		out["vpcId"] = vpcID
	}
	if subnetID != "" {
		out["subnetId"] = subnetID
	}
	if len(ids) > 0 {
		out["natGatewayIds"] = ids
	}
	return out
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

func tagFiltersFromArgs(value any) []ec2types.Filter {
	tagMap, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	filters := make([]ec2types.Filter, 0, len(tagMap))
	for key, raw := range tagMap {
		if strings.TrimSpace(key) == "" {
			continue
		}
		values := toStringSlice(raw)
		if len(values) == 0 {
			continue
		}
		filters = append(filters, ec2types.Filter{
			Name:   aws.String("tag:" + key),
			Values: values,
		})
	}
	return filters
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
