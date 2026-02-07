package helm

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func writeRepoIndex(t *testing.T, dir string) {
	t.Helper()
	index := repo.NewIndexFile()
	index.Add(&chart.Metadata{Name: "demo", Version: "0.1.0"}, "demo-0.1.0.tgz", "file://"+dir, "")
	index.SortEntries()
	if err := index.WriteFile(filepath.Join(dir, "index.yaml"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
}

func TestRepoAddAndUpdateSuccess(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoIndex(t, repoDir)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})

	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	t.Setenv("HELM_REPOSITORY_CACHE", filepath.Join(t.TempDir(), "cache"))
	if err := repo.NewFile().WriteFile(repoFile, 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}

	origNew := newChartRepository
	origDownload := downloadIndexFile
	t.Cleanup(func() {
		newChartRepository = origNew
		downloadIndexFile = origDownload
	})
	newChartRepository = func(entry *repo.Entry, _ getter.Providers) (*repo.ChartRepository, error) {
		return &repo.ChartRepository{Config: entry}, nil
	}
	downloadIndexFile = func(_ *repo.ChartRepository) (string, error) {
		return "index.yaml", nil
	}

	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local", "url": "https://example.local"},
	}); err != nil {
		t.Fatalf("handleRepoAdd: %v", err)
	}
	if _, err := toolset.handleRepoUpdate(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local"},
	}); err != nil {
		t.Fatalf("handleRepoUpdate: %v", err)
	}
}

func TestRepoAddAndUpdateErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "", "url": ""},
	}); err == nil {
		t.Fatalf("expected repo add args error")
	}
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	if _, err := toolset.handleRepoUpdate(context.Background(), mcp.ToolRequest{}); err == nil {
		t.Fatalf("expected repo update error for missing file")
	}
}

func TestRepoListMissingFile(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	origLoad := loadRepoFile
	t.Cleanup(func() { loadRepoFile = origLoad })
	loadRepoFile = func(path string) (*repo.File, error) {
		return &repo.File{}, os.ErrNotExist
	}
	result, err := toolset.handleRepoList(context.Background(), mcp.ToolRequest{})
	if err != nil {
		t.Fatalf("handleRepoList: %v", err)
	}
	repos := result.Data.(map[string]any)["repositories"].([]map[string]any)
	if len(repos) != 0 {
		t.Fatalf("expected empty repos")
	}
}

func TestRepoAddUpdatesExistingEntry(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	if err := repo.NewFile().WriteFile(repoFile, 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	origNew := newChartRepository
	origDownload := downloadIndexFile
	origLoad := loadRepoFile
	t.Cleanup(func() {
		newChartRepository = origNew
		downloadIndexFile = origDownload
		loadRepoFile = origLoad
	})
	newChartRepository = func(entry *repo.Entry, _ getter.Providers) (*repo.ChartRepository, error) {
		return &repo.ChartRepository{Config: entry}, nil
	}
	downloadIndexFile = func(_ *repo.ChartRepository) (string, error) { return "index.yaml", nil }
	loadRepoFile = repo.LoadFile

	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local", "url": "https://example.local"},
	}); err != nil {
		t.Fatalf("handleRepoAdd first: %v", err)
	}
	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local", "url": "https://example.local/updated"},
	}); err != nil {
		t.Fatalf("handleRepoAdd update: %v", err)
	}
}

func TestRepoAddLoadFileError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	origLoad := loadRepoFile
	t.Cleanup(func() { loadRepoFile = origLoad })
	loadRepoFile = func(path string) (*repo.File, error) {
		return &repo.File{}, errors.New("load error")
	}
	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local", "url": "https://example.local"},
	}); err == nil {
		t.Fatalf("expected repo add load error")
	}
}

func TestRepoAddMissingFileUsesNew(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")
	t.Setenv("HELM_REPOSITORY_CONFIG", repoFile)
	t.Setenv("HELM_REPOSITORY_CACHE", filepath.Join(t.TempDir(), "cache"))
	origNew := newChartRepository
	origDownload := downloadIndexFile
	origLoad := loadRepoFile
	t.Cleanup(func() {
		newChartRepository = origNew
		downloadIndexFile = origDownload
		loadRepoFile = origLoad
	})
	newChartRepository = func(entry *repo.Entry, _ getter.Providers) (*repo.ChartRepository, error) {
		return &repo.ChartRepository{Config: entry}, nil
	}
	downloadIndexFile = func(_ *repo.ChartRepository) (string, error) { return "index.yaml", nil }
	loadRepoFile = func(path string) (*repo.File, error) {
		return &repo.File{}, os.ErrNotExist
	}
	if _, err := toolset.handleRepoAdd(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"name": "local", "url": "https://example.local"},
	}); err != nil {
		t.Fatalf("handleRepoAdd: %v", err)
	}
}

