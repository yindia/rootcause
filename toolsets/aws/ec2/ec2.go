package awsec2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autotypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolsetContext
	ec2Client func(context.Context, string) (*ec2.Client, string, error)
	asgClient func(context.Context, string) (*autoscaling.Client, string, error)
	elbClient func(context.Context, string) (*elasticloadbalancingv2.Client, string, error)
	iamClient func(context.Context, string) (*iam.Client, string, error)
	toolsetID string
}

func ToolSpecs(
	ctx mcp.ToolsetContext,
	toolsetID string,
	ec2Client func(context.Context, string) (*ec2.Client, string, error),
	asgClient func(context.Context, string) (*autoscaling.Client, string, error),
	elbClient func(context.Context, string) (*elasticloadbalancingv2.Client, string, error),
	iamClient func(context.Context, string) (*iam.Client, string, error),
) []mcp.ToolSpec {
	svc := &Service{
		ctx:       ctx,
		ec2Client: ec2Client,
		asgClient: asgClient,
		elbClient: elbClient,
		iamClient: iamClient,
		toolsetID: toolsetID,
	}
	return []mcp.ToolSpec{
		{
			Name:        "aws.ec2.list_instances",
			Description: "List EC2 instances (optional filters by ids, VPC, subnet, state).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListInstances(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListInstances,
		},
		{
			Name:        "aws.ec2.get_instance",
			Description: "Get an EC2 instance by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetInstance(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetInstance,
		},
		{
			Name:        "aws.ec2.list_auto_scaling_groups",
			Description: "List Auto Scaling Groups (optional name filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListASGs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListASGs,
		},
		{
			Name:        "aws.ec2.get_auto_scaling_group",
			Description: "Get an Auto Scaling Group by name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetASG(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetASG,
		},
		{
			Name:        "aws.ec2.list_load_balancers",
			Description: "List ALB/NLB load balancers (optional name/ARN filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListLoadBalancers(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListLoadBalancers,
		},
		{
			Name:        "aws.ec2.get_load_balancer",
			Description: "Get an ALB/NLB load balancer by ARN or name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetLoadBalancer(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetLoadBalancer,
		},
		{
			Name:        "aws.ec2.list_target_groups",
			Description: "List ALB/NLB target groups (optional name/ARN/LB filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListTargetGroups(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListTargetGroups,
		},
		{
			Name:        "aws.ec2.get_target_group",
			Description: "Get a target group by ARN or name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetTargetGroup(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetTargetGroup,
		},
		{
			Name:        "aws.ec2.list_listeners",
			Description: "List ALB/NLB listeners (optional LB or listener ARN filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListListeners(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListListeners,
		},
		{
			Name:        "aws.ec2.get_listener",
			Description: "Get an ALB/NLB listener by ARN.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetListener(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetListener,
		},
		{
			Name:        "aws.ec2.get_target_health",
			Description: "Get target health for an ALB/NLB target group.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetTargetHealth(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetTargetHealth,
		},
		{
			Name:        "aws.ec2.list_listener_rules",
			Description: "List listener rules (by listener ARN or rule ARNs).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListListenerRules(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListListenerRules,
		},
		{
			Name:        "aws.ec2.get_listener_rule",
			Description: "Get a listener rule by ARN.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetListenerRule(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetListenerRule,
		},
		{
			Name:        "aws.ec2.list_auto_scaling_policies",
			Description: "List auto scaling policies (optional ASG/policy filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListAutoScalingPolicies(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListAutoScalingPolicies,
		},
		{
			Name:        "aws.ec2.get_auto_scaling_policy",
			Description: "Get an auto scaling policy by name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetAutoScalingPolicy(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetAutoScalingPolicy,
		},
		{
			Name:        "aws.ec2.list_scaling_activities",
			Description: "List auto scaling activities (optional ASG or activity filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListScalingActivities(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListScalingActivities,
		},
		{
			Name:        "aws.ec2.get_scaling_activity",
			Description: "Get an auto scaling activity by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetScalingActivity(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetScalingActivity,
		},
		{
			Name:        "aws.ec2.list_launch_templates",
			Description: "List EC2 launch templates (optional id/name filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListLaunchTemplates(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListLaunchTemplates,
		},
		{
			Name:        "aws.ec2.get_launch_template",
			Description: "Get an EC2 launch template by id or name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetLaunchTemplate(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetLaunchTemplate,
		},
		{
			Name:        "aws.ec2.list_launch_configurations",
			Description: "List ASG launch configurations (optional name filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListLaunchConfigurations(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListLaunchConfigurations,
		},
		{
			Name:        "aws.ec2.get_launch_configuration",
			Description: "Get an ASG launch configuration by name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetLaunchConfiguration(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetLaunchConfiguration,
		},
		{
			Name:        "aws.ec2.get_instance_iam",
			Description: "Get instance profile and IAM roles attached to an EC2 instance.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetInstanceIAM(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetInstanceIAM,
		},
		{
			Name:        "aws.ec2.get_security_group_rules",
			Description: "Get security group rules by security group id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetSecurityGroupRules(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetSecurityGroupRules,
		},
		{
			Name:        "aws.ec2.list_spot_instance_requests",
			Description: "List spot instance requests (optional id/state filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListSpotInstanceRequests(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListSpotInstanceRequests,
		},
		{
			Name:        "aws.ec2.get_spot_instance_request",
			Description: "Get a spot instance request by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetSpotInstanceRequest(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetSpotInstanceRequest,
		},
		{
			Name:        "aws.ec2.list_capacity_reservations",
			Description: "List EC2 capacity reservations (optional id filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListCapacityReservations(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListCapacityReservations,
		},
		{
			Name:        "aws.ec2.get_capacity_reservation",
			Description: "Get an EC2 capacity reservation by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetCapacityReservation(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetCapacityReservation,
		},
		{
			Name:        "aws.ec2.list_volumes",
			Description: "List EBS volumes (optional id/instance filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListVolumes(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListVolumes,
		},
		{
			Name:        "aws.ec2.get_volume",
			Description: "Get an EBS volume by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetVolume(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetVolume,
		},
		{
			Name:        "aws.ec2.list_snapshots",
			Description: "List EBS snapshots (optional id/owner/volume filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListSnapshots(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListSnapshots,
		},
		{
			Name:        "aws.ec2.get_snapshot",
			Description: "Get an EBS snapshot by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetSnapshot(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetSnapshot,
		},
		{
			Name:        "aws.ec2.list_volume_attachments",
			Description: "List volume attachments (optional volume/instance filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListVolumeAttachments(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListVolumeAttachments,
		},
		{
			Name:        "aws.ec2.list_placement_groups",
			Description: "List placement groups (optional name filter).",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListPlacementGroups(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListPlacementGroups,
		},
		{
			Name:        "aws.ec2.get_placement_group",
			Description: "Get a placement group by name.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetPlacementGroup(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetPlacementGroup,
		},
		{
			Name:        "aws.ec2.list_instance_status",
			Description: "List EC2 instance status and scheduled events.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2ListInstanceStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListInstanceStatus,
		},
		{
			Name:        "aws.ec2.get_instance_status",
			Description: "Get EC2 instance status and scheduled events by id.",
			ToolsetID:   toolsetID,
			InputSchema: schemaEC2GetInstanceStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleGetInstanceStatus,
		},
	}
}

func (s *Service) handleListInstances(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["instanceIds"])
	vpcID := toString(req.Arguments["vpcId"])
	subnetID := toString(req.Arguments["subnetId"])
	state := strings.TrimSpace(toString(req.Arguments["state"]))
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeInstancesInput{}
	if len(ids) > 0 {
		input.InstanceIds = ids
	}
	var filters []ec2types.Filter
	if vpcID != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("vpc-id"), Values: []string{vpcID}})
	}
	if subnetID != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("subnet-id"), Values: []string{subnetID}})
	}
	if state != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("instance-state-name"), Values: []string{state}})
	}
	if len(filters) > 0 {
		input.Filters = filters
	}
	var instances []map[string]any
	for {
		out, err := client.DescribeInstances(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, reservation := range out.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, summarizeInstance(inst))
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
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"instances": instances,
		"count":     len(instances),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetInstance(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	instanceID := toString(req.Arguments["instanceId"])
	if instanceID == "" {
		return errorResult(errors.New("instanceId is required")), errors.New("instanceId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		return errorResult(err), err
	}
	for _, reservation := range out.Reservations {
		for _, inst := range reservation.Instances {
			if aws.ToString(inst.InstanceId) == instanceID {
				result := map[string]any{
					"region":   regionOrDefault(usedRegion),
					"instance": summarizeInstance(inst),
				}
				return mcp.ToolResult{
					Data: s.ctx.Redactor.RedactValue(result),
					Metadata: mcp.ToolMetadata{
						Resources: []string{fmt.Sprintf("ec2/instance/%s", instanceID)},
					},
				}, nil
			}
		}
	}
	return errorResult(fmt.Errorf("instance %s not found", instanceID)), fmt.Errorf("instance %s not found", instanceID)
}

func (s *Service) handleListASGs(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	names := toStringSlice(req.Arguments["autoScalingGroupNames"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	if len(names) > 0 {
		input.AutoScalingGroupNames = names
	}
	var groups []map[string]any
	for {
		out, err := client.DescribeAutoScalingGroups(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, group := range out.AutoScalingGroups {
			groups = append(groups, summarizeASG(group))
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
		"region":            regionOrDefault(usedRegion),
		"autoScalingGroups": groups,
		"count":             len(groups),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetASG(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	name := toString(req.Arguments["autoScalingGroupName"])
	if name == "" {
		return errorResult(errors.New("autoScalingGroupName is required")), errors.New("autoScalingGroupName is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []string{name}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.AutoScalingGroups) == 0 {
		return errorResult(fmt.Errorf("auto scaling group %s not found", name)), fmt.Errorf("auto scaling group %s not found", name)
	}
	result := map[string]any{
		"region":           regionOrDefault(usedRegion),
		"autoScalingGroup": summarizeASG(out.AutoScalingGroups[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("autoscaling/group/%s", name)},
		},
	}, nil
}

func (s *Service) handleListLoadBalancers(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	arns := toStringSlice(req.Arguments["loadBalancerArns"])
	names := toStringSlice(req.Arguments["names"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}
	if len(arns) > 0 {
		input.LoadBalancerArns = arns
	}
	if len(names) > 0 {
		input.Names = names
	}
	var lbs []map[string]any
	for {
		out, err := client.DescribeLoadBalancers(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, lb := range out.LoadBalancers {
			lbs = append(lbs, summarizeLoadBalancer(lb))
			if limit > 0 && len(lbs) >= limit {
				break
			}
		}
		if limit > 0 && len(lbs) >= limit {
			break
		}
		if out.NextMarker == nil || aws.ToString(out.NextMarker) == "" {
			break
		}
		input.Marker = out.NextMarker
	}
	data := map[string]any{
		"region":        regionOrDefault(usedRegion),
		"loadBalancers": lbs,
		"count":         len(lbs),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetLoadBalancer(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	arn := strings.TrimSpace(toString(req.Arguments["loadBalancerArn"]))
	name := strings.TrimSpace(toString(req.Arguments["name"]))
	if arn == "" && name == "" {
		return errorResult(errors.New("loadBalancerArn or name is required")), errors.New("loadBalancerArn or name is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}
	if arn != "" {
		input.LoadBalancerArns = []string{arn}
	} else {
		input.Names = []string{name}
	}
	out, err := client.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if len(out.LoadBalancers) == 0 {
		key := name
		if arn != "" {
			key = arn
		}
		return errorResult(fmt.Errorf("load balancer %s not found", key)), fmt.Errorf("load balancer %s not found", key)
	}
	result := map[string]any{
		"region":       regionOrDefault(usedRegion),
		"loadBalancer": summarizeLoadBalancer(out.LoadBalancers[0]),
	}
	resourceID := name
	if arn != "" {
		resourceID = arn
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("elbv2/load-balancer/%s", resourceID)},
		},
	}, nil
}

func (s *Service) handleListTargetGroups(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	arns := toStringSlice(req.Arguments["targetGroupArns"])
	names := toStringSlice(req.Arguments["names"])
	lbArn := strings.TrimSpace(toString(req.Arguments["loadBalancerArn"]))
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeTargetGroupsInput{}
	if len(arns) > 0 {
		input.TargetGroupArns = arns
	}
	if len(names) > 0 {
		input.Names = names
	}
	if lbArn != "" {
		input.LoadBalancerArn = aws.String(lbArn)
	}
	var groups []map[string]any
	for {
		out, err := client.DescribeTargetGroups(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, group := range out.TargetGroups {
			groups = append(groups, summarizeTargetGroup(group))
			if limit > 0 && len(groups) >= limit {
				break
			}
		}
		if limit > 0 && len(groups) >= limit {
			break
		}
		if out.NextMarker == nil || aws.ToString(out.NextMarker) == "" {
			break
		}
		input.Marker = out.NextMarker
	}
	data := map[string]any{
		"region":       regionOrDefault(usedRegion),
		"targetGroups": groups,
		"count":        len(groups),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetTargetGroup(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	arn := strings.TrimSpace(toString(req.Arguments["targetGroupArn"]))
	name := strings.TrimSpace(toString(req.Arguments["name"]))
	if arn == "" && name == "" {
		return errorResult(errors.New("targetGroupArn or name is required")), errors.New("targetGroupArn or name is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeTargetGroupsInput{}
	if arn != "" {
		input.TargetGroupArns = []string{arn}
	} else {
		input.Names = []string{name}
	}
	out, err := client.DescribeTargetGroups(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if len(out.TargetGroups) == 0 {
		key := name
		if arn != "" {
			key = arn
		}
		return errorResult(fmt.Errorf("target group %s not found", key)), fmt.Errorf("target group %s not found", key)
	}
	result := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"targetGroup": summarizeTargetGroup(out.TargetGroups[0]),
	}
	resourceID := name
	if arn != "" {
		resourceID = arn
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("elbv2/target-group/%s", resourceID)},
		},
	}, nil
}

func (s *Service) handleListListeners(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	arns := toStringSlice(req.Arguments["listenerArns"])
	lbArn := strings.TrimSpace(toString(req.Arguments["loadBalancerArn"]))
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeListenersInput{}
	if len(arns) > 0 {
		input.ListenerArns = arns
	}
	if lbArn != "" {
		input.LoadBalancerArn = aws.String(lbArn)
	}
	var listeners []map[string]any
	for {
		out, err := client.DescribeListeners(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, listener := range out.Listeners {
			listeners = append(listeners, summarizeListener(listener))
			if limit > 0 && len(listeners) >= limit {
				break
			}
		}
		if limit > 0 && len(listeners) >= limit {
			break
		}
		if out.NextMarker == nil || aws.ToString(out.NextMarker) == "" {
			break
		}
		input.Marker = out.NextMarker
	}
	data := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"listeners": listeners,
		"count":     len(listeners),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetListener(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	arn := strings.TrimSpace(toString(req.Arguments["listenerArn"]))
	if arn == "" {
		return errorResult(errors.New("listenerArn is required")), errors.New("listenerArn is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{ListenerArns: []string{arn}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Listeners) == 0 {
		return errorResult(fmt.Errorf("listener %s not found", arn)), fmt.Errorf("listener %s not found", arn)
	}
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"listener": summarizeListener(out.Listeners[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("elbv2/listener/%s", arn)},
		},
	}, nil
}

func (s *Service) handleGetTargetHealth(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	groupArn := strings.TrimSpace(toString(req.Arguments["targetGroupArn"]))
	if groupArn == "" {
		return errorResult(errors.New("targetGroupArn is required")), errors.New("targetGroupArn is required")
	}
	targetIDs := toStringSlice(req.Arguments["targetIds"])
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(groupArn),
	}
	if len(targetIDs) > 0 {
		var targets []elbtypes.TargetDescription
		for _, id := range targetIDs {
			targets = append(targets, elbtypes.TargetDescription{Id: aws.String(id)})
		}
		input.Targets = targets
	}
	out, err := client.DescribeTargetHealth(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	var health []map[string]any
	for _, desc := range out.TargetHealthDescriptions {
		reason := ""
		description := ""
		if desc.TargetHealth != nil {
			reason = string(desc.TargetHealth.Reason)
			description = aws.ToString(desc.TargetHealth.Description)
		}
		health = append(health, map[string]any{
			"target":       desc.Target,
			"health":       desc.TargetHealth,
			"healthReason": reason,
			"description":  description,
		})
	}
	result := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"targetGroupArn": groupArn,
		"targetHealth":   health,
		"count":          len(health),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListListenerRules(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	arns := toStringSlice(req.Arguments["ruleArns"])
	listenerArn := strings.TrimSpace(toString(req.Arguments["listenerArn"]))
	limit := toInt(req.Arguments["limit"], 100)
	if len(arns) == 0 && listenerArn == "" {
		return errorResult(errors.New("listenerArn or ruleArns is required")), errors.New("listenerArn or ruleArns is required")
	}
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &elasticloadbalancingv2.DescribeRulesInput{}
	if len(arns) > 0 {
		input.RuleArns = arns
	} else if listenerArn != "" {
		input.ListenerArn = aws.String(listenerArn)
	}
	var rules []map[string]any
	for {
		out, err := client.DescribeRules(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, rule := range out.Rules {
			rules = append(rules, summarizeListenerRule(rule))
			if limit > 0 && len(rules) >= limit {
				break
			}
		}
		if limit > 0 && len(rules) >= limit {
			break
		}
		if out.NextMarker == nil || aws.ToString(out.NextMarker) == "" {
			break
		}
		input.Marker = out.NextMarker
	}
	data := map[string]any{
		"region": regionOrDefault(usedRegion),
		"rules":  rules,
		"count":  len(rules),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetListenerRule(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	ruleArn := strings.TrimSpace(toString(req.Arguments["ruleArn"]))
	if ruleArn == "" {
		return errorResult(errors.New("ruleArn is required")), errors.New("ruleArn is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.elbClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{RuleArns: []string{ruleArn}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Rules) == 0 {
		return errorResult(fmt.Errorf("listener rule %s not found", ruleArn)), fmt.Errorf("listener rule %s not found", ruleArn)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"rule":   summarizeListenerRule(out.Rules[0]),
	}
	return mcp.ToolResult{
		Data: s.ctx.Redactor.RedactValue(result),
		Metadata: mcp.ToolMetadata{
			Resources: []string{fmt.Sprintf("elbv2/rule/%s", ruleArn)},
		},
	}, nil
}

func (s *Service) handleListAutoScalingPolicies(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	group := toString(req.Arguments["autoScalingGroupName"])
	policyNames := toStringSlice(req.Arguments["policyNames"])
	policyTypes := toStringSlice(req.Arguments["policyTypes"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribePoliciesInput{}
	if group != "" {
		input.AutoScalingGroupName = aws.String(group)
	}
	if len(policyNames) > 0 {
		input.PolicyNames = policyNames
	}
	if len(policyTypes) > 0 {
		input.PolicyTypes = policyTypes
	}
	var policies []map[string]any
	for {
		out, err := client.DescribePolicies(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, policy := range out.ScalingPolicies {
			policies = append(policies, summarizeScalingPolicy(policy))
			if limit > 0 && len(policies) >= limit {
				break
			}
		}
		if limit > 0 && len(policies) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"scalingPolicies": policies,
		"count":           len(policies),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetAutoScalingPolicy(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	policyName := toString(req.Arguments["policyName"])
	group := toString(req.Arguments["autoScalingGroupName"])
	if policyName == "" && group == "" {
		return errorResult(errors.New("policyName or autoScalingGroupName is required")), errors.New("policyName or autoScalingGroupName is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribePoliciesInput{}
	if policyName != "" {
		input.PolicyNames = []string{policyName}
	}
	if group != "" {
		input.AutoScalingGroupName = aws.String(group)
	}
	out, err := client.DescribePolicies(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if len(out.ScalingPolicies) == 0 {
		key := policyName
		if key == "" {
			key = group
		}
		return errorResult(fmt.Errorf("scaling policy %s not found", key)), fmt.Errorf("scaling policy %s not found", key)
	}
	result := map[string]any{
		"region":        regionOrDefault(usedRegion),
		"scalingPolicy": summarizeScalingPolicy(out.ScalingPolicies[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListScalingActivities(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	group := toString(req.Arguments["autoScalingGroupName"])
	ids := toStringSlice(req.Arguments["activityIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribeScalingActivitiesInput{}
	if group != "" {
		input.AutoScalingGroupName = aws.String(group)
	}
	if len(ids) > 0 {
		input.ActivityIds = ids
	}
	var activities []map[string]any
	for {
		out, err := client.DescribeScalingActivities(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, activity := range out.Activities {
			activities = append(activities, summarizeScalingActivity(activity))
			if limit > 0 && len(activities) >= limit {
				break
			}
		}
		if limit > 0 && len(activities) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"activities": activities,
		"count":      len(activities),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetScalingActivity(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	activityID := toString(req.Arguments["activityId"])
	if activityID == "" {
		return errorResult(errors.New("activityId is required")), errors.New("activityId is required")
	}
	region := toString(req.Arguments["region"])
	group := toString(req.Arguments["autoScalingGroupName"])
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribeScalingActivitiesInput{ActivityIds: []string{activityID}}
	if group != "" {
		input.AutoScalingGroupName = aws.String(group)
	}
	out, err := client.DescribeScalingActivities(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Activities) == 0 {
		return errorResult(fmt.Errorf("scaling activity %s not found", activityID)), fmt.Errorf("scaling activity %s not found", activityID)
	}
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"activity": summarizeScalingActivity(out.Activities[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListLaunchTemplates(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["launchTemplateIds"])
	names := toStringSlice(req.Arguments["names"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeLaunchTemplatesInput{}
	if len(ids) > 0 {
		input.LaunchTemplateIds = ids
	}
	if len(names) > 0 {
		input.LaunchTemplateNames = names
	}
	var templates []map[string]any
	for {
		out, err := client.DescribeLaunchTemplates(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, tmpl := range out.LaunchTemplates {
			templates = append(templates, summarizeLaunchTemplate(tmpl))
			if limit > 0 && len(templates) >= limit {
				break
			}
		}
		if limit > 0 && len(templates) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"launchTemplates": templates,
		"count":           len(templates),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetLaunchTemplate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	id := strings.TrimSpace(toString(req.Arguments["launchTemplateId"]))
	name := strings.TrimSpace(toString(req.Arguments["name"]))
	if id == "" && name == "" {
		return errorResult(errors.New("launchTemplateId or name is required")), errors.New("launchTemplateId or name is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeLaunchTemplatesInput{}
	if id != "" {
		input.LaunchTemplateIds = []string{id}
	} else {
		input.LaunchTemplateNames = []string{name}
	}
	out, err := client.DescribeLaunchTemplates(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	if len(out.LaunchTemplates) == 0 {
		key := name
		if id != "" {
			key = id
		}
		return errorResult(fmt.Errorf("launch template %s not found", key)), fmt.Errorf("launch template %s not found", key)
	}
	result := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"launchTemplate": summarizeLaunchTemplate(out.LaunchTemplates[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListLaunchConfigurations(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	names := toStringSlice(req.Arguments["launchConfigurationNames"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &autoscaling.DescribeLaunchConfigurationsInput{}
	if len(names) > 0 {
		input.LaunchConfigurationNames = names
	}
	var configs []map[string]any
	for {
		out, err := client.DescribeLaunchConfigurations(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, cfg := range out.LaunchConfigurations {
			configs = append(configs, summarizeLaunchConfiguration(cfg))
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
		"region":               regionOrDefault(usedRegion),
		"launchConfigurations": configs,
		"count":                len(configs),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetLaunchConfiguration(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	name := toString(req.Arguments["launchConfigurationName"])
	if name == "" {
		return errorResult(errors.New("launchConfigurationName is required")), errors.New("launchConfigurationName is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.asgClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []string{name},
	})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.LaunchConfigurations) == 0 {
		return errorResult(fmt.Errorf("launch configuration %s not found", name)), fmt.Errorf("launch configuration %s not found", name)
	}
	result := map[string]any{
		"region":              regionOrDefault(usedRegion),
		"launchConfiguration": summarizeLaunchConfiguration(out.LaunchConfigurations[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleGetInstanceIAM(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	instanceID := toString(req.Arguments["instanceId"])
	if instanceID == "" {
		return errorResult(errors.New("instanceId is required")), errors.New("instanceId is required")
	}
	if s.iamClient == nil {
		return errorResult(errors.New("iam client not available")), errors.New("iam client not available")
	}
	region := toString(req.Arguments["region"])
	ec2Client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		return errorResult(err), err
	}
	instance, found := findInstance(out.Reservations, instanceID)
	if !found {
		return errorResult(fmt.Errorf("instance %s not found", instanceID)), fmt.Errorf("instance %s not found", instanceID)
	}
	if instance.IamInstanceProfile == nil || instance.IamInstanceProfile.Arn == nil {
		result := map[string]any{
			"region":     regionOrDefault(usedRegion),
			"instanceId": instanceID,
			"iam":        "no instance profile attached",
		}
		return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
	}
	profileArn := aws.ToString(instance.IamInstanceProfile.Arn)
	profileName := instanceProfileNameFromArn(profileArn)
	if profileName == "" {
		return errorResult(fmt.Errorf("unable to parse instance profile name from ARN: %s", profileArn)), fmt.Errorf("unable to parse instance profile name from ARN: %s", profileArn)
	}
	iamClient, _, err := s.iamClient(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	profileOut, err := iamClient.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: aws.String(profileName)})
	if err != nil {
		return errorResult(err), err
	}
	profile := profileOut.InstanceProfile
	result := map[string]any{
		"region":     regionOrDefault(usedRegion),
		"instanceId": instanceID,
		"profile":    summarizeInstanceProfile(profile),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleGetSecurityGroupRules(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
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
	sg := out.SecurityGroups[0]
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"groupId":  groupID,
		"inbound":  summarizePermissions(sg.IpPermissions),
		"outbound": summarizePermissions(sg.IpPermissionsEgress),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListSpotInstanceRequests(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["spotInstanceRequestIds"])
	states := toStringSlice(req.Arguments["states"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeSpotInstanceRequestsInput{}
	if len(ids) > 0 {
		input.SpotInstanceRequestIds = ids
	}
	if len(states) > 0 {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("state"),
			Values: states,
		})
	}
	var requests []map[string]any
	for {
		out, err := client.DescribeSpotInstanceRequests(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, req := range out.SpotInstanceRequests {
			requests = append(requests, summarizeSpotRequest(req))
			if limit > 0 && len(requests) >= limit {
				break
			}
		}
		if limit > 0 && len(requests) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":               regionOrDefault(usedRegion),
		"spotInstanceRequests": requests,
		"count":                len(requests),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetSpotInstanceRequest(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	requestID := toString(req.Arguments["spotInstanceRequestId"])
	if requestID == "" {
		return errorResult(errors.New("spotInstanceRequestId is required")), errors.New("spotInstanceRequestId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeSpotInstanceRequests(ctx, &ec2.DescribeSpotInstanceRequestsInput{SpotInstanceRequestIds: []string{requestID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.SpotInstanceRequests) == 0 {
		return errorResult(fmt.Errorf("spot instance request %s not found", requestID)), fmt.Errorf("spot instance request %s not found", requestID)
	}
	result := map[string]any{
		"region":              regionOrDefault(usedRegion),
		"spotInstanceRequest": summarizeSpotRequest(out.SpotInstanceRequests[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListCapacityReservations(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["capacityReservationIds"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeCapacityReservationsInput{}
	if len(ids) > 0 {
		input.CapacityReservationIds = ids
	}
	var reservations []map[string]any
	for {
		out, err := client.DescribeCapacityReservations(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, res := range out.CapacityReservations {
			reservations = append(reservations, summarizeCapacityReservation(res))
			if limit > 0 && len(reservations) >= limit {
				break
			}
		}
		if limit > 0 && len(reservations) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":               regionOrDefault(usedRegion),
		"capacityReservations": reservations,
		"count":                len(reservations),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetCapacityReservation(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	resID := toString(req.Arguments["capacityReservationId"])
	if resID == "" {
		return errorResult(errors.New("capacityReservationId is required")), errors.New("capacityReservationId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeCapacityReservations(ctx, &ec2.DescribeCapacityReservationsInput{CapacityReservationIds: []string{resID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.CapacityReservations) == 0 {
		return errorResult(fmt.Errorf("capacity reservation %s not found", resID)), fmt.Errorf("capacity reservation %s not found", resID)
	}
	result := map[string]any{
		"region":              regionOrDefault(usedRegion),
		"capacityReservation": summarizeCapacityReservation(out.CapacityReservations[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListVolumes(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["volumeIds"])
	instanceID := toString(req.Arguments["instanceId"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeVolumesInput{}
	if len(ids) > 0 {
		input.VolumeIds = ids
	}
	if instanceID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("attachment.instance-id"),
			Values: []string{instanceID},
		})
	}
	var volumes []map[string]any
	for {
		out, err := client.DescribeVolumes(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, vol := range out.Volumes {
			volumes = append(volumes, summarizeVolume(vol))
			if limit > 0 && len(volumes) >= limit {
				break
			}
		}
		if limit > 0 && len(volumes) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":  regionOrDefault(usedRegion),
		"volumes": volumes,
		"count":   len(volumes),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetVolume(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	volumeID := toString(req.Arguments["volumeId"])
	if volumeID == "" {
		return errorResult(errors.New("volumeId is required")), errors.New("volumeId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{VolumeIds: []string{volumeID}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Volumes) == 0 {
		return errorResult(fmt.Errorf("volume %s not found", volumeID)), fmt.Errorf("volume %s not found", volumeID)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"volume": summarizeVolume(out.Volumes[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListSnapshots(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["snapshotIds"])
	owners := toStringSlice(req.Arguments["ownerIds"])
	volumeID := toString(req.Arguments["volumeId"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeSnapshotsInput{}
	if len(ids) > 0 {
		input.SnapshotIds = ids
	}
	if len(owners) == 0 {
		owners = []string{"self"}
	}
	input.OwnerIds = owners
	if volumeID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("volume-id"),
			Values: []string{volumeID},
		})
	}
	var snaps []map[string]any
	for {
		out, err := client.DescribeSnapshots(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, snap := range out.Snapshots {
			snaps = append(snaps, summarizeSnapshot(snap))
			if limit > 0 && len(snaps) >= limit {
				break
			}
		}
		if limit > 0 && len(snaps) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":    regionOrDefault(usedRegion),
		"snapshots": snaps,
		"count":     len(snaps),
		"owners":    owners,
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetSnapshot(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	snapshotID := toString(req.Arguments["snapshotId"])
	if snapshotID == "" {
		return errorResult(errors.New("snapshotId is required")), errors.New("snapshotId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
		OwnerIds:    []string{"self"},
	})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.Snapshots) == 0 {
		return errorResult(fmt.Errorf("snapshot %s not found", snapshotID)), fmt.Errorf("snapshot %s not found", snapshotID)
	}
	result := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"snapshot": summarizeSnapshot(out.Snapshots[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListVolumeAttachments(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	volumeID := toString(req.Arguments["volumeId"])
	instanceID := toString(req.Arguments["instanceId"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeVolumesInput{}
	if volumeID != "" {
		input.VolumeIds = []string{volumeID}
	}
	if instanceID != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("attachment.instance-id"),
			Values: []string{instanceID},
		})
	}
	var attachments []map[string]any
	for {
		out, err := client.DescribeVolumes(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, vol := range out.Volumes {
			for _, att := range vol.Attachments {
				attachments = append(attachments, map[string]any{
					"volumeId":            aws.ToString(vol.VolumeId),
					"instanceId":          aws.ToString(att.InstanceId),
					"state":               att.State,
					"device":              aws.ToString(att.Device),
					"attachTime":          att.AttachTime,
					"deleteOnTermination": att.DeleteOnTermination,
				})
				if limit > 0 && len(attachments) >= limit {
					break
				}
			}
			if limit > 0 && len(attachments) >= limit {
				break
			}
		}
		if limit > 0 && len(attachments) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":      regionOrDefault(usedRegion),
		"attachments": attachments,
		"count":       len(attachments),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleListPlacementGroups(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	names := toStringSlice(req.Arguments["groupNames"])
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribePlacementGroupsInput{}
	if len(names) > 0 {
		input.GroupNames = names
	}
	out, err := client.DescribePlacementGroups(ctx, input)
	if err != nil {
		return errorResult(err), err
	}
	var groups []map[string]any
	for _, group := range out.PlacementGroups {
		groups = append(groups, summarizePlacementGroup(group))
		if limit > 0 && len(groups) >= limit {
			break
		}
	}
	data := map[string]any{
		"region":          regionOrDefault(usedRegion),
		"placementGroups": groups,
		"count":           len(groups),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetPlacementGroup(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	name := toString(req.Arguments["groupName"])
	if name == "" {
		return errorResult(errors.New("groupName is required")), errors.New("groupName is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribePlacementGroups(ctx, &ec2.DescribePlacementGroupsInput{GroupNames: []string{name}})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.PlacementGroups) == 0 {
		return errorResult(fmt.Errorf("placement group %s not found", name)), fmt.Errorf("placement group %s not found", name)
	}
	result := map[string]any{
		"region":         regionOrDefault(usedRegion),
		"placementGroup": summarizePlacementGroup(out.PlacementGroups[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func (s *Service) handleListInstanceStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	region := toString(req.Arguments["region"])
	ids := toStringSlice(req.Arguments["instanceIds"])
	includeAll := toBool(req.Arguments["includeAll"], true)
	limit := toInt(req.Arguments["limit"], 100)
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	input := &ec2.DescribeInstanceStatusInput{IncludeAllInstances: aws.Bool(includeAll)}
	if len(ids) > 0 {
		input.InstanceIds = ids
	}
	var statuses []map[string]any
	for {
		out, err := client.DescribeInstanceStatus(ctx, input)
		if err != nil {
			return errorResult(err), err
		}
		for _, status := range out.InstanceStatuses {
			statuses = append(statuses, summarizeInstanceStatus(status))
			if limit > 0 && len(statuses) >= limit {
				break
			}
		}
		if limit > 0 && len(statuses) >= limit {
			break
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		input.NextToken = out.NextToken
	}
	data := map[string]any{
		"region":   regionOrDefault(usedRegion),
		"statuses": statuses,
		"count":    len(statuses),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(data)}, nil
}

func (s *Service) handleGetInstanceStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	instanceID := toString(req.Arguments["instanceId"])
	if instanceID == "" {
		return errorResult(errors.New("instanceId is required")), errors.New("instanceId is required")
	}
	region := toString(req.Arguments["region"])
	client, usedRegion, err := s.ec2Client(ctx, region)
	if err != nil {
		return errorResult(err), err
	}
	out, err := client.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
		InstanceIds:         []string{instanceID},
		IncludeAllInstances: aws.Bool(true),
	})
	if err != nil {
		return errorResult(err), err
	}
	if len(out.InstanceStatuses) == 0 {
		return errorResult(fmt.Errorf("instance status %s not found", instanceID)), fmt.Errorf("instance status %s not found", instanceID)
	}
	result := map[string]any{
		"region": regionOrDefault(usedRegion),
		"status": summarizeInstanceStatus(out.InstanceStatuses[0]),
	}
	return mcp.ToolResult{Data: s.ctx.Redactor.RedactValue(result)}, nil
}

func summarizeInstance(inst ec2types.Instance) map[string]any {
	var sgIDs []string
	for _, sg := range inst.SecurityGroups {
		sgIDs = append(sgIDs, aws.ToString(sg.GroupId))
	}
	return map[string]any{
		"id":                 aws.ToString(inst.InstanceId),
		"state":              inst.State,
		"type":               inst.InstanceType,
		"imageId":            aws.ToString(inst.ImageId),
		"vpcId":              aws.ToString(inst.VpcId),
		"subnetId":           aws.ToString(inst.SubnetId),
		"availabilityZone":   aws.ToString(inst.Placement.AvailabilityZone),
		"privateIp":          aws.ToString(inst.PrivateIpAddress),
		"publicIp":           aws.ToString(inst.PublicIpAddress),
		"keyName":            aws.ToString(inst.KeyName),
		"iamInstanceProfile": inst.IamInstanceProfile,
		"launchTime":         inst.LaunchTime,
		"securityGroupIds":   sgIDs,
		"tags":               tagMap(inst.Tags),
	}
}

func summarizeASG(group autotypes.AutoScalingGroup) map[string]any {
	var instances []map[string]any
	for _, inst := range group.Instances {
		instances = append(instances, map[string]any{
			"id":                   aws.ToString(inst.InstanceId),
			"lifecycleState":       inst.LifecycleState,
			"healthStatus":         aws.ToString(inst.HealthStatus),
			"availabilityZone":     aws.ToString(inst.AvailabilityZone),
			"protectedFromScaleIn": inst.ProtectedFromScaleIn,
		})
	}
	return map[string]any{
		"name":               aws.ToString(group.AutoScalingGroupName),
		"arn":                aws.ToString(group.AutoScalingGroupARN),
		"minSize":            group.MinSize,
		"maxSize":            group.MaxSize,
		"desiredCapacity":    group.DesiredCapacity,
		"availabilityZones":  group.AvailabilityZones,
		"vpcZoneIdentifiers": strings.Split(aws.ToString(group.VPCZoneIdentifier), ","),
		"healthCheckType":    aws.ToString(group.HealthCheckType),
		"healthCheckGrace":   group.HealthCheckGracePeriod,
		"targetGroupArns":    group.TargetGroupARNs,
		"loadBalancerNames":  group.LoadBalancerNames,
		"instances":          instances,
		"tags":               tagMapAutoScaling(group.Tags),
	}
}

func summarizeLoadBalancer(lb elbtypes.LoadBalancer) map[string]any {
	var zones []map[string]any
	for _, zone := range lb.AvailabilityZones {
		zones = append(zones, map[string]any{
			"zone":     aws.ToString(zone.ZoneName),
			"subnetId": aws.ToString(zone.SubnetId),
		})
	}
	return map[string]any{
		"arn":               aws.ToString(lb.LoadBalancerArn),
		"name":              aws.ToString(lb.LoadBalancerName),
		"type":              lb.Type,
		"scheme":            lb.Scheme,
		"vpcId":             aws.ToString(lb.VpcId),
		"state":             lb.State,
		"dnsName":           aws.ToString(lb.DNSName),
		"ipAddressType":     lb.IpAddressType,
		"securityGroups":    lb.SecurityGroups,
		"availabilityZones": zones,
	}
}

func summarizeTargetGroup(group elbtypes.TargetGroup) map[string]any {
	return map[string]any{
		"arn":                 aws.ToString(group.TargetGroupArn),
		"name":                aws.ToString(group.TargetGroupName),
		"protocol":            group.Protocol,
		"port":                group.Port,
		"targetType":          group.TargetType,
		"vpcId":               aws.ToString(group.VpcId),
		"healthCheckProtocol": group.HealthCheckProtocol,
		"healthCheckPort":     aws.ToString(group.HealthCheckPort),
		"healthCheckPath":     aws.ToString(group.HealthCheckPath),
		"matcher":             group.Matcher,
		"loadBalancerArns":    group.LoadBalancerArns,
	}
}

func summarizeListener(listener elbtypes.Listener) map[string]any {
	var certs []string
	for _, cert := range listener.Certificates {
		certs = append(certs, aws.ToString(cert.CertificateArn))
	}
	var actions []map[string]any
	for _, action := range listener.DefaultActions {
		item := map[string]any{
			"type": action.Type,
		}
		if action.TargetGroupArn != nil {
			item["targetGroupArn"] = aws.ToString(action.TargetGroupArn)
		}
		if action.ForwardConfig != nil {
			item["forwardConfig"] = action.ForwardConfig
		}
		actions = append(actions, item)
	}
	return map[string]any{
		"arn":             aws.ToString(listener.ListenerArn),
		"loadBalancerArn": aws.ToString(listener.LoadBalancerArn),
		"port":            listener.Port,
		"protocol":        listener.Protocol,
		"sslPolicy":       aws.ToString(listener.SslPolicy),
		"certificates":    certs,
		"defaultActions":  actions,
	}
}

func summarizeListenerRule(rule elbtypes.Rule) map[string]any {
	var actions []map[string]any
	for _, action := range rule.Actions {
		item := map[string]any{
			"type": action.Type,
		}
		if action.TargetGroupArn != nil {
			item["targetGroupArn"] = aws.ToString(action.TargetGroupArn)
		}
		if action.ForwardConfig != nil {
			item["forwardConfig"] = action.ForwardConfig
		}
		actions = append(actions, item)
	}
	return map[string]any{
		"arn":        aws.ToString(rule.RuleArn),
		"priority":   aws.ToString(rule.Priority),
		"isDefault":  rule.IsDefault,
		"conditions": rule.Conditions,
		"actions":    actions,
	}
}

func summarizeScalingPolicy(policy autotypes.ScalingPolicy) map[string]any {
	return map[string]any{
		"name":                   aws.ToString(policy.PolicyName),
		"arn":                    aws.ToString(policy.PolicyARN),
		"type":                   policy.PolicyType,
		"autoScalingGroup":       aws.ToString(policy.AutoScalingGroupName),
		"adjustmentType":         policy.AdjustmentType,
		"scalingAdjustment":      policy.ScalingAdjustment,
		"cooldown":               policy.Cooldown,
		"minAdjustmentMagnitude": policy.MinAdjustmentMagnitude,
		"targetTracking":         policy.TargetTrackingConfiguration,
		"stepAdjustments":        policy.StepAdjustments,
	}
}

func summarizeScalingActivity(activity autotypes.Activity) map[string]any {
	return map[string]any{
		"id":               aws.ToString(activity.ActivityId),
		"status":           activity.StatusCode,
		"description":      aws.ToString(activity.Description),
		"cause":            aws.ToString(activity.Cause),
		"autoScalingGroup": aws.ToString(activity.AutoScalingGroupName),
		"startTime":        activity.StartTime,
		"endTime":          activity.EndTime,
		"details":          aws.ToString(activity.Details),
		"progress":         activity.Progress,
	}
}

func summarizeLaunchTemplate(tmpl ec2types.LaunchTemplate) map[string]any {
	return map[string]any{
		"id":             aws.ToString(tmpl.LaunchTemplateId),
		"name":           aws.ToString(tmpl.LaunchTemplateName),
		"createdBy":      aws.ToString(tmpl.CreatedBy),
		"createTime":     tmpl.CreateTime,
		"defaultVersion": tmpl.DefaultVersionNumber,
		"latestVersion":  tmpl.LatestVersionNumber,
	}
}

func summarizeLaunchConfiguration(cfg autotypes.LaunchConfiguration) map[string]any {
	return map[string]any{
		"name":               aws.ToString(cfg.LaunchConfigurationName),
		"imageId":            aws.ToString(cfg.ImageId),
		"instanceType":       aws.ToString(cfg.InstanceType),
		"iamInstanceProfile": aws.ToString(cfg.IamInstanceProfile),
		"securityGroups":     cfg.SecurityGroups,
		"createdTime":        cfg.CreatedTime,
		"keyName":            aws.ToString(cfg.KeyName),
		"associatePublicIp":  cfg.AssociatePublicIpAddress,
		"ebsOptimized":       cfg.EbsOptimized,
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

func summarizeSpotRequest(req ec2types.SpotInstanceRequest) map[string]any {
	return map[string]any{
		"id":         aws.ToString(req.SpotInstanceRequestId),
		"state":      req.State,
		"status":     req.Status,
		"type":       req.Type,
		"instanceId": aws.ToString(req.InstanceId),
		"spotPrice":  aws.ToString(req.SpotPrice),
		"validFrom":  req.ValidFrom,
		"validUntil": req.ValidUntil,
		"launchSpec": req.LaunchSpecification,
		"createTime": req.CreateTime,
	}
}

func summarizeCapacityReservation(res ec2types.CapacityReservation) map[string]any {
	return map[string]any{
		"id":               aws.ToString(res.CapacityReservationId),
		"state":            res.State,
		"instanceType":     res.InstanceType,
		"availabilityZone": aws.ToString(res.AvailabilityZone),
		"tenancy":          res.Tenancy,
		"instanceCount":    res.TotalInstanceCount,
		"availableCount":   res.AvailableInstanceCount,
		"startDate":        res.StartDate,
		"endDate":          res.EndDate,
		"endDateType":      res.EndDateType,
	}
}

func summarizeVolume(vol ec2types.Volume) map[string]any {
	var attachments []map[string]any
	for _, att := range vol.Attachments {
		attachments = append(attachments, map[string]any{
			"instanceId": aws.ToString(att.InstanceId),
			"state":      att.State,
			"device":     aws.ToString(att.Device),
			"attachTime": att.AttachTime,
		})
	}
	return map[string]any{
		"id":               aws.ToString(vol.VolumeId),
		"state":            vol.State,
		"type":             vol.VolumeType,
		"sizeGiB":          vol.Size,
		"iops":             vol.Iops,
		"throughput":       vol.Throughput,
		"availabilityZone": aws.ToString(vol.AvailabilityZone),
		"encrypted":        vol.Encrypted,
		"snapshotId":       aws.ToString(vol.SnapshotId),
		"attachments":      attachments,
		"tags":             tagMap(vol.Tags),
		"createTime":       vol.CreateTime,
	}
}

func summarizeSnapshot(snap ec2types.Snapshot) map[string]any {
	return map[string]any{
		"id":         aws.ToString(snap.SnapshotId),
		"state":      snap.State,
		"volumeId":   aws.ToString(snap.VolumeId),
		"volumeSize": snap.VolumeSize,
		"startTime":  snap.StartTime,
		"ownerId":    aws.ToString(snap.OwnerId),
		"progress":   aws.ToString(snap.Progress),
		"encrypted":  snap.Encrypted,
		"tags":       tagMap(snap.Tags),
	}
}

func summarizePlacementGroup(group ec2types.PlacementGroup) map[string]any {
	return map[string]any{
		"name":           aws.ToString(group.GroupName),
		"state":          group.State,
		"strategy":       group.Strategy,
		"partitionCount": group.PartitionCount,
		"tags":           tagMap(group.Tags),
	}
}

func summarizeInstanceStatus(status ec2types.InstanceStatus) map[string]any {
	var events []map[string]any
	for _, event := range status.Events {
		events = append(events, map[string]any{
			"code":        event.Code,
			"description": aws.ToString(event.Description),
			"notBefore":   event.NotBefore,
			"notAfter":    event.NotAfter,
		})
	}
	return map[string]any{
		"instanceId":       aws.ToString(status.InstanceId),
		"availabilityZone": aws.ToString(status.AvailabilityZone),
		"state":            status.InstanceState,
		"systemStatus":     status.SystemStatus,
		"instanceStatus":   status.InstanceStatus,
		"events":           events,
	}
}

func findInstance(reservations []ec2types.Reservation, instanceID string) (ec2types.Instance, bool) {
	for _, reservation := range reservations {
		for _, inst := range reservation.Instances {
			if aws.ToString(inst.InstanceId) == instanceID {
				return inst, true
			}
		}
	}
	return ec2types.Instance{}, false
}

func instanceProfileNameFromArn(arn string) string {
	if arn == "" {
		return ""
	}
	parts := strings.Split(arn, "instance-profile/")
	if len(parts) < 2 {
		return ""
	}
	name := parts[len(parts)-1]
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.Contains(name, "/") {
		segments := strings.Split(name, "/")
		name = segments[len(segments)-1]
	}
	if name == "" {
		return ""
	}
	return name
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

func tagMapAutoScaling(tags []autotypes.TagDescription) map[string]string {
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

func toBool(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
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
