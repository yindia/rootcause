package awsvpc

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53resolver/types"
)

func TestSummarizersExtended(t *testing.T) {
	now := time.Now()
	routeTable := ec2types.RouteTable{
		RouteTableId: aws.String("rtb-1"),
		VpcId:        aws.String("vpc-1"),
		Routes: []ec2types.Route{
			{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-1")},
		},
		Associations: []ec2types.RouteTableAssociation{
			{RouteTableAssociationId: aws.String("rtbassoc-1"), SubnetId: aws.String("subnet-1"), Main: aws.Bool(true)},
		},
		Tags: []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
	}
	tableSummary := summarizeRouteTable(routeTable)
	if tableSummary["id"] != "rtb-1" {
		t.Fatalf("unexpected route table summary: %#v", tableSummary)
	}

	nat := ec2types.NatGateway{
		NatGatewayId: aws.String("nat-1"),
		VpcId:        aws.String("vpc-1"),
		SubnetId:     aws.String("subnet-1"),
		State:        ec2types.NatGatewayStateAvailable,
		CreateTime:   &now,
		NatGatewayAddresses: []ec2types.NatGatewayAddress{
			{AllocationId: aws.String("eipalloc-1"), PublicIp: aws.String("1.2.3.4"), PrivateIp: aws.String("10.0.0.5")},
		},
	}
	natSummary := summarizeNatGateway(nat)
	if natSummary["id"] != "nat-1" {
		t.Fatalf("unexpected nat gateway summary: %#v", natSummary)
	}

	sg := ec2types.SecurityGroup{
		GroupId:     aws.String("sg-1"),
		GroupName:   aws.String("web"),
		Description: aws.String("demo"),
		VpcId:       aws.String("vpc-1"),
		IpPermissions: []ec2types.IpPermission{
			{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(80), ToPort: aws.Int32(80), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
	}
	sgSummary := summarizeSecurityGroup(sg)
	if sgSummary["id"] != "sg-1" {
		t.Fatalf("unexpected security group summary: %#v", sgSummary)
	}

	acl := ec2types.NetworkAcl{
		NetworkAclId: aws.String("acl-1"),
		VpcId:        aws.String("vpc-1"),
		IsDefault:    aws.Bool(true),
		Entries: []ec2types.NetworkAclEntry{
			{RuleNumber: aws.Int32(100), Protocol: aws.String("6"), RuleAction: ec2types.RuleActionAllow, Egress: aws.Bool(false), CidrBlock: aws.String("0.0.0.0/0")},
		},
		Associations: []ec2types.NetworkAclAssociation{
			{NetworkAclAssociationId: aws.String("aclassoc-1"), SubnetId: aws.String("subnet-1")},
		},
	}
	aclSummary := summarizeNetworkAcl(acl)
	if aclSummary["id"] != "acl-1" {
		t.Fatalf("unexpected network acl summary: %#v", aclSummary)
	}

	igw := ec2types.InternetGateway{
		InternetGatewayId: aws.String("igw-1"),
		Attachments: []ec2types.InternetGatewayAttachment{
			{VpcId: aws.String("vpc-1"), State: ec2types.AttachmentStatusAttached},
		},
	}
	igwSummary := summarizeInternetGateway(igw)
	if igwSummary["id"] != "igw-1" {
		t.Fatalf("unexpected internet gateway summary: %#v", igwSummary)
	}

	endpoint := ec2types.VpcEndpoint{
		VpcEndpointId:       aws.String("vpce-1"),
		VpcId:               aws.String("vpc-1"),
		ServiceName:         aws.String("com.amazonaws.vpce"),
		VpcEndpointType:     ec2types.VpcEndpointTypeInterface,
		State:               ec2types.StateAvailable,
		SubnetIds:           []string{"subnet-1"},
		NetworkInterfaceIds: []string{"eni-1"},
		Groups: []ec2types.SecurityGroupIdentifier{
			{GroupId: aws.String("sg-1")},
		},
	}
	epSummary := summarizeVpcEndpoint(endpoint)
	if epSummary["id"] != "vpce-1" {
		t.Fatalf("unexpected vpc endpoint summary: %#v", epSummary)
	}

	iface := ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-1"),
		Status:             ec2types.NetworkInterfaceStatusInUse,
		Description:        aws.String("demo"),
		InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
		VpcId:              aws.String("vpc-1"),
		SubnetId:           aws.String("subnet-1"),
		PrivateIpAddress:   aws.String("10.0.0.10"),
		PrivateIpAddresses: []ec2types.NetworkInterfacePrivateIpAddress{{PrivateIpAddress: aws.String("10.0.0.10")}},
		Groups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-1")},
		},
		Attachment: &ec2types.NetworkInterfaceAttachment{AttachmentId: aws.String("attach-1"), InstanceId: aws.String("i-1"), DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached},
	}
	ifaceSummary := summarizeNetworkInterface(iface)
	if ifaceSummary["id"] != "eni-1" {
		t.Fatalf("unexpected network interface summary: %#v", ifaceSummary)
	}

	resolverEndpoint := r53types.ResolverEndpoint{
		Id:               aws.String("rslvr-endpoint-1"),
		Name:             aws.String("resolver"),
		Direction:        r53types.ResolverEndpointDirectionInbound,
		Status:           r53types.ResolverEndpointStatusOperational,
		HostVPCId:        aws.String("vpc-1"),
		SecurityGroupIds: []string{"sg-1"},
		IpAddressCount:   aws.Int32(1),
		CreationTime:     aws.String(now.Format(time.RFC3339)),
	}
	endpointSummary := summarizeResolverEndpoint(resolverEndpoint)
	if endpointSummary["id"] != "rslvr-endpoint-1" {
		t.Fatalf("unexpected resolver endpoint summary: %#v", endpointSummary)
	}

	rule := r53types.ResolverRule{
		Id:                 aws.String("rslvr-rule-1"),
		Name:               aws.String("corp"),
		DomainName:         aws.String("corp.local"),
		RuleType:           r53types.RuleTypeOptionForward,
		Status:             r53types.ResolverRuleStatusComplete,
		ResolverEndpointId: aws.String("rslvr-endpoint-1"),
		TargetIps:          []r53types.TargetAddress{{Ip: aws.String("10.0.0.2"), Port: aws.Int32(53)}},
	}
	ruleSummary := summarizeResolverRule(rule)
	if ruleSummary["id"] != "rslvr-rule-1" {
		t.Fatalf("unexpected resolver rule summary: %#v", ruleSummary)
	}

	if len(summarizeCidrBlocks([]ec2types.VpcCidrBlockAssociation{{CidrBlock: aws.String("10.0.0.0/16")}})) != 1 {
		t.Fatalf("expected cidr summary")
	}
	if len(summarizeIpv6Cidr([]ec2types.VpcIpv6CidrBlockAssociation{{Ipv6CidrBlock: aws.String("::/0")}})) != 1 {
		t.Fatalf("expected ipv6 summary")
	}
	if len(summarizeSubnetIpv6Cidr([]ec2types.SubnetIpv6CidrBlockAssociation{{Ipv6CidrBlock: aws.String("::/64")}})) != 1 {
		t.Fatalf("expected subnet ipv6 summary")
	}

	filters := summarizeNatFilters("vpc-1", "subnet-1", []string{"nat-1"})
	if filters["vpcId"] != "vpc-1" {
		t.Fatalf("unexpected nat filters: %#v", filters)
	}
}
