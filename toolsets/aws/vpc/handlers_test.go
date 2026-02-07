package awsvpc

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestVPCHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	ec2Called := false
	r53Called := false
	svc := &Service{
		ctx: ctx,
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			ec2Called = true
			return nil, "", nil
		},
		resolverClient: func(context.Context, string) (*route53resolver.Client, string, error) {
			r53Called = true
			return nil, "", nil
		},
	}

	tests := []struct {
		name    string
		handler func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{"getVPCMissing", svc.handleGetVPC, map[string]any{}, "vpcId is required"},
		{"getSubnetMissing", svc.handleGetSubnet, map[string]any{}, "subnetId is required"},
		{"getRouteTableMissing", svc.handleGetRouteTable, map[string]any{}, "routeTableId is required"},
		{"getNatGatewayMissing", svc.handleGetNatGateway, map[string]any{}, "natGatewayId is required"},
		{"getSecurityGroupMissing", svc.handleGetSecurityGroup, map[string]any{}, "groupId is required"},
		{"getNetworkAclMissing", svc.handleGetNetworkAcl, map[string]any{}, "networkAclId is required"},
		{"getInternetGatewayMissing", svc.handleGetInternetGateway, map[string]any{}, "internetGatewayId is required"},
		{"getEndpointMissing", svc.handleGetEndpoint, map[string]any{}, "endpointId is required"},
		{"getNetworkInterfaceMissing", svc.handleGetNetworkInterface, map[string]any{}, "networkInterfaceId is required"},
		{"getResolverEndpointMissing", svc.handleGetResolverEndpoint, map[string]any{}, "resolverEndpointId is required"},
		{"getResolverRuleMissing", svc.handleGetResolverRule, map[string]any{}, "resolverRuleId is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Called = false
			r53Called = false
			_, err := tt.handler(context.Background(), mcp.ToolRequest{Arguments: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
			if ec2Called || r53Called {
				t.Fatalf("client should not be invoked")
			}
		})
	}
}
