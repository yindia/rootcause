package awsec2

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestEC2Schemas(t *testing.T) {
	schemas := []map[string]any{
		schemaEC2ListInstances(),
		schemaEC2GetInstance(),
		schemaEC2ListASGs(),
		schemaEC2GetASG(),
		schemaEC2ListLoadBalancers(),
		schemaEC2GetLoadBalancer(),
		schemaEC2ListTargetGroups(),
		schemaEC2GetTargetGroup(),
		schemaEC2ListListeners(),
		schemaEC2GetListener(),
		schemaEC2GetTargetHealth(),
		schemaEC2ListListenerRules(),
		schemaEC2GetListenerRule(),
		schemaEC2ListAutoScalingPolicies(),
		schemaEC2GetAutoScalingPolicy(),
		schemaEC2ListScalingActivities(),
		schemaEC2GetScalingActivity(),
		schemaEC2ListLaunchTemplates(),
		schemaEC2GetLaunchTemplate(),
		schemaEC2ListLaunchConfigurations(),
		schemaEC2GetLaunchConfiguration(),
		schemaEC2GetInstanceIAM(),
		schemaEC2GetSecurityGroupRules(),
		schemaEC2ListSpotInstanceRequests(),
		schemaEC2GetSpotInstanceRequest(),
		schemaEC2ListCapacityReservations(),
		schemaEC2GetCapacityReservation(),
		schemaEC2ListVolumes(),
		schemaEC2GetVolume(),
		schemaEC2ListSnapshots(),
		schemaEC2GetSnapshot(),
		schemaEC2ListVolumeAttachments(),
		schemaEC2ListPlacementGroups(),
		schemaEC2GetPlacementGroup(),
		schemaEC2ListInstanceStatus(),
		schemaEC2GetInstanceStatus(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestEC2ToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil, nil, nil, nil)
	if len(specs) == 0 {
		t.Fatalf("expected ec2 tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.ec2.list_instances" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.ec2.list_instances")
	}
}
