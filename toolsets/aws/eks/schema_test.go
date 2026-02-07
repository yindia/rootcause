package awseks

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestEKSSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaEKSListClusters(),
		schemaEKSGetCluster(),
		schemaEKSListNodegroups(),
		schemaEKSGetNodegroup(),
		schemaEKSListAddons(),
		schemaEKSGetAddon(),
		schemaEKSListFargateProfiles(),
		schemaEKSGetFargateProfile(),
		schemaEKSListIdentityProviderConfigs(),
		schemaEKSGetIdentityProviderConfig(),
		schemaEKSListUpdates(),
		schemaEKSGetUpdate(),
		schemaEKSListNodes(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestEKSToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil, nil, nil)
	if len(specs) == 0 {
		t.Fatalf("expected eks tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.eks.list_clusters" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.eks.list_clusters")
	}
}
