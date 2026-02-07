package k8s

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type fakeUpgrader struct{}

func (fakeUpgrader) NewConnection(*http.Response) (httpstream.Connection, error) {
	return nil, nil
}

type fakePortForwarder struct {
	ready chan struct{}
	ports []portforward.ForwardedPort
}

func (f *fakePortForwarder) ForwardPorts() error {
	close(f.ready)
	return nil
}

func (f *fakePortForwarder) GetPorts() ([]portforward.ForwardedPort, error) {
	return f.ports, nil
}

type fakeExecutor struct{}

func (fakeExecutor) Stream(opts remotecommand.StreamOptions) error {
	return fakeExecutor{}.StreamWithContext(context.Background(), opts)
}

func (fakeExecutor) StreamWithContext(ctx context.Context, opts remotecommand.StreamOptions) error {
	if opts.Stdout != nil {
		_, _ = io.WriteString(opts.Stdout, "ok")
	}
	if opts.Stderr != nil {
		_, _ = io.WriteString(opts.Stderr, "warn")
	}
	return nil
}

func TestHandlePortForwardSuccess(t *testing.T) {
	origRoundTripper := spdyRoundTripperFor
	origForwarder := newPortForwarder
	defer func() {
		spdyRoundTripperFor = origRoundTripper
		newPortForwarder = origForwarder
	}()
	spdyRoundTripperFor = func(*rest.Config) (http.RoundTripper, spdy.Upgrader, error) {
		return http.DefaultTransport, fakeUpgrader{}, nil
	}
	newPortForwarder = func(dialer httpstream.Dialer, addresses []string, ports []string, stopChan, readyChan chan struct{}, out, errOut io.Writer) (portForwarder, error) {
		return &fakePortForwarder{
			ready: readyChan,
			ports: []portforward.ForwardedPort{{Local: 10443, Remote: 443}},
		}, nil
	}

	clientset, restCfg := newRestClientset(t)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: clientset, RestConfig: restCfg},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	result, err := toolset.handlePortForward(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
			"ports":     []any{"10443:443"},
		},
	})
	if err != nil {
		t.Fatalf("handlePortForward success: %v", err)
	}
	if data, ok := result.Data.(map[string]any); !ok || data["pod"] != "api" {
		t.Fatalf("unexpected port forward result: %#v", result.Data)
	}
}

func TestHandleExecSuccess(t *testing.T) {
	origExecutor := newSPDYExecutor
	defer func() {
		newSPDYExecutor = origExecutor
	}()
	newSPDYExecutor = func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
		return fakeExecutor{}, nil
	}

	clientset, restCfg := newRestClientset(t)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: clientset, RestConfig: restCfg},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	result, err := toolset.handleExec(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
			"command":   []any{"ls"},
		},
	})
	if err != nil {
		t.Fatalf("handleExec success: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["stdout"] != "ok" {
		t.Fatalf("unexpected exec output: %#v", result.Data)
	}
}
