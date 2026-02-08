package awskms

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestKMSSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaKMSListKeys(),
		schemaKMSListAliases(),
		schemaKMSDescribeKey(),
		schemaKMSGetKeyPolicy(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestKMSToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil)
	if len(specs) == 0 {
		t.Fatalf("expected kms tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.kms.list_keys" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.kms.list_keys")
	}
}
