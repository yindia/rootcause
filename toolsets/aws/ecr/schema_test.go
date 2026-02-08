package awsecr

import (
	"testing"

	"rootcause/internal/mcp"
)

func TestECRSchemas(t *testing.T) {
	schemas := []map[string]any{
		schemaECRListRepositories(),
		schemaECRDescribeRepository(),
		schemaECRListImages(),
		schemaECRDescribeImages(),
		schemaECRDescribeRegistry(),
		schemaECRGetAuthorizationToken(),
	}
	for i, schema := range schemas {
		if schema == nil || schema["type"] == "" {
			t.Fatalf("schema %d missing type", i)
		}
	}
}

func TestECRToolSpecs(t *testing.T) {
	specs := ToolSpecs(mcp.ToolsetContext{}, "aws", nil)
	if len(specs) == 0 {
		t.Fatalf("expected ecr tool specs")
	}
	found := false
	for _, spec := range specs {
		if spec.Name == "aws.ecr.list_repositories" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected aws.ecr.list_repositories")
	}
}