func TestRepoListLoadError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	origLoad := loadRepoFile
	t.Cleanup(func() { loadRepoFile = origLoad })
	loadRepoFile = func(path string) (*repo.File, error) {
		return &repo.File{}, errors.New("load error")
	}
	if _, err := toolset.handleRepoList(context.Background(), mcp.ToolRequest{}); err == nil {
		t.Fatalf("expected repo list load error")
	}
}

func TestSharedRESTClientGetterErrorsExtra(t *testing.T) {
	getter := &sharedRESTClientGetter{}
	if _, err := getter.ToRESTConfig(); err == nil {
		t.Fatalf("expected rest config error")
	}
	if _, err := getter.ToDiscoveryClient(); err == nil {
		t.Fatalf("expected discovery error")
	}
	if _, err := getter.ToRESTMapper(); err == nil {
		t.Fatalf("expected mapper error")
	}
	loader := getter.ToRawKubeConfigLoader()
	if _, err := loader.RawConfig(); err != nil {
		t.Fatalf("expected raw config: %v", err)
	}
}

func TestHandleListAndStatusErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleList(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleNamespace},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected namespace required error")
	}
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err == nil {
		t.Fatalf("expected status args error")
	}
}

func TestHandleListFilterLimit(t *testing.T) {
	rel := &release.Release{
		Name:      "demo",
		Namespace: "default",
		Version:   1,
		Info:      &release.Info{Status: release.StatusDeployed},
	}
	toolset := &Toolset{ctx: mcp.ToolsetContext{Policy: policy.NewAuthorizer()}}
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		mem := driver.NewMemory()
		mem.SetNamespace(namespace)
		cfg := &action.Configuration{
			Releases:     storage.Init(mem),
			Log:          func(string, ...interface{}) {},
			Capabilities: nil,
		}
		cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}
		_ = cfg.Releases.Create(rel)
		return cfg, nil
	}
	if _, err := toolset.handleList(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"filter":    "demo",
			"limit":     1,
		},
	}); err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if _, err := toolset.handleList(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"allNamespaces": true},
	}); err != nil {
		t.Fatalf("handleList all namespaces: %v", err)
	}
}

func TestHandleTemplateApplyInvokerError(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	template := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  namespace: {{ .Release.Namespace }}\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	_ = reg.Add(mcp.ToolSpec{
		Name:      "k8s.apply",
		ToolsetID: "k8s",
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{}, errors.New("apply fail")
		},
	})
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{RestConfig: &rest.Config{Host: "https://example.com"}},
		Policy:   policy.NewAuthorizer(),
		Invoker:  mcp.NewToolInvoker(reg, mcp.ToolContext{Config: &cfg}),
		Registry: reg,
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	if _, err := toolset.handleTemplateApply(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err == nil {
		t.Fatalf("expected template apply error")
	}
}

func TestHandleStatusActionConfigError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return nil, errors.New("action config error")
	}
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default", "release": "demo"},
	}); err == nil {
		t.Fatalf("expected action config error")
	}
}

func TestHandleStatusReleaseNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		cfg := &action.Configuration{Releases: storage.Init(driver.NewMemory())}
		cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}
		return cfg, nil
	}
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default", "release": "missing"},
	}); err == nil {
		t.Fatalf("expected status not found error")
	}
}

func TestHandleInstallReadValuesError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleInstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"namespace": "default",
			"release":   "demo",
			"chart":     "missing",
			"valuesYAML": "invalid: [",
		},
	}); err == nil {
		t.Fatalf("expected readValues error")
	}
}

func TestHandleUpgradeLoadChartError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleUpgrade(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"namespace": "default",
			"release":   "demo",
			"chart":     "missing",
		},
	}); err == nil {
		t.Fatalf("expected loadChart error")
	}
}

func TestHandleUninstallReleaseNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		cfg := &action.Configuration{Releases: storage.Init(driver.NewMemory())}
		cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}
		return cfg, nil
	}
	if _, err := toolset.handleUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"namespace": "default",
			"release":   "missing",
		},
	}); err == nil {
		t.Fatalf("expected uninstall not found error")
	}
}

func TestHandleUninstallActionConfigError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return nil, errors.New("action config error")
	}
	if _, err := toolset.handleUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":   true,
			"namespace": "default",
			"release":   "demo",
		},
	}); err == nil {
		t.Fatalf("expected action config error")
	}
}

