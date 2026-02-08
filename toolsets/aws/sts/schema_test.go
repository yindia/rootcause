package awssts

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestSTSSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaSTSGetCallerIdentity(),
		schemaSTSAssumeRole(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestSTSToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil)
	if len(specs) == 0 {
		t.Fatalf("expected sts tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.sts.get_caller_identity" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.sts.get_caller_identity")
	}
}
