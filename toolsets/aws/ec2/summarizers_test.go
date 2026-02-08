package awsec2

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	autotypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func TestSummarizersExtended(t *testing.T) {
	now := time.Now()
	lb := elbtypes.LoadBalancer{
		LoadBalancerArn:  aws.String("arn:lb"),
		LoadBalancerName: aws.String("demo"),
		Type:             elbtypes.LoadBalancerTypeEnumApplication,
		Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
		VpcId:            aws.String("vpc-1"),
		State:            &elbtypes.LoadBalancerState{Code: elbtypes.LoadBalancerStateEnumActive},
		DNSName:          aws.String("demo.aws"),
		IpAddressType:    elbtypes.IpAddressTypeIpv4,
		SecurityGroups:   []string{"sg-1"},
		AvailabilityZones: []elbtypes.AvailabilityZone{
			{ZoneName: aws.String("us-east-1a"), SubnetId: aws.String("subnet-1")},
		},
	}
	lbSummary := summarizeLoadBalancer(lb)
	if lbSummary["name"] != "demo" {
		t.Fatalf("unexpected load balancer summary: %#v", lbSummary)
	}

	targetGroup := elbtypes.TargetGroup{
		TargetGroupArn:  aws.String("arn:tg"),
		TargetGroupName: aws.String("tg"),
		Protocol:        elbtypes.ProtocolEnumHttp,
		Port:            aws.Int32(80),
		VpcId:           aws.String("vpc-1"),
	}
	tgSummary := summarizeTargetGroup(targetGroup)
	if tgSummary["name"] != "tg" {
		t.Fatalf("unexpected target group summary: %#v", tgSummary)
	}

	listener := elbtypes.Listener{
		ListenerArn:     aws.String("arn:listener"),
		LoadBalancerArn: aws.String("arn:lb"),
		Port:            aws.Int32(443),
		Protocol:        elbtypes.ProtocolEnumHttps,
		SslPolicy:       aws.String("ELBSecurityPolicy-2016-08"),
		Certificates: []elbtypes.Certificate{
			{CertificateArn: aws.String("arn:cert")},
		},
		DefaultActions: []elbtypes.Action{
			{
				Type:           elbtypes.ActionTypeEnumForward,
				TargetGroupArn: aws.String("arn:tg"),
				ForwardConfig:  &elbtypes.ForwardActionConfig{},
			},
		},
	}
	listenerSummary := summarizeListener(listener)
	if listenerSummary["arn"] != "arn:listener" {
		t.Fatalf("unexpected listener summary: %#v", listenerSummary)
	}

	rule := elbtypes.Rule{
		RuleArn:   aws.String("arn:rule"),
		Priority:  aws.String("1"),
		IsDefault: aws.Bool(false),
		Actions: []elbtypes.Action{
			{Type: elbtypes.ActionTypeEnumForward, TargetGroupArn: aws.String("arn:tg")},
		},
	}
	ruleSummary := summarizeListenerRule(rule)
	if ruleSummary["arn"] != "arn:rule" {
		t.Fatalf("unexpected rule summary: %#v", ruleSummary)
	}

	policy := autotypes.ScalingPolicy{
		PolicyName:           aws.String("scale"),
		PolicyARN:            aws.String("arn:policy"),
		PolicyType:           aws.String("TargetTrackingScaling"),
		AutoScalingGroupName: aws.String("asg"),
		ScalingAdjustment:    aws.Int32(2),
	}
	policySummary := summarizeScalingPolicy(policy)
	if policySummary["name"] != "scale" {
		t.Fatalf("unexpected policy summary: %#v", policySummary)
	}

	activity := autotypes.Activity{
		ActivityId:           aws.String("act-1"),
		StatusCode:           autotypes.ScalingActivityStatusCodeSuccessful,
		AutoScalingGroupName: aws.String("asg"),
		Progress:             aws.Int32(100),
		StartTime:            &now,
	}
	activitySummary := summarizeScalingActivity(activity)
	if activitySummary["id"] != "act-1" {
		t.Fatalf("unexpected activity summary: %#v", activitySummary)
	}

	launchTemplate := ec2types.LaunchTemplate{
		LaunchTemplateId:   aws.String("lt-1"),
		LaunchTemplateName: aws.String("tmpl"),
		CreatedBy:          aws.String("me"),
		CreateTime:         &now,
	}
	tmplSummary := summarizeLaunchTemplate(launchTemplate)
	if tmplSummary["id"] != "lt-1" {
		t.Fatalf("unexpected launch template summary: %#v", tmplSummary)
	}

	launchConfig := autotypes.LaunchConfiguration{
		LaunchConfigurationName: aws.String("lc-1"),
		ImageId:                 aws.String("ami-1"),
		InstanceType:            aws.String("t3.micro"),
		IamInstanceProfile:      aws.String("profile"),
		SecurityGroups:          []string{"sg-1"},
		CreatedTime:             &now,
	}
	configSummary := summarizeLaunchConfiguration(launchConfig)
	if configSummary["name"] != "lc-1" {
		t.Fatalf("unexpected launch configuration summary: %#v", configSummary)
	}

	instanceProfile := &iamtypes.InstanceProfile{
		InstanceProfileName: aws.String("profile"),
		Arn:                 aws.String("arn:profile"),
		Roles: []iamtypes.Role{
			{RoleName: aws.String("role")},
		},
	}
	profileSummary := summarizeInstanceProfile(instanceProfile)
	if profileSummary["name"] != "profile" {
		t.Fatalf("unexpected instance profile summary: %#v", profileSummary)
	}
	if summarizeInstanceProfile(nil)["status"] != "not found" {
		t.Fatalf("expected nil instance profile status")
	}

	perms := []ec2types.IpPermission{
		{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(80),
			ToPort:     aws.Int32(80),
			IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16")}},
			Ipv6Ranges: []ec2types.Ipv6Range{{CidrIpv6: aws.String("::/0")}},
			UserIdGroupPairs: []ec2types.UserIdGroupPair{
				{GroupId: aws.String("sg-1"), UserId: aws.String("123")},
			},
			PrefixListIds: []ec2types.PrefixListId{
				{PrefixListId: aws.String("pl-1")},
			},
		},
	}
	permsSummary := summarizePermissions(perms)
	if len(permsSummary) != 1 {
		t.Fatalf("unexpected permissions summary: %#v", permsSummary)
	}

	spot := ec2types.SpotInstanceRequest{
		SpotInstanceRequestId: aws.String("sir-1"),
		State:                 ec2types.SpotInstanceStateActive,
		Status:                &ec2types.SpotInstanceStatus{},
		Type:                  ec2types.SpotInstanceTypeOneTime,
		InstanceId:            aws.String("i-1"),
		SpotPrice:             aws.String("0.03"),
		ValidFrom:             &now,
		ValidUntil:            &now,
		LaunchSpecification:   &ec2types.LaunchSpecification{},
		CreateTime:            &now,
	}
	spotSummary := summarizeSpotRequest(spot)
	if spotSummary["id"] != "sir-1" {
		t.Fatalf("unexpected spot request summary: %#v", spotSummary)
	}

	reservation := ec2types.CapacityReservation{
		CapacityReservationId:  aws.String("cr-1"),
		State:                  ec2types.CapacityReservationStateActive,
		InstanceType:           aws.String(string(ec2types.InstanceTypeT3Micro)),
		AvailabilityZone:       aws.String("us-east-1a"),
		Tenancy:                ec2types.CapacityReservationTenancyDefault,
		TotalInstanceCount:     aws.Int32(2),
		AvailableInstanceCount: aws.Int32(1),
		StartDate:              &now,
		EndDate:                &now,
		EndDateType:            ec2types.EndDateTypeLimited,
	}
	resSummary := summarizeCapacityReservation(reservation)
	if resSummary["id"] != "cr-1" {
		t.Fatalf("unexpected capacity reservation summary: %#v", resSummary)
	}

	volume := ec2types.Volume{
		VolumeId:         aws.String("vol-1"),
		State:            ec2types.VolumeStateInUse,
		VolumeType:       ec2types.VolumeTypeGp3,
		Size:             aws.Int32(10),
		Iops:             aws.Int32(3000),
		Throughput:       aws.Int32(125),
		AvailabilityZone: aws.String("us-east-1a"),
		Encrypted:        aws.Bool(true),
		SnapshotId:       aws.String("snap-1"),
		Attachments: []ec2types.VolumeAttachment{
			{InstanceId: aws.String("i-1"), State: ec2types.VolumeAttachmentStateAttached, Device: aws.String("/dev/xvda"), AttachTime: &now},
		},
		Tags:       []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
		CreateTime: &now,
	}
	volSummary := summarizeVolume(volume)
	if volSummary["id"] != "vol-1" {
		t.Fatalf("unexpected volume summary: %#v", volSummary)
	}

	snapshot := ec2types.Snapshot{
		SnapshotId: aws.String("snap-1"),
		State:      ec2types.SnapshotStateCompleted,
		VolumeId:   aws.String("vol-1"),
		VolumeSize: aws.Int32(10),
		StartTime:  &now,
		OwnerId:    aws.String("123"),
		Progress:   aws.String("100%"),
		Encrypted:  aws.Bool(false),
		Tags:       []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
	}
	snapSummary := summarizeSnapshot(snapshot)
	if snapSummary["id"] != "snap-1" {
		t.Fatalf("unexpected snapshot summary: %#v", snapSummary)
	}

	placement := ec2types.PlacementGroup{
		GroupName:      aws.String("pg-1"),
		State:          ec2types.PlacementGroupStateAvailable,
		Strategy:       ec2types.PlacementStrategyCluster,
		PartitionCount: aws.Int32(1),
		Tags:           []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
	}
	placementSummary := summarizePlacementGroup(placement)
	if placementSummary["name"] != "pg-1" {
		t.Fatalf("unexpected placement group summary: %#v", placementSummary)
	}

	status := ec2types.InstanceStatus{
		InstanceId:       aws.String("i-1"),
		AvailabilityZone: aws.String("us-east-1a"),
		InstanceState:    &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		SystemStatus:     &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
		InstanceStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
		Events: []ec2types.InstanceStatusEvent{
			{Code: ec2types.EventCodeInstanceReboot, Description: aws.String("reboot"), NotBefore: &now, NotAfter: &now},
		},
	}
	statusSummary := summarizeInstanceStatus(status)
	if statusSummary["instanceId"] != "i-1" {
		t.Fatalf("unexpected instance status summary: %#v", statusSummary)
	}

	reservations := []ec2types.Reservation{
		{Instances: []ec2types.Instance{{InstanceId: aws.String("i-2")}}},
	}
	if _, found := findInstance(reservations, "i-2"); !found {
		t.Fatalf("expected instance to be found")
	}
	if _, found := findInstance(reservations, "i-3"); found {
		t.Fatalf("expected instance to be missing")
	}

	if got := instanceProfileNameFromArn("arn:aws:iam::123:instance-profile/path/profile"); got != "profile" {
		t.Fatalf("unexpected instance profile name: %q", got)
	}
	if got := instanceProfileNameFromArn(""); got != "" {
		t.Fatalf("expected empty instance profile name")
	}
}
