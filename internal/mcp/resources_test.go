package mcp

import (
	"context"
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
