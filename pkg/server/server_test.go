package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/config"

	_ "rootcause/toolsets/k8s"
)

func TestBuildRuntimeMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = kubeconfigPath
	cfg.Toolsets = []string{}

	toolCtx, reg, err := buildRuntime(cfg, io.Discard)
	if err != nil {
		t.Fatalf("buildRuntime failed: %v", err)
	}
	if toolCtx.Clients == nil {
		t.Fatalf("expected clients")
	}
	if reg == nil {
		t.Fatalf("expected registry")
	}
	if len(reg.Names()) != 0 {
		t.Fatalf("expected no tools registered")
	}
}

func TestRunWithInMemoryTransport(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`toolsets = ["k8s"]`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := Run(ctx, Options{
		ConfigPath: configPath,
		Kubeconfig: kubeconfigPath,
		Version:    "test",
		Stderr:     io.Discard,
		Transport:  fakeTransport{},
	})
	if time.Since(start) > time.Second {
		t.Fatalf("run took too long")
	}
	_ = err
}

func TestBuildRuntimeUnknownToolset(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = kubeconfigPath
	cfg.Toolsets = []string{"missing"}

	_, _, err := buildRuntime(cfg, io.Discard)
	if err == nil {
		t.Fatalf("expected error for unknown toolset")
	}
}

func TestRunConfigLoadError(t *testing.T) {
	t.Setenv("ROOTCAUSE_CONFIG", "")
	err := Run(context.Background(), Options{
		ConfigPath: filepath.Join(t.TempDir(), "missing.toml"),
		Version:    "test",
		Stderr:     io.Discard,
		Transport:  fakeTransport{},
	})
	if err == nil {
		t.Fatalf("expected error for config load failure")
	}
}

type fakeTransport struct{}

func (fakeTransport) Connect(context.Context) (sdkmcp.Connection, error) {
	return &fakeConn{}, nil
}

type fakeConn struct{}

func (c *fakeConn) Read(context.Context) (sdkjsonrpc.Message, error) {
	return nil, io.EOF
}

func (c *fakeConn) Write(context.Context, sdkjsonrpc.Message) error {
	return nil
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) SessionID() string {
	return "test"
}
