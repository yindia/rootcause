package awsvpc

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestVPCSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaVPCListVPCs(),
		schemaVPCGetVPC(),
		schemaVPCListSubnets(),
		schemaVPCGetSubnet(),
		schemaVPCListRouteTables(),
		schemaVPCGetRouteTable(),
		schemaVPCListNatGateways(),
		schemaVPCGetNatGateway(),
		schemaVPCListSecurityGroups(),
		schemaVPCGetSecurityGroup(),
		schemaVPCListNetworkAcls(),
		schemaVPCGetNetworkAcl(),
		schemaVPCListInternetGateways(),
		schemaVPCGetInternetGateway(),
		schemaVPCListEndpoints(),
		schemaVPCGetEndpoint(),
		schemaVPCListNetworkInterfaces(),
		schemaVPCGetNetworkInterface(),
		schemaVPCListResolverEndpoints(),
		schemaVPCGetResolverEndpoint(),
		schemaVPCListResolverRules(),
		schemaVPCGetResolverRule(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestVPCToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil, nil)
	if len(specs) == 0 {
		t.Fatalf("expected vpc tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.vpc.list_vpcs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.vpc.list_vpcs")
	}
}
