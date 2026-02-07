package k8s

import "testing"

func TestSchemaExecReadonly(t *testing.T) {
	schema := schemaExecReadonly()
	if schema["type"] == "" {
		t.Fatalf("expected schema type")
	}
}
