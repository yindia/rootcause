package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/config"
	rcmcp "rootcause/internal/mcp"

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

func TestRunUsesEnvConfig(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("kubeconfig = %q\ntoolsets = [\"k8s\"]\n", kubeconfigPath)), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ROOTCAUSE_CONFIG", configPath)

	err := Run(context.Background(), Options{
		ConfigPath: "",
		Version:    "test",
		Stderr:     io.Discard,
		Transport:  fakeTransport{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRunTransportError(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("kubeconfig = %q\ntoolsets = [\"k8s\"]\n", kubeconfigPath)), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := Run(context.Background(), Options{
		ConfigPath: configPath,
		Version:    "test",
		Stderr:     io.Discard,
		Transport:  errorTransport{},
	})
	if err == nil {
		t.Fatalf("expected server error")
	}
}

func TestRunOverridesApplied(t *testing.T) {
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
	toolsets := []string{"k8s"}
	readOnly := true
	disableDestructive := true
	logLevel := "debug"
	err := Run(context.Background(), Options{
		ConfigPath:         configPath,
		Kubeconfig:         kubeconfigPath,
		Context:            "test",
		Toolsets:           toolsets,
		ReadOnly:           readOnly,
		DisableDestructive: disableDestructive,
		LogLevel:           logLevel,
		Stderr:             nil,
		Transport:          fakeTransport{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRunInitError(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("kubeconfig = %q\ntoolsets = [\"missing\"]\n", kubeconfigPath)), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := Run(context.Background(), Options{
		ConfigPath: configPath,
		Version:    "test",
		Stderr:     io.Discard,
		Transport:  fakeTransport{},
	})
	if err == nil {
		t.Fatalf("expected init error")
	}
}

func TestRunReloadSignal(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("kubeconfig = %q\ntoolsets = [\"k8s\"]\n", kubeconfigPath)), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	done := make(chan struct{})
	runErr := make(chan error, 1)
	go func() {
		runErr <- Run(context.Background(), Options{
			ConfigPath: configPath,
			Version:    "test",
			Stderr:     io.Discard,
			Transport:  blockingTransport{done: done},
		})
	}()
	time.Sleep(50 * time.Millisecond)
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	close(done)
	if err := <-runErr; err != nil {
		t.Fatalf("run: %v", err)
	}
}

type errorToolset struct {
	id string
}

func (t errorToolset) ID() string {
	return t.id
}

func (t errorToolset) Version() string {
	return "0.0.0"
}

func (t errorToolset) Init(rcmcp.ToolsetContext) error {
	return fmt.Errorf("init error")
}

func (t errorToolset) Register(rcmcp.Registry) error {
	return nil
}

type registerErrorToolset struct {
	id string
}

func (t registerErrorToolset) ID() string {
	return t.id
}

func (t registerErrorToolset) Version() string {
	return "0.0.0"
}

func (t registerErrorToolset) Init(rcmcp.ToolsetContext) error {
	return nil
}

func (t registerErrorToolset) Register(rcmcp.Registry) error {
	return fmt.Errorf("register error")
}

func TestBuildRuntimeToolsetInitError(t *testing.T) {
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
	id := fmt.Sprintf("test-init-%d", time.Now().UnixNano())
	if err := rcmcp.RegisterToolset(id, func() rcmcp.Toolset { return errorToolset{id: id} }); err != nil {
		t.Fatalf("register toolset: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = kubeconfigPath
	cfg.Toolsets = []string{id}
	_, _, err := buildRuntime(cfg, io.Discard)
	if err == nil {
		t.Fatalf("expected init error")
	}
}

func TestBuildRuntimeToolsetRegisterError(t *testing.T) {
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
	id := fmt.Sprintf("test-register-%d", time.Now().UnixNano())
	if err := rcmcp.RegisterToolset(id, func() rcmcp.Toolset { return registerErrorToolset{id: id} }); err != nil {
		t.Fatalf("register toolset: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = kubeconfigPath
	cfg.Toolsets = []string{id}
	_, _, err := buildRuntime(cfg, io.Discard)
	if err == nil {
		t.Fatalf("expected register error")
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

type errorTransport struct{}

func (errorTransport) Connect(context.Context) (sdkmcp.Connection, error) {
	return nil, fmt.Errorf("connect error")
}

type blockingTransport struct {
	done chan struct{}
}

func (t blockingTransport) Connect(context.Context) (sdkmcp.Connection, error) {
	return &blockingConn{done: t.done}, nil
}

type blockingConn struct {
	done chan struct{}
}

func (c *blockingConn) Read(context.Context) (sdkjsonrpc.Message, error) {
	<-c.done
	return nil, io.EOF
}

func (c *blockingConn) Write(context.Context, sdkjsonrpc.Message) error {
	return nil
}

func (c *blockingConn) Close() error {
	return nil
}

func (c *blockingConn) SessionID() string {
	return "blocking"
}
