package helm

import (
	"context"
	"net/http"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestExpandHome(t *testing.T) {
	got := expandHome("~/.kube/config")
	if got == "" || got == "~/.kube/config" {
		t.Fatalf("expected expanded home path, got %q", got)
	}
	if same := expandHome("/tmp/config"); same != "/tmp/config" {
		t.Fatalf("expected unchanged path, got %q", same)
	}
}

func TestSharedRESTClientGetterErrors(t *testing.T) {
	getter := &sharedRESTClientGetter{}
	if _, err := getter.ToRESTConfig(); err == nil {
		t.Fatalf("expected rest config error")
	}
	if _, err := getter.ToDiscoveryClient(); err == nil {
		t.Fatalf("expected discovery error")
	}
	if _, err := getter.ToRESTMapper(); err == nil {
		t.Fatalf("expected rest mapper error")
	}
}

func TestHelmSettingsFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = "/tmp/kubeconfig"
	cfg.Context = "demo"
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg}}
	settings := toolset.helmSettings()
	if settings.KubeConfig != cfg.Kubeconfig || settings.KubeContext != cfg.Context {
		t.Fatalf("unexpected helm settings: %#v", settings)
	}
}

func TestTemplateApplyInvalidManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}}}
	_, err := toolset.handleTemplateApply(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"manifest":  "not-yaml: [",
			"confirm":   true,
		},
	})
	if err == nil {
		t.Fatalf("expected template apply error")
	}
}

func TestBuildArtifactoryIndexURL(t *testing.T) {
	tests := []struct {
		base string
		repo string
		want string
	}{
		{base: "https://acme.jfrog.io", repo: "helm-local", want: "https://acme.jfrog.io/artifactory/helm-local/index.yaml"},
		{base: "https://acme.jfrog.io/artifactory", repo: "helm-local", want: "https://acme.jfrog.io/artifactory/helm-local/index.yaml"},
		{base: "https://acme.jfrog.io/artifactory/helm-local", repo: "helm-local", want: "https://acme.jfrog.io/artifactory/helm-local/index.yaml"},
	}
	for _, tc := range tests {
		if got := buildArtifactoryIndexURL(tc.base, tc.repo); got != tc.want {
			t.Fatalf("buildArtifactoryIndexURL(%q,%q)=%q, want %q", tc.base, tc.repo, got, tc.want)
		}
	}
}

func TestApplyArtifactoryAuth(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	applyArtifactoryAuth(req, map[string]any{"accessToken": "abc"})
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("expected bearer auth, got %q", got)
	}

	req2, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request2: %v", err)
	}
	applyArtifactoryAuth(req2, map[string]any{"apiKey": "k"})
	if got := req2.Header.Get("X-JFrog-Art-Api"); got != "k" {
		t.Fatalf("expected api key header, got %q", got)
	}

	req3, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request3: %v", err)
	}
	applyArtifactoryAuth(req3, map[string]any{"username": "u", "password": "p"})
	user, pass, ok := req3.BasicAuth()
	if !ok || user != "u" || pass != "p" {
		t.Fatalf("expected basic auth u/p, got ok=%v user=%q pass=%q", ok, user, pass)
	}
}

func TestFlattenIndexAndFilterCharts(t *testing.T) {
	index := repo.NewIndexFile()
	index.Entries = map[string]repo.ChartVersions{
		"nginx": {
			&repo.ChartVersion{Metadata: &chart.Metadata{Name: "nginx", Version: "1.2.0", Description: "web server", Deprecated: false, Keywords: []string{"web"}}, Created: time.Now(), Digest: "a"},
			&repo.ChartVersion{Metadata: &chart.Metadata{Name: "nginx", Version: "1.1.0", Description: "old", Deprecated: true}, Created: time.Now(), Digest: "b"},
		},
	}
	charts := flattenIndex("helm-local", index, false)
	if len(charts) != 1 {
		t.Fatalf("expected deprecated filtered out, got %d", len(charts))
	}
	filtered := filterCharts(charts, "web")
	if len(filtered) != 1 {
		t.Fatalf("expected one chart match, got %d", len(filtered))
	}
}

func TestDiffManifestObjects(t *testing.T) {
	current, err := decodeManifest("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n  namespace: default\n")
	if err != nil {
		t.Fatalf("decode current: %v", err)
	}
	desired, err := decodeManifest("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n  namespace: default\n")
	if err != nil {
		t.Fatalf("decode desired: %v", err)
	}
	added, removed, changed, unchanged := diffManifestObjects(current, desired)
	if len(added) != 1 || len(removed) != 1 || len(changed) != 0 || len(unchanged) != 0 {
		t.Fatalf("unexpected diff result: added=%d removed=%d changed=%d unchanged=%d", len(added), len(removed), len(changed), len(unchanged))
	}
}
