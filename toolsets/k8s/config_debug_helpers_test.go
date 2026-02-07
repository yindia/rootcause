package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestConfigDebugHelpers(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}, Data: map[string]string{"foo": "bar"}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"}, Data: map[string][]byte{"token": []byte("value")}}
	client := k8sfake.NewSimpleClientset(cm, secret)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	issue := toolset.checkConfigMapKeys(context.Background(), "default", "app", []string{"foo", "missing"}, "direct", false, "")
	if len(issue.MissingKeys) == 0 {
		t.Fatalf("expected missing keys issue")
	}
	secretIssue := toolset.checkSecretKeys(context.Background(), "default", "sec", []string{"token", "missing"}, "direct", false, "")
	if len(secretIssue.MissingKeys) == 0 {
		t.Fatalf("expected missing secret keys")
	}
	keys := volumeItems([]corev1.KeyToPath{{Key: "foo"}, {Key: "bar"}})
	if len(keys) != 2 {
		t.Fatalf("expected volume items")
	}
}

func TestConfigDebugMissingRefs(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	missingCM := toolset.checkConfigMapKeys(context.Background(), "default", "", []string{"key"}, "direct", false, "")
	if !missingCM.Missing {
		t.Fatalf("expected missing configmap when name is empty")
	}
	missingSecret := toolset.checkSecretKeys(context.Background(), "default", "", []string{"key"}, "direct", false, "")
	if !missingSecret.Missing {
		t.Fatalf("expected missing secret when name is empty")
	}
}
