package awsiam

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestIAMSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaIAMListRoles(),
		schemaIAMGetRole(),
		schemaIAMGetInstanceProfile(),
		schemaIAMUpdateRole(),
		schemaIAMDeleteRole(),
		schemaIAMListPolicies(),
		schemaIAMGetPolicy(),
		schemaIAMUpdatePolicy(),
		schemaIAMDeletePolicy(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestIAMToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil)
	if len(specs) == 0 {
		t.Fatalf("expected iam tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.iam.list_roles" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.iam.list_roles")
	}
}
