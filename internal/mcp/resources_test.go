package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
)

func TestResourceHandlerNamespaceList(t *testing.T) {
	typed := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "zeta"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}},
	)
	ctx := ToolContext{
		Config:   &config.Config{},
		Policy:   policy.NewAuthorizer(),
		Redactor: redact.New(),
		Clients:  &kube.Clients{Typed: typed, RestConfig: &rest.Config{Host: "https://example.com"}},
	}
	h := resourceHandler(ctx)
	res, err := h(context.Background(), &sdkmcp.ReadResourceRequest{Params: &sdkmcp.ReadResourceParams{URI: "namespace://list"}})
	if err != nil {
		t.Fatalf("resource handler: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected one content entry")
	}
	text := res.Contents[0].Text
	if !strings.Contains(text, "alpha") || !strings.Contains(text, "zeta") {
		t.Fatalf("expected namespace names in output: %s", text)
	}
}

func TestResourceHandlerSecretManifestRedactsData(t *testing.T) {
	typed := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "db-secret", Namespace: "default", Annotations: map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"data":{"password":"c3VwZXItc2VjcmV0LXZhbHVl"}}`}},
			Data:       map[string][]byte{"password": []byte("super-secret-value")},
		},
	)
	ctx := ToolContext{
		Config:   &config.Config{},
		Policy:   policy.NewAuthorizer(),
		Redactor: redact.New(),
		Clients:  &kube.Clients{Typed: typed, RestConfig: &rest.Config{Host: "https://example.com"}},
	}
	h := resourceHandler(ctx)
	res, err := h(context.Background(), &sdkmcp.ReadResourceRequest{Params: &sdkmcp.ReadResourceParams{URI: "manifest://secrets/default/db-secret"}})
	if err != nil {
		t.Fatalf("resource handler: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected one content entry")
	}
	text := res.Contents[0].Text
	if strings.Contains(text, "super-secret-value") {
		t.Fatalf("secret value leaked in manifest output: %s", text)
	}
	if strings.Contains(text, "c3VwZXItc2VjcmV0LXZhbHVl") {
		t.Fatalf("base64 secret leaked in manifest output: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("expected redacted marker in manifest output: %s", text)
	}
}

func TestKubeconfigPathExpandsHome(t *testing.T) {
	home := homedir.HomeDir()
	if home == "" {
		t.Skip("home directory unavailable")
	}
	resolved := kubeconfigPath("~/.kube/config")
	if resolved != filepath.Join(home, ".kube", "config") {
		t.Fatalf("unexpected kubeconfig expansion: %q", resolved)
	}
}

func TestRegisterSDKResources(t *testing.T) {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rootcause", Version: "test"}, nil)
	ctx := ToolContext{Policy: policy.NewAuthorizer()}
	uris, templates, err := RegisterSDKResources(server, ctx)
	if err != nil {
		t.Fatalf("register resources: %v", err)
	}
	if len(uris) == 0 || len(templates) == 0 {
		t.Fatalf("expected static resources and templates to be registered")
	}
}

func TestSkillResourcesIncludeConfiguredCustomSkills(t *testing.T) {
	customRoot := t.TempDir()
	customSkillDir := filepath.Join(customRoot, "team-runbook")
	err := os.MkdirAll(customSkillDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir custom skill: %v", err)
	}
	content := "---\ntags: [rootcause, team]\ndescription: Team custom runbook\n---\n# Team Runbook\n\nUse this for team incidents.\n"
	err = os.WriteFile(filepath.Join(customSkillDir, "SKILL.md"), []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write custom skill: %v", err)
	}
	ctx := ToolContext{
		Config: &config.Config{Skills: config.SkillsConfig{CustomDirs: []string{customRoot}}},
		Policy: policy.NewAuthorizer(),
	}
	resourceHandlerFunc := resourceHandler(ctx)

	catalogResult, err := resourceHandlerFunc(context.Background(), &sdkmcp.ReadResourceRequest{Params: &sdkmcp.ReadResourceParams{URI: "skill://catalog"}})
	if err != nil {
		t.Fatalf("read skill catalog: %v", err)
	}
	if !strings.Contains(catalogResult.Contents[0].Text, "team-runbook") {
		t.Fatalf("expected custom skill in catalog: %s", catalogResult.Contents[0].Text)
	}
	if !strings.Contains(catalogResult.Contents[0].Text, `"custom":true`) || !strings.Contains(catalogResult.Contents[0].Text, `"rootcause"`) {
		t.Fatalf("expected custom metadata and tags in catalog: %s", catalogResult.Contents[0].Text)
	}

	skillResult, err := resourceHandlerFunc(context.Background(), &sdkmcp.ReadResourceRequest{Params: &sdkmcp.ReadResourceParams{URI: "skill://team-runbook"}})
	if err != nil {
		t.Fatalf("read custom skill: %v", err)
	}
	if skillResult.Contents[0].Text != content {
		t.Fatalf("unexpected skill content: %q", skillResult.Contents[0].Text)
	}
}

func TestSkillResourceMissingCustomSkillReturnsNotFound(t *testing.T) {
	ctx := ToolContext{Config: &config.Config{}, Policy: policy.NewAuthorizer()}
	resourceHandlerFunc := resourceHandler(ctx)

	_, err := resourceHandlerFunc(context.Background(), &sdkmcp.ReadResourceRequest{Params: &sdkmcp.ReadResourceParams{URI: "skill://missing-skill"}})
	if err == nil {
		t.Fatalf("expected missing skill resource error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}
