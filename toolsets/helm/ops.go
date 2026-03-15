package helm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	sigsyaml "sigs.k8s.io/yaml"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

const helmDriver = "secrets"

const defaultArtifactHubURL = "https://artifacthub.io"

var newChartRepository = repo.NewChartRepository
var downloadIndexFile = func(chartRepo *repo.ChartRepository) (string, error) {
	return chartRepo.DownloadIndexFile()
}
var loadRepoFile = repo.LoadFile

type sharedRESTClientGetter struct {
	restConfig *rest.Config
	mapper     meta.RESTMapper
	discovery  discovery.DiscoveryInterface
	kubeconfig string
	context    string
	namespace  string
}

func (g *sharedRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	if g.restConfig == nil {
		return nil, errors.New("missing rest config")
	}
	return g.restConfig, nil
}

func (g *sharedRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	if g.discovery == nil {
		return nil, errors.New("missing discovery client")
	}
	return memory.NewMemCacheClient(g.discovery), nil
}

func (g *sharedRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	if g.mapper == nil {
		return nil, errors.New("missing rest mapper")
	}
	return g.mapper, nil
}

func (g *sharedRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if g.kubeconfig != "" {
		rules.ExplicitPath = expandHome(g.kubeconfig)
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: g.context}
	if g.namespace != "" {
		overrides.Context.Namespace = g.namespace
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}

func expandHome(path string) string {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path
	}
	home := homedir.HomeDir()
	if home == "" {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func (t *Toolset) helmSettings() *cli.EnvSettings {
	settings := cli.New()
	if t.ctx.Config != nil {
		if t.ctx.Config.Kubeconfig != "" {
			settings.KubeConfig = t.ctx.Config.Kubeconfig
		}
		if t.ctx.Config.Context != "" {
			settings.KubeContext = t.ctx.Config.Context
		}
	}
	return settings
}

func (t *Toolset) actionConfig(namespace string) (*action.Configuration, error) {
	if t.actionConfigOverride != nil {
		return t.actionConfigOverride(namespace)
	}
	getter := &sharedRESTClientGetter{
		restConfig: t.ctx.Clients.RestConfig,
		mapper:     t.ctx.Clients.Mapper,
		discovery:  t.ctx.Clients.Discovery,
		kubeconfig: t.ctx.Config.Kubeconfig,
		context:    t.ctx.Config.Context,
		namespace:  namespace,
	}
	cfg := new(action.Configuration)
	if err := cfg.Init(getter, namespace, helmDriver, func(string, ...interface{}) {}); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *Toolset) handleRepoAdd(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	name := toString(args["name"])
	url := toString(args["url"])
	if name == "" || url == "" {
		return errorResult(errors.New("name and url are required")), errors.New("name and url are required")
	}
	settings := t.helmSettings()
	repoFile := settings.RepositoryConfig
	if err := os.MkdirAll(filepath.Dir(repoFile), 0o755); err != nil {
		return errorResult(err), err
	}
	file, err := loadRepoFile(repoFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return errorResult(err), err
		}
		file = repo.NewFile()
	}
	entry := &repo.Entry{
		Name:                  name,
		URL:                   url,
		Username:              toString(args["username"]),
		Password:              toString(args["password"]),
		CAFile:                toString(args["caFile"]),
		CertFile:              toString(args["certFile"]),
		KeyFile:               toString(args["keyFile"]),
		InsecureSkipTLSverify: toBool(args["insecureSkipTLSVerify"]),
		PassCredentialsAll:    toBool(args["passCredentialsAll"]),
	}
	if file.Has(name) {
		file.Update(entry)
	} else {
		file.Add(entry)
	}
	chartRepo, err := newChartRepository(entry, getter.All(settings))
	if err != nil {
		return errorResult(err), err
	}
	if _, err := downloadIndexFile(chartRepo); err != nil {
		return errorResult(err), err
	}
	if err := file.WriteFile(repoFile, 0o644); err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"added": name, "url": url}}, nil
}

