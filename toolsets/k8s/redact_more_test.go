package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestRedactUnstructuredSecret(t *testing.T) {
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
		},
		"data": map[string]any{
			"token": "abc",
		},
		"stringData": map[string]any{
			"password": "secret",
		},
	}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	redacted := toolset.redactUnstructured(secret)
	data, ok := redacted["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected redacted data map, got %#v", redacted["data"])
	}
	if data["token"] != "[REDACTED]" {
		t.Fatalf("expected token redaction, got %#v", data["token"])
	}
	if redacted["stringData"] != "[REDACTED]" {
		t.Fatalf("expected stringData redaction, got %#v", redacted["stringData"])
	}
}