func TestHandleTemplateUninstallResolveError(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: UnknownKind\nmetadata:\n  name: demo\n  namespace: default\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "unknown.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := config.DefaultConfig()
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		Dynamic:    dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	if _, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err == nil {
		t.Fatalf("expected resolve error")
	}
}

func TestRequireConfirmErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleInstall(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected install confirm error")
	}
	if _, err := toolset.handleUpgrade(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected upgrade confirm error")
	}
	if _, err := toolset.handleUninstall(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected uninstall confirm error")
	}
}

func TestRenderManifestActionConfigError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return nil, errors.New("fail")
	}
	if _, err := toolset.renderManifest("default", "demo", "chart", map[string]any{}); err == nil {
		t.Fatalf("expected render manifest error")
	}
}

func TestLoadChartError(t *testing.T) {
	settings := cli.New()
	if _, err := loadChart(settings, action.ChartPathOptions{}, "missing-chart"); err == nil {
		t.Fatalf("expected load chart error")
	}
}

func TestDecodeManifestError(t *testing.T) {
	if _, err := decodeManifest("::invalid"); err == nil {
		t.Fatalf("expected decode manifest error")
	}
}

func TestSummarizeReleaseNilInfo(t *testing.T) {
	out := summarizeRelease(&release.Release{Name: "demo", Namespace: "default"})
	if out["status"] != "unknown" {
		t.Fatalf("expected unknown status")
	}
	if out["name"] != "demo" {
		t.Fatalf("expected release name")
	}
}

func TestHelmSettingsOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = "/tmp/config"
	cfg.Context = "demo"
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	settings := toolset.helmSettings()
	if settings.KubeConfig != "/tmp/config" || settings.KubeContext != "demo" {
		t.Fatalf("unexpected helm settings: %#v", settings)
	}
}

func TestExpandHomeNoTilde(t *testing.T) {
	if expandHome("/tmp") != "/tmp" {
		t.Fatalf("expected expandHome to be passthrough")
	}
}

func TestHandleTemplateUninstallNamespaceMismatch(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  namespace: other\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := config.DefaultConfig()
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	if _, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err == nil {
		t.Fatalf("expected namespace mismatch error")
	}
}

func TestHandleTemplateUninstallClusterScoped(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: demo-ns\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "ns.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := config.DefaultConfig()
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
			},
		},
	}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(),
		Dynamic:    dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	if _, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err != nil {
		t.Fatalf("handleTemplateUninstall: %v", err)
	}
}

func TestHandleTemplateUninstallSkippedAndNotFound(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  labels:\n    app: demo\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: missing\n  namespace: default\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := config.DefaultConfig()
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		Dynamic:    dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	result, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	})
	if err != nil {
		t.Fatalf("handleTemplateUninstall: %v", err)
	}
	if result.Data.(map[string]any)["skipped"] == nil {
		t.Fatalf("expected skipped entries")
	}
}

func TestHandleTemplateUninstallDefaultNamespace(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart yaml: %v", err)
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := config.DefaultConfig()
	discoveryClient := &helmDiscovery{resources: []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	getter := &staticRESTClientGetter{
		restConfig: &rest.Config{Host: "https://example.com"},
		mapper:     mapper,
		discovery:  discoveryClient,
	}
	actionCfg := new(action.Configuration)
	if err := actionCfg.Init(getter, "default", "memory", func(string, ...interface{}) {}); err != nil {
		t.Fatalf("init action config: %v", err)
	}
	actionCfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
		},
	}}
	clients := &kube.Clients{
		Typed:      k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		Dynamic:    dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj),
		Discovery:  discoveryClient,
		Mapper:     mapper,
		RestConfig: &rest.Config{Host: "https://example.com"},
	}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	toolset.actionConfigOverride = func(namespace string) (*action.Configuration, error) {
		return actionCfg, nil
	}
	if _, err := toolset.handleTemplateUninstall(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"chart":     chartDir,
			"confirm":   true,
		},
	}); err != nil {
		t.Fatalf("handleTemplateUninstall: %v", err)
	}
}

func TestHandleInstallAndUpgradeMissingArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleInstall(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"confirm": true},
	}); err == nil {
		t.Fatalf("expected missing args error")
	}
	if _, err := toolset.handleUpgrade(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"confirm": true},
	}); err == nil {
		t.Fatalf("expected missing args error")
	}
	if _, err := toolset.handleUninstall(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"confirm": true},
	}); err == nil {
		t.Fatalf("expected missing args error")
	}
}
