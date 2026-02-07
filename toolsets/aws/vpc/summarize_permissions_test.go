package awsvpc

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestSummarizePermissionsVariants(t *testing.T) {
	perms := []ec2types.IpPermission{
		{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(80),
			ToPort:     aws.Int32(80),
			IpRanges: []ec2types.IpRange{
				{CidrIp: aws.String("0.0.0.0/0")},
			},
			Ipv6Ranges: []ec2types.Ipv6Range{
				{CidrIpv6: aws.String("::/0")},
			},
			UserIdGroupPairs: []ec2types.UserIdGroupPair{
				{GroupId: aws.String("sg-1"), GroupName: aws.String("web"), UserId: aws.String("123"), VpcId: aws.String("vpc-1")},
			},
			PrefixListIds: []ec2types.PrefixListId{
				{PrefixListId: aws.String("pl-1")},
			},
		},
	}
	out := summarizePermissions(perms)
	if len(out) != 1 {
		t.Fatalf("expected one permission entry, got %#v", out)
	}
	entry := out[0]
	if entry["ipv4Ranges"] == nil || entry["ipv6Ranges"] == nil || entry["securityGroups"] == nil || entry["prefixListIds"] == nil {
		t.Fatalf("expected permission fields, got %#v", entry)
	}
}
