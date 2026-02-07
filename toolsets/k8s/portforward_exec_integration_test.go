package k8s

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newRestClientset(t *testing.T) (*kubernetes.Clientset, *rest.Config) {
	t.Helper()
	cfg := &rest.Config{
		Host:    "http://127.0.0.1:1",
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		},
		Timeout: 200 * time.Millisecond,
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes.NewForConfig: %v", err)
	}
	return clientset, cfg
}

func TestHandlePortForwardContextCancel(t *testing.T) {
	clientset, cfg := newRestClientset(t)
	appCfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &appCfg,
		Clients:  &kube.Clients{Typed: clientset, RestConfig: cfg},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := toolset.handlePortForward(ctx, mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
			"ports":     []any{"8080:80"},
		},
	})
	if err == nil {
		t.Fatalf("expected port-forward error")
	}
}

func TestHandleExecReadonlyExecCommand(t *testing.T) {
	clientset, restCfg := newRestClientset(t)
	appCfg := config.DefaultConfig()
	appCfg.Exec.AllowedCommands = []string{"ls"}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &appCfg,
		Clients:  &kube.Clients{Typed: clientset, RestConfig: restCfg},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := toolset.handleExecReadonly(ctx, mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
			"command":   []any{"ls"},
		},
	})
	if err == nil {
		t.Fatalf("expected exec error")
	}
}