func (t *Toolset) handleRepoList(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	settings := t.helmSettings()
	file, err := loadRepoFile(settings.RepositoryConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.ToolResult{Data: map[string]any{"repositories": []map[string]any{}}}, nil
		}
		return errorResult(err), err
	}
	repos := []map[string]any{}
	for _, entry := range file.Repositories {
		repos = append(repos, map[string]any{"name": entry.Name, "url": entry.URL})
	}
	sort.Slice(repos, func(i, j int) bool {
		return toString(repos[i]["name"]) < toString(repos[j]["name"])
	})
	return mcp.ToolResult{Data: map[string]any{"repositories": repos}}, nil
}

func (t *Toolset) handleRepoUpdate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	settings := t.helmSettings()
	name := toString(req.Arguments["name"])
	file, err := loadRepoFile(settings.RepositoryConfig)
	if err != nil {
		return errorResult(err), err
	}
	var updated []string
	for _, entry := range file.Repositories {
		if name != "" && entry.Name != name {
			continue
		}
		chartRepo, err := newChartRepository(entry, getter.All(settings))
		if err != nil {
			return errorResult(err), err
		}
		if _, err := downloadIndexFile(chartRepo); err != nil {
			return errorResult(err), err
		}
		updated = append(updated, entry.Name)
	}
	if len(updated) == 0 && name != "" {
		return errorResult(errors.New("repository not found")), errors.New("repository not found")
	}
	sort.Strings(updated)
	return mcp.ToolResult{Data: map[string]any{"updated": updated}}, nil
}

func (t *Toolset) handleListCharts(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	baseURL := artifactHubURL(args)
	repoName := toString(args["repo"])
	limit := toInt(args["limit"])
	if limit <= 0 {
		limit = 20
	}
	offset := toInt(args["offset"])
	query := url.Values{}
	query.Set("kind", "0")
	query.Set("facets", "false")
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("offset", fmt.Sprintf("%d", max(0, offset)))
	query.Set("sort", "last_updated")
	if repoName != "" {
		query.Set("repo", repoName)
	}
	if deprecated, ok := args["includeDeprecated"].(bool); ok {
		query.Set("deprecated", fmt.Sprintf("%t", deprecated))
	}
	payload, err := t.fetchArtifactHubJSON(ctx, baseURL, "/api/v1/packages/search", query)
	if err != nil {
		return errorResult(err), err
	}
	if out, ok := payload.(map[string]any); ok {
		out["artifactHubURL"] = baseURL
		if repoName != "" {
			out["repo"] = repoName
		}
		return mcp.ToolResult{Data: out}, nil
	}
	return mcp.ToolResult{Data: payload}, nil
}

func (t *Toolset) handleSearchCharts(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	baseURL := artifactHubURL(args)
	repoName := toString(args["repo"])
	query := strings.TrimSpace(toString(args["query"]))
	if query == "" {
		err := errors.New("query is required")
		return errorResult(err), err
	}
	limit := toInt(args["limit"])
	if limit <= 0 {
		limit = 20
	}
	offset := toInt(args["offset"])
	params := url.Values{}
	params.Set("kind", "0")
	params.Set("facets", "false")
	params.Set("ts_query_web", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", max(0, offset)))
	params.Set("sort", "relevance")
	if repoName != "" {
		params.Set("repo", repoName)
	}
	if deprecated, ok := args["includeDeprecated"].(bool); ok {
		params.Set("deprecated", fmt.Sprintf("%t", deprecated))
	}
	payload, err := t.fetchArtifactHubJSON(ctx, baseURL, "/api/v1/packages/search", params)
	if err != nil {
		return errorResult(err), err
	}
	if out, ok := payload.(map[string]any); ok {
		out["artifactHubURL"] = baseURL
		out["query"] = query
		if repoName != "" {
			out["repo"] = repoName
		}
		return mcp.ToolResult{Data: out}, nil
	}
	return mcp.ToolResult{Data: payload}, nil
}

func (t *Toolset) handleGetChart(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	baseURL := artifactHubURL(args)
	repoName := toString(args["repo"])
	chartName := toString(args["chart"])
	version := strings.TrimSpace(toString(args["version"]))
	if repoName == "" || chartName == "" {
		err := errors.New("repo and chart are required")
		return errorResult(err), err
	}
	path := fmt.Sprintf("/api/v1/packages/helm/%s/%s", url.PathEscape(repoName), url.PathEscape(chartName))
	if version != "" {
		path += "/" + url.PathEscape(version)
	}
	payload, err := t.fetchArtifactHubJSON(ctx, baseURL, path, nil)
	if err != nil {
		return errorResult(err), err
	}
	if out, ok := payload.(map[string]any); ok {
		out["artifactHubURL"] = baseURL
		out["repo"] = repoName
		out["chartName"] = chartName
		if version != "" {
			out["requestedVersion"] = version
		}
		return mcp.ToolResult{Data: out}, nil
	}
	return mcp.ToolResult{Data: payload}, nil
}

func artifactHubURL(args map[string]any) string {
	if override := strings.TrimSpace(toString(args["artifactHubURL"])); override != "" {
		return strings.TrimRight(override, "/")
	}
	return defaultArtifactHubURL
}

func (t *Toolset) fetchArtifactHubJSON(ctx context.Context, baseURL, endpointPath string, query url.Values) (any, error) {
	endpoint := strings.TrimRight(baseURL, "/") + endpointPath
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("artifact hub request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (t *Toolset) httpClient() *http.Client {
	timeout := 60 * time.Second
	if t.ctx.Config != nil && t.ctx.Config.Timeouts.DefaultSeconds > 0 {
		timeout = time.Duration(t.ctx.Config.Timeouts.DefaultSeconds) * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func (t *Toolset) loadChartsCatalog(repoName string, includeDeprecated bool) ([]map[string]any, error) {
	settings := t.helmSettings()
	file, err := loadRepoFile(settings.RepositoryConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	charts := make([]map[string]any, 0)
	for _, entry := range file.Repositories {
		if repoName != "" && entry.Name != repoName {
			continue
		}
		indexPath := chartRepoIndexPath(settings.RepositoryCache, entry.Name)
		index, err := repo.LoadIndexFile(indexPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for name, versions := range index.Entries {
			for _, chartVersion := range versions {
				if chartVersion == nil || chartVersion.Metadata == nil {
					continue
				}
				if chartVersion.Metadata.Deprecated && !includeDeprecated {
					continue
				}
				charts = append(charts, map[string]any{
					"repo":        entry.Name,
					"repoURL":     entry.URL,
					"name":        name,
					"version":     chartVersion.Metadata.Version,
					"appVersion":  chartVersion.Metadata.AppVersion,
					"description": chartVersion.Metadata.Description,
					"home":        chartVersion.Metadata.Home,
					"keywords":    chartVersion.Metadata.Keywords,
					"sources":     chartVersion.Metadata.Sources,
					"deprecated":  chartVersion.Metadata.Deprecated,
					"created":     chartVersion.Created,
					"digest":      chartVersion.Digest,
					"urls":        chartVersion.URLs,
				})
			}
		}
	}
	sort.Slice(charts, func(i, j int) bool {
		leftRepo := toString(charts[i]["repo"])
		rightRepo := toString(charts[j]["repo"])
		if leftRepo != rightRepo {
			return leftRepo < rightRepo
		}
		leftName := toString(charts[i]["name"])
		rightName := toString(charts[j]["name"])
		if leftName != rightName {
			return leftName < rightName
		}
		return toString(charts[i]["version"]) > toString(charts[j]["version"])
	})
	return charts, nil
}

func flattenIndex(repoName string, index *repo.IndexFile, includeDeprecated bool) []map[string]any {
	if index == nil {
		return []map[string]any{}
	}
	charts := make([]map[string]any, 0)
	for name, versions := range index.Entries {
		for _, chartVersion := range versions {
			if chartVersion == nil || chartVersion.Metadata == nil {
				continue
			}
			if chartVersion.Metadata.Deprecated && !includeDeprecated {
				continue
			}
			charts = append(charts, map[string]any{
				"repo":        repoName,
				"name":        name,
				"version":     chartVersion.Metadata.Version,
				"appVersion":  chartVersion.Metadata.AppVersion,
				"description": chartVersion.Metadata.Description,
				"home":        chartVersion.Metadata.Home,
				"keywords":    chartVersion.Metadata.Keywords,
				"sources":     chartVersion.Metadata.Sources,
				"deprecated":  chartVersion.Metadata.Deprecated,
				"created":     chartVersion.Created,
				"digest":      chartVersion.Digest,
				"urls":        chartVersion.URLs,
			})
		}
	}
	sort.Slice(charts, func(i, j int) bool {
		leftName := toString(charts[i]["name"])
		rightName := toString(charts[j]["name"])
		if leftName != rightName {
			return leftName < rightName
		}
		return toString(charts[i]["version"]) > toString(charts[j]["version"])
	})
	return charts
}

func filterCharts(charts []map[string]any, query string) []map[string]any {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return charts
	}
	out := make([]map[string]any, 0)
	for _, chart := range charts {
		if containsAny(normalized,
			toString(chart["repo"]),
			toString(chart["name"]),
			toString(chart["description"]),
			strings.Join(toStringSlice(chart["keywords"]), " "),
		) {
			out = append(out, chart)
		}
	}
	return out
}

func paginateCharts(charts []map[string]any, offset, limit int) ([]map[string]any, int) {
	total := len(charts)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []map[string]any{}, total
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return charts[offset:end], total
}

func containsAny(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func buildArtifactoryIndexURL(baseURL, repoName string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	repo := strings.Trim(strings.TrimSpace(repoName), "/")
	if strings.HasSuffix(base, "/artifactory") {
		return base + "/" + repo + "/index.yaml"
	}
	if strings.Contains(base, "/artifactory/") {
		return base + "/index.yaml"
	}
	return base + "/artifactory/" + repo + "/index.yaml"
}

func applyArtifactoryAuth(req *http.Request, args map[string]any) {
	if req == nil {
		return
	}
	if token := strings.TrimSpace(toString(args["accessToken"])); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if apiKey := strings.TrimSpace(toString(args["apiKey"])); apiKey != "" {
		req.Header.Set("X-JFrog-Art-Api", apiKey)
		return
	}
	username := strings.TrimSpace(toString(args["username"]))
	password := toString(args["password"])
	if username != "" {
		req.SetBasicAuth(username, password)
	}
}

func limitOrDefault(limit int) int {
	if limit > 0 {
		return limit
	}
	return 50
}

func chartRepoIndexPath(repoCacheDir, repoName string) string {
	if repoCacheDir != "" {
		sanitized := strings.ReplaceAll(repoName, "/", "-")
		return filepath.Join(repoCacheDir, sanitized+"-index.yaml")
	}
	return repoName + "-index.yaml"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (t *Toolset) handleList(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	allNamespaces := toBool(args["allNamespaces"])
	filter := toString(args["filter"])
	limit := toInt(args["limit"])

	if allNamespaces {
		if req.User.Role != policy.RoleCluster {
			return errorResult(errors.New("allNamespaces requires cluster role")), errors.New("allNamespaces requires cluster role")
		}
	}
	if namespace == "" {
		if req.User.Role == policy.RoleNamespace && !allNamespaces {
			return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
		}
		namespace = "default"
	}
	if !allNamespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return errorResult(err), err
		}
	}

	actionNamespace := namespace
	if allNamespaces {
		actionNamespace = ""
	}
	cfg, err := t.actionConfig(actionNamespace)
	if err != nil {
		return errorResult(err), err
	}
	list := action.NewList(cfg)
	list.All = true
	list.AllNamespaces = allNamespaces
	list.SetStateMask()
	if filter != "" {
		list.Filter = filter
	}
	if limit > 0 {
		list.Limit = limit
	}
	releases, err := list.Run()
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"releases": summarizeReleases(releases)}}, nil
}

func (t *Toolset) handleStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	if releaseName == "" || namespace == "" {
		return errorResult(errors.New("release and namespace are required")), errors.New("release and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	status := action.NewStatus(cfg)
	rel, err := status.Run(releaseName)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: summarizeRelease(rel)}, nil
}

func (t *Toolset) handleDiffRelease(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	if releaseName == "" || namespace == "" {
		err := errors.New("release and namespace are required")
		return errorResult(err), err
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	get := action.NewGet(cfg)
	currentRelease, err := get.Run(releaseName)
	if err != nil {
		return errorResult(err), err
	}
	currentManifest := currentRelease.Manifest
	desiredManifest := currentManifest
	targetChart := strings.TrimSpace(toString(args["chart"]))
	if targetChart != "" {
		desiredManifest, err = t.renderManifest(namespace, releaseName, targetChart, args)
		if err != nil {
			return errorResult(err), err
		}
	}
	currentObjects, err := decodeManifest(currentManifest)
	if err != nil {
		return errorResult(err), err
	}
	desiredObjects, err := decodeManifest(desiredManifest)
	if err != nil {
		return errorResult(err), err
	}
	added, removed, changed, unchanged := diffManifestObjects(currentObjects, desiredObjects)
	result := map[string]any{
		"release":   releaseName,
		"namespace": namespace,
		"summary": map[string]any{
			"added":     len(added),
			"removed":   len(removed),
			"changed":   len(changed),
			"unchanged": len(unchanged),
		},
		"added":   added,
		"removed": removed,
		"changed": changed,
	}
	if toBool(args["includeUnchanged"]) {
		result["unchanged"] = unchanged
	}
	if targetChart != "" {
		result["targetChart"] = targetChart
	}
	return mcp.ToolResult{Data: result, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleRollbackAdvisor(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	if releaseName == "" || namespace == "" {
		err := errors.New("release and namespace are required")
		return errorResult(err), err
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	status := action.NewStatus(cfg)
	current, err := status.Run(releaseName)
	if err != nil {
		return errorResult(err), err
	}
	history := action.NewHistory(cfg)
	history.Max = toInt(args["historyLimit"])
	if history.Max <= 0 {
		history.Max = 20
	}
	revisions, err := history.Run(releaseName)
	if err != nil {
		return errorResult(err), err
	}
	recommendations := make([]map[string]any, 0)
	for _, rel := range revisions {
		if rel == nil || rel.Version >= current.Version {
			continue
		}
		if rel.Info == nil {
			continue
		}
		if rel.Info.Status == release.StatusDeployed || rel.Info.Status == release.StatusSuperseded {
			severity := "low"
			if rel.Info.Status == release.StatusSuperseded {
				severity = "medium"
			}
			recommendations = append(recommendations, map[string]any{
				"version":     rel.Version,
				"status":      rel.Info.Status.String(),
				"description": fmt.Sprintf("Rollback candidate revision %d (%s)", rel.Version, rel.Info.Status.String()),
				"risk":        severity,
			})
		}
	}
	sort.Slice(recommendations, func(i, j int) bool {
		return toInt(recommendations[i]["version"]) > toInt(recommendations[j]["version"])
	})
	if len(recommendations) > 5 {
		recommendations = recommendations[:5]
	}
	next := []string{"Validate selected revision with helm.diff_release before rollback.", "Run smoke checks after rollback to confirm service recovery."}
	return mcp.ToolResult{Data: map[string]any{
		"release":             releaseName,
		"namespace":           namespace,
		"currentRevision":     current.Version,
		"currentStatus":       current.Info.Status.String(),
		"recommendations":     recommendations,
		"recommendationCount": len(recommendations),
		"nextChecks":          next,
	}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleInstall(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	chartRef := toString(args["chart"])
	if namespace == "" || releaseName == "" || chartRef == "" {
		return errorResult(errors.New("release, chart, and namespace are required")), errors.New("release, chart, and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	settings := t.helmSettings()
	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = toBool(args["createNamespace"])
	install.Wait = toBool(args["wait"])
	install.Atomic = toBool(args["atomic"])
	install.IncludeCRDs = toBool(args["includeCRDs"])
	if timeout := toInt(args["timeoutSeconds"]); timeout > 0 {
		install.Timeout = time.Duration(timeout) * time.Second
	}
	applyChartPathOptions(&install.ChartPathOptions, args)
	values, err := readValues(args)
	if err != nil {
		return errorResult(err), err
	}
	chart, err := loadChart(settings, install.ChartPathOptions, chartRef)
	if err != nil {
		return errorResult(err), err
	}
	rel, err := install.Run(chart, values)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: summarizeRelease(rel), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleUpgrade(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	chartRef := toString(args["chart"])
	if namespace == "" || releaseName == "" || chartRef == "" {
		return errorResult(errors.New("release, chart, and namespace are required")), errors.New("release, chart, and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	settings := t.helmSettings()
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Install = toBool(args["install"])
	upgrade.Wait = toBool(args["wait"])
	upgrade.Atomic = toBool(args["atomic"])
	if timeout := toInt(args["timeoutSeconds"]); timeout > 0 {
		upgrade.Timeout = time.Duration(timeout) * time.Second
	}
	applyChartPathOptions(&upgrade.ChartPathOptions, args)
	values, err := readValues(args)
	if err != nil {
		return errorResult(err), err
	}
	chart, err := loadChart(settings, upgrade.ChartPathOptions, chartRef)
	if err != nil {
		return errorResult(err), err
	}
	rel, err := upgrade.Run(releaseName, chart, values)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: summarizeRelease(rel), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleUninstall(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	if namespace == "" || releaseName == "" {
		return errorResult(errors.New("release and namespace are required")), errors.New("release and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return errorResult(err), err
	}
	uninstall := action.NewUninstall(cfg)
	uninstall.KeepHistory = toBool(args["keepHistory"])
	if timeout := toInt(args["timeoutSeconds"]); timeout > 0 {
		uninstall.Timeout = time.Duration(timeout) * time.Second
	}
	resp, err := uninstall.Run(releaseName)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"uninstalled": releaseName, "info": resp.Info}}, nil
}

func (t *Toolset) handleTemplateApply(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	chartRef := toString(args["chart"])
	if namespace == "" || releaseName == "" || chartRef == "" {
		return errorResult(errors.New("release, chart, and namespace are required")), errors.New("release, chart, and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	manifest, err := t.renderManifest(namespace, releaseName, chartRef, args)
	if err != nil {
		return errorResult(err), err
	}
	applyArgs := map[string]any{
		"manifest":     manifest,
		"namespace":    namespace,
		"fieldManager": toString(args["fieldManager"]),
		"force":        toBool(args["force"]),
		"confirm":      true,
	}
	result, err := t.ctx.Invoker.Call(ctx, req.User, "k8s.apply", applyArgs)
	if err != nil {
		return errorResult(err), err
	}
	applied := result.Data
	if dataMap, ok := result.Data.(map[string]any); ok {
		if val, ok := dataMap["applied"]; ok {
			applied = val
		}
	}
	return mcp.ToolResult{Data: map[string]any{"release": releaseName, "applied": applied}, Metadata: result.Metadata}, nil
}

func (t *Toolset) handleTemplateUninstall(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	namespace := toString(args["namespace"])
	releaseName := toString(args["release"])
	chartRef := toString(args["chart"])
	if namespace == "" || releaseName == "" || chartRef == "" {
		return errorResult(errors.New("release, chart, and namespace are required")), errors.New("release, chart, and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	manifest, err := t.renderManifest(namespace, releaseName, chartRef, args)
	if err != nil {
		return errorResult(err), err
	}
	objects, err := decodeManifest(manifest)
	if err != nil {
		return errorResult(err), err
	}
	deleted := []string{}
	skipped := []string{}
	for _, obj := range objects {
		apiVersion := obj.GetAPIVersion()
		kind := obj.GetKind()
		if apiVersion == "" || kind == "" {
			skipped = append(skipped, fmt.Sprintf("missing apiVersion/kind for %s", obj.GetName()))
			continue
		}
		name := obj.GetName()
		if name == "" {
			skipped = append(skipped, fmt.Sprintf("missing name for %s", kind))
			continue
		}
		gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, "")
		if err != nil {
			return errorResult(err), err
		}
		objNamespace := obj.GetNamespace()
		if namespaced {
			if objNamespace == "" {
				objNamespace = namespace
				obj.SetNamespace(namespace)
			}
			if objNamespace == "" {
				return errorResult(errors.New("namespace required in manifest or input")), errors.New("namespace required in manifest or input")
			}
			if objNamespace != namespace {
				return errorResult(errors.New("manifest namespace does not match input")), errors.New("manifest namespace does not match input")
			}
		}
		if err := t.ctx.Policy.CheckNamespace(req.User, objNamespace, namespaced); err != nil {
			return errorResult(err), err
		}
		if namespaced {
			err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(objNamespace).Delete(ctx, name, metav1.DeleteOptions{})
		} else {
			err = t.ctx.Clients.Dynamic.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
		}
		if err != nil {
			if apierrors.IsNotFound(err) {
				skipped = append(skipped, fmt.Sprintf("%s/%s not found", kind, name))
				continue
			}
			return errorResult(err), err
		}
		deleted = append(deleted, t.ctx.Evidence.ResourceRef(gvr, objNamespace, name))
	}
	sort.Strings(deleted)
	sort.Strings(skipped)
	out := map[string]any{"release": releaseName, "deleted": deleted}
	if len(skipped) > 0 {
		out["skipped"] = skipped
	}
	return mcp.ToolResult{Data: out, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: deleted}}, nil
}

func (t *Toolset) renderManifest(namespace, releaseName, chartRef string, args map[string]any) (string, error) {
	cfg, err := t.actionConfig(namespace)
	if err != nil {
		return "", err
	}
	settings := t.helmSettings()
	install := action.NewInstall(cfg)
	install.DryRun = true
	install.ClientOnly = true
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.IncludeCRDs = toBool(args["includeCRDs"])
	applyChartPathOptions(&install.ChartPathOptions, args)
	values, err := readValues(args)
	if err != nil {
		return "", err
	}
	chart, err := loadChart(settings, install.ChartPathOptions, chartRef)
	if err != nil {
		return "", err
	}
	rel, err := install.Run(chart, values)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

func loadChart(settings *cli.EnvSettings, opts action.ChartPathOptions, chartRef string) (*chart.Chart, error) {
	chartPath, err := opts.LocateChart(chartRef, settings)
	if err != nil {
		return nil, err
	}
	return loader.Load(chartPath)
}

func applyChartPathOptions(opts *action.ChartPathOptions, args map[string]any) {
	opts.Version = toString(args["version"])
	opts.RepoURL = toString(args["repoURL"])
	opts.Username = toString(args["username"])
	opts.Password = toString(args["password"])
	opts.CertFile = toString(args["certFile"])
	opts.KeyFile = toString(args["keyFile"])
	opts.CaFile = toString(args["caFile"])
	opts.InsecureSkipTLSverify = toBool(args["insecureSkipTLSVerify"])
	opts.PassCredentialsAll = toBool(args["passCredentialsAll"])
}

func readValues(args map[string]any) (map[string]any, error) {
	merged := map[string]any{}
	if raw, ok := args["values"].(map[string]any); ok {
		merged = mergeMaps(merged, raw)
	}
	if raw := toString(args["valuesYAML"]); raw != "" {
		values := map[string]any{}
		if err := sigsyaml.Unmarshal([]byte(raw), &values); err != nil {
			return nil, err
		}
		merged = mergeMaps(merged, values)
	}
	for _, file := range toStringSlice(args["valuesFiles"]) {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		values := map[string]any{}
		if err := sigsyaml.Unmarshal(data, &values); err != nil {
			return nil, err
		}
		merged = mergeMaps(merged, values)
	}
	return merged, nil
}

func mergeMaps(dst map[string]any, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for key, value := range src {
		if existing, ok := dst[key]; ok {
			existingMap, okExisting := existing.(map[string]any)
			valueMap, okValue := value.(map[string]any)
			if okExisting && okValue {
				dst[key] = mergeMaps(existingMap, valueMap)
				continue
			}
		}
		dst[key] = value
	}
	return dst
}

func decodeManifest(manifest string) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	var out []*unstructured.Unstructured
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		out = append(out, &unstructured.Unstructured{Object: raw})
	}
	return out, nil
}

func diffManifestObjects(current, desired []*unstructured.Unstructured) ([]string, []string, []map[string]any, []string) {
	currentIndex := indexManifestObjects(current)
	desiredIndex := indexManifestObjects(desired)
	added := make([]string, 0)
	removed := make([]string, 0)
	changed := make([]map[string]any, 0)
	unchanged := make([]string, 0)

	for key, target := range desiredIndex {
		live, ok := currentIndex[key]
		if !ok {
			added = append(added, key)
			continue
		}
		if live != target {
			changed = append(changed, map[string]any{"resource": key, "current": live, "desired": target})
		} else {
			unchanged = append(unchanged, key)
		}
	}
	for key := range currentIndex {
		if _, ok := desiredIndex[key]; !ok {
			removed = append(removed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(unchanged)
	sort.Slice(changed, func(i, j int) bool {
		return toString(changed[i]["resource"]) < toString(changed[j]["resource"])
	})
	return added, removed, changed, unchanged
}

func indexManifestObjects(items []*unstructured.Unstructured) map[string]string {
	index := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		key := manifestObjectKey(item)
		if key == "" {
			continue
		}
		buf, err := sigsyaml.Marshal(item.Object)
		if err != nil {
			continue
		}
		index[key] = strings.TrimSpace(string(buf))
	}
	return index
}

func manifestObjectKey(item *unstructured.Unstructured) string {
	if item == nil {
		return ""
	}
	kind := item.GetKind()
	apiVersion := item.GetAPIVersion()
	name := item.GetName()
	namespace := item.GetNamespace()
	if kind == "" || name == "" {
		return ""
	}
	if namespace == "" {
		return fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
	}
	return fmt.Sprintf("%s/%s/%s/%s", apiVersion, kind, namespace, name)
}

func summarizeRelease(rel *release.Release) map[string]any {
	if rel == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"name":      rel.Name,
		"namespace": rel.Namespace,
		"revision":  rel.Version,
	}
	if rel.Info != nil {
		out["status"] = rel.Info.Status.String()
	} else {
		out["status"] = "unknown"
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		out["chart"] = fmt.Sprintf("%s-%s", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
		out["appVersion"] = rel.Chart.Metadata.AppVersion
	}
	if rel.Info != nil {
		out["updated"] = rel.Info.LastDeployed.Time
		if rel.Info.Notes != "" {
			out["notes"] = rel.Info.Notes
		}
	}
	return out
}

func summarizeReleases(releases []*release.Release) []map[string]any {
	out := make([]map[string]any, 0, len(releases))
	for _, rel := range releases {
		out = append(out, summarizeRelease(rel))
	}
	return out
}

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}
}

func requireConfirm(args map[string]any) error {
	if val, ok := args["confirm"].(bool); ok && val {
		return nil
	}
	return errors.New("confirmation required: set confirm=true to proceed")
}

func toString(val any) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func toStringSlice(val any) []string {
	if val == nil {
		return nil
	}
	if list, ok := val.([]string); ok {
		return list
	}
	if list, ok := val.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			out = append(out, toString(item))
		}
		return out
	}
	if s, ok := val.(string); ok {
		return []string{s}
	}
	return nil
}

func toBool(val any) bool {
	if v, ok := val.(bool); ok {
		return v
	}
	return false
}

func toInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
