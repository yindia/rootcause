package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"rootcause/internal/mcp"
)

const (
	defaultRegistryURL = "https://registry.terraform.io"
	providersV2Path    = "/v2/providers"
	providerDocsV2Path = "/v2/provider-docs"
)

type providerDetailResponse struct {
	Data     providerData          `json:"data"`
	Included []providerVersionData `json:"included"`
}

type providerData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes providerAttributes `json:"attributes"`
}

type providerAttributes struct {
	FullName  string `json:"full-name"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Tier      string `json:"tier"`
}

type providerVersionData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes providerVersionAttributes `json:"attributes"`
}

type providerVersionAttributes struct {
	Version string `json:"version"`
}

type providerDocsResponse struct {
	Data []providerDocData `json:"data"`
}

type providerDocData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes providerDocAttributes `json:"attributes"`
}

type providerDocAttributes struct {
	Category    string `json:"category"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Subcategory string `json:"subcategory"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	Content     string `json:"content"`
	Truncated   bool   `json:"truncated"`
}

func (t *Toolset) handleListModules(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	registry := registryURL(args)
	query := url.Values{}
	setQueryInt(query, "limit", toInt(args["limit"]))
	setQueryInt(query, "offset", toInt(args["offset"]))
	namespace := strings.TrimSpace(toString(args["namespace"]))
	if provider := toString(args["provider"]); provider != "" {
		query.Set("provider", provider)
	}
	if verified, ok := args["verified"].(bool); ok {
		query.Set("verified", fmt.Sprintf("%t", verified))
	}
	endpoint := fmt.Sprintf("%s/v1/modules", registry)
	if namespace != "" {
		endpoint = fmt.Sprintf("%s/v1/modules/%s", registry, namespace)
	}
	if len(query) > 0 {
		endpoint = endpoint + "?" + query.Encode()
	}
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: payload}, nil
}

func (t *Toolset) handleSearchModules(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	queryValue := strings.TrimSpace(toString(args["query"]))
	if queryValue == "" {
		err := errors.New("query is required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	query := url.Values{}
	query.Set("q", queryValue)
	setQueryInt(query, "limit", toInt(args["limit"]))
	setQueryInt(query, "offset", toInt(args["offset"]))
	if namespace := toString(args["namespace"]); namespace != "" {
		query.Set("namespace", namespace)
	}
	if provider := toString(args["provider"]); provider != "" {
		query.Set("provider", provider)
	}
	if verified, ok := args["verified"].(bool); ok {
		query.Set("verified", fmt.Sprintf("%t", verified))
	}
	endpoint := fmt.Sprintf("%s/v1/modules/search", registry)
	endpoint = endpoint + "?" + query.Encode()
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: payload}, nil
}

func (t *Toolset) handleGetModule(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	name := toString(args["name"])
	provider := toString(args["provider"])
	if namespace == "" || name == "" || provider == "" {
		err := errors.New("namespace, name, and provider are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	version := strings.TrimSpace(toString(args["version"]))
	moduleURL := fmt.Sprintf("%s/v1/modules/%s/%s/%s", registry, namespace, name, provider)
	if version != "" {
		moduleURL = fmt.Sprintf("%s/v1/modules/%s/%s/%s/%s", registry, namespace, name, provider, version)
	}
	versionsURL := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions", registry, namespace, name, provider)
	modulePayload, err := t.getJSON(ctx, moduleURL)
	if err != nil {
		return errorResult(err), err
	}
	versionsPayload, err := t.getJSON(ctx, versionsURL)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"module": modulePayload, "versions": versionsPayload}}, nil
}

func (t *Toolset) handleListModuleVersions(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	name := toString(args["name"])
	provider := toString(args["provider"])
	if namespace == "" || name == "" || provider == "" {
		err := errors.New("namespace, name, and provider are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	versionsURL := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions", registry, namespace, name, provider)
	versionsPayload, err := t.getJSON(ctx, versionsURL)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: versionsPayload}, nil
}

func (t *Toolset) handleListProviders(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	registry := registryURL(args)
	query := url.Values{}
	setQueryInt(query, "page[size]", toInt(args["pageSize"]))
	setQueryInt(query, "page[number]", toInt(args["pageNumber"]))
	if namespace := toString(args["namespace"]); namespace != "" {
		query.Set("filter[namespace]", namespace)
	}
	if tier := toString(args["tier"]); tier != "" {
		query.Set("filter[tier]", tier)
	}
	endpoint := fmt.Sprintf("%s%s", registry, providersV2Path)
	if len(query) > 0 {
		endpoint = endpoint + "?" + query.Encode()
	}
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return errorResult(err), err
	}
	limited := applyLimit(payload, toInt(args["limit"]))
	return mcp.ToolResult{Data: limited}, nil
}

func (t *Toolset) handleSearchProviders(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	queryValue := strings.TrimSpace(toString(args["query"]))
	if queryValue == "" {
		err := errors.New("query is required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	pageSize := toInt(args["pageSize"])
	if pageSize <= 0 {
		pageSize = 50
	}
	pageNumber := toInt(args["pageNumber"])
	if pageNumber <= 0 {
		pageNumber = 1
	}
	limit := toInt(args["limit"])
	if limit <= 0 {
		limit = 50
	}
	providers, err := t.searchProviders(ctx, registry, queryValue, toString(args["namespace"]), toString(args["tier"]), pageSize, pageNumber, limit)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"query": queryValue, "providers": providers, "count": len(providers)}}, nil
}

func (t *Toolset) handleGetProvider(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	providerType := toString(args["type"])
	if namespace == "" || providerType == "" {
		err := errors.New("namespace and type are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	endpoint := fmt.Sprintf("%s%s/%s/%s?include=provider-versions", registry, providersV2Path, namespace, providerType)
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return errorResult(err), err
	}
	version := strings.TrimSpace(toString(args["version"]))
	allowPrerelease := toBool(args["allowPrerelease"])
	providerVersionID, latestVersion := resolveProviderVersionInfo(payload, version, allowPrerelease)
	if version != "" && providerVersionID == "" {
		err := fmt.Errorf("provider version not found: %s", version)
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"provider": payload, "version": chooseVersion(version, latestVersion), "providerVersionID": providerVersionID}}, nil
}

func (t *Toolset) handleListProviderVersions(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	providerType := toString(args["type"])
	if namespace == "" || providerType == "" {
		err := errors.New("namespace and type are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	versionsURL := fmt.Sprintf("%s/v1/providers/%s/%s/versions", registry, namespace, providerType)
	payload, err := t.getJSON(ctx, versionsURL)
	if err != nil {
		return errorResult(err), err
	}
	allowPrerelease := toBool(args["allowPrerelease"])
	limit := toInt(args["limit"])
	return mcp.ToolResult{Data: filterProviderVersionsPayload(payload, allowPrerelease, limit)}, nil
}

func (t *Toolset) handleGetProviderPackage(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	providerType := toString(args["type"])
	version := toString(args["version"])
	osName := toString(args["os"])
	arch := toString(args["arch"])
	if namespace == "" || providerType == "" || version == "" || osName == "" || arch == "" {
		err := errors.New("namespace, type, version, os, and arch are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	endpoint := fmt.Sprintf("%s/v1/providers/%s/%s/%s/download/%s/%s", registry, namespace, providerType, version, osName, arch)
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: payload}, nil
}

func (t *Toolset) handleListResources(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleListDocs(ctx, req, "resources")
}

func (t *Toolset) handleSearchResources(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleSearchDocs(ctx, req, "resources")
}

func (t *Toolset) handleGetResource(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleGetDoc(ctx, req, "resources")
}

func (t *Toolset) handleListDataSources(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleListDocs(ctx, req, "data-sources")
}

func (t *Toolset) handleSearchDataSources(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleSearchDocs(ctx, req, "data-sources")
}

func (t *Toolset) handleGetDataSource(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleGetDoc(ctx, req, "data-sources")
}

func (t *Toolset) searchProviders(ctx context.Context, registry, queryText, namespace, tier string, pageSize, pageNumber, limit int) ([]map[string]any, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}
	if limit <= 0 {
		limit = 50
	}
	normalized := strings.ToLower(strings.TrimSpace(queryText))
	var out []map[string]any
	current := pageNumber
	for scanned := 0; scanned < 20 && len(out) < limit; scanned++ {
		page, nextPage, err := t.listProvidersPage(ctx, registry, namespace, tier, pageSize, current)
		if err != nil {
			return nil, err
		}
		for _, item := range page {
			if providerMatchesQuery(item, normalized) {
				out = append(out, item)
				if len(out) >= limit {
					break
				}
			}
		}
		if nextPage <= 0 {
			break
		}
		current = nextPage
	}
	return out, nil
}

func (t *Toolset) listProvidersPage(ctx context.Context, registry, namespace, tier string, pageSize, pageNumber int) ([]map[string]any, int, error) {
	query := url.Values{}
	setQueryInt(query, "page[size]", pageSize)
	setQueryInt(query, "page[number]", pageNumber)
	if namespace != "" {
		query.Set("filter[namespace]", namespace)
	}
	if tier != "" {
		query.Set("filter[tier]", tier)
	}
	endpoint := fmt.Sprintf("%s%s", registry, providersV2Path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, 0, errors.New("unexpected providers response")
	}
	itemsRaw, ok := root["data"].([]any)
	if !ok {
		return nil, 0, nil
	}
	items := make([]map[string]any, 0, len(itemsRaw))
	for _, raw := range itemsRaw {
		if entry, ok := raw.(map[string]any); ok {
			items = append(items, entry)
		}
	}
	nextPage := 0
	if links, ok := root["links"].(map[string]any); ok {
		nextURL := toString(links["next"])
		if nextURL != "" {
			if parsed, err := url.Parse(nextURL); err == nil {
				if n := parsed.Query().Get("page[number]"); n != "" {
					nextPage = parseInt(n)
				}
			}
		}
	}
	return items, nextPage, nil
}

func providerMatchesQuery(item map[string]any, query string) bool {
	if query == "" {
		return true
	}
	attrs, _ := item["attributes"].(map[string]any)
	values := []string{
		toString(attrs["full-name"]),
		toString(attrs["name"]),
		toString(attrs["namespace"]),
		toString(attrs["description"]),
	}
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func (t *Toolset) searchProviderDocs(ctx context.Context, registry, providerVersionID, category, query string, pageSize, pageNumber, limit int) ([]providerDocData, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}
	if limit <= 0 {
		limit = 50
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	current := pageNumber
	out := make([]providerDocData, 0)
	for scanned := 0; scanned < 30 && len(out) < limit; scanned++ {
		page, nextURL, err := t.listProviderDocsPage(ctx, registry, providerVersionID, category, pageSize, current)
		if err != nil {
			return nil, err
		}
		for _, doc := range page {
			if strings.Contains(strings.ToLower(doc.Attributes.Slug), normalized) || strings.Contains(strings.ToLower(doc.Attributes.Title), normalized) {
				out = append(out, doc)
				if len(out) >= limit {
					break
				}
			}
		}
		if nextURL == "" {
			break
		}
		nextPage := nextPageFromURL(nextURL)
		if nextPage <= 0 {
			break
		}
		current = nextPage
	}
	return out, nil
}

func (t *Toolset) findProviderDoc(ctx context.Context, registry, providerVersionID, category, slug string) (*providerDocData, error) {
	current := 1
	for scanned := 0; scanned < 50; scanned++ {
		page, nextURL, err := t.listProviderDocsPage(ctx, registry, providerVersionID, category, 200, current)
		if err != nil {
			return nil, err
		}
		match := findDocBySlug(page, slug)
		if match != nil {
			return match, nil
		}
		if nextURL == "" {
			break
		}
		nextPage := nextPageFromURL(nextURL)
		if nextPage <= 0 {
			break
		}
		current = nextPage
	}
	return nil, nil
}

func (t *Toolset) handleListDocs(ctx context.Context, req mcp.ToolRequest, category string) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["providerNamespace"])
	providerType := toString(args["providerType"])
	if namespace == "" || providerType == "" {
		err := errors.New("providerNamespace and providerType are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	version := strings.TrimSpace(toString(args["providerVersion"]))
	allowPrerelease := toBool(args["allowPrerelease"])
	providerVersionID, resolvedVersion, err := t.resolveProviderVersionID(ctx, registry, namespace, providerType, version, allowPrerelease)
	if err != nil {
		return errorResult(err), err
	}
	pageSize := toInt(args["pageSize"])
	pageNumber := toInt(args["pageNumber"])
	limit := toInt(args["limit"])
	includeContent := toBool(args["includeContent"])
	list, nextURL, err := t.listProviderDocsPage(ctx, registry, providerVersionID, category, pageSize, pageNumber)
	if err != nil {
		return errorResult(err), err
	}
	if includeContent {
		list, err = t.enrichDocsContent(ctx, registry, list, limit)
		if err != nil {
			return errorResult(err), err
		}
	}
	return mcp.ToolResult{Data: map[string]any{
		"provider": fmt.Sprintf("%s/%s", namespace, providerType),
		"version":  resolvedVersion,
		"category": category,
		"docs":     applyDocLimit(list, limit),
		"next":     nextURL,
	}}, nil
}

func (t *Toolset) handleSearchDocs(ctx context.Context, req mcp.ToolRequest, category string) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["providerNamespace"])
	providerType := toString(args["providerType"])
	queryValue := strings.TrimSpace(toString(args["query"]))
	if namespace == "" || providerType == "" || queryValue == "" {
		err := errors.New("providerNamespace, providerType, and query are required")
		return errorResult(err), err
	}
	registry := registryURL(args)
	version := strings.TrimSpace(toString(args["providerVersion"]))
	allowPrerelease := toBool(args["allowPrerelease"])
	providerVersionID, resolvedVersion, err := t.resolveProviderVersionID(ctx, registry, namespace, providerType, version, allowPrerelease)
	if err != nil {
		return errorResult(err), err
	}
	pageSize := toInt(args["pageSize"])
	pageNumber := toInt(args["pageNumber"])
	limit := toInt(args["limit"])
	if limit <= 0 {
		limit = 50
	}
	includeContent := toBool(args["includeContent"])
	filtered, err := t.searchProviderDocs(ctx, registry, providerVersionID, category, queryValue, pageSize, pageNumber, limit)
	if err != nil {
		return errorResult(err), err
	}
	if includeContent {
		filtered, err = t.enrichDocsContent(ctx, registry, filtered, limit)
		if err != nil {
			return errorResult(err), err
		}
	}
	return mcp.ToolResult{Data: map[string]any{
		"provider": fmt.Sprintf("%s/%s", namespace, providerType),
		"version":  resolvedVersion,
		"category": category,
		"query":    queryValue,
		"docs":     applyDocLimit(filtered, limit),
	}}, nil
}

func (t *Toolset) handleGetDoc(ctx context.Context, req mcp.ToolRequest, category string) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["providerNamespace"])
	providerType := toString(args["providerType"])
	var typeKey string
	if category == "resources" {
		typeKey = "resourceType"
	} else {
		typeKey = "dataSourceType"
	}
	itemType := strings.TrimSpace(toString(args[typeKey]))
	if namespace == "" || providerType == "" || itemType == "" {
		err := fmt.Errorf("providerNamespace, providerType, and %s are required", typeKey)
		return errorResult(err), err
	}
	registry := registryURL(args)
	version := strings.TrimSpace(toString(args["providerVersion"]))
	allowPrerelease := toBool(args["allowPrerelease"])
	providerVersionID, resolvedVersion, err := t.resolveProviderVersionID(ctx, registry, namespace, providerType, version, allowPrerelease)
	if err != nil {
		return errorResult(err), err
	}
	match, err := t.findProviderDoc(ctx, registry, providerVersionID, category, itemType)
	if err != nil {
		return errorResult(err), err
	}
	if match == nil {
		err := fmt.Errorf("%s not found: %s", category, itemType)
		return errorResult(err), err
	}
	doc, err := t.fetchProviderDoc(ctx, registry, match.ID)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"provider": fmt.Sprintf("%s/%s", namespace, providerType),
		"version":  resolvedVersion,
		"category": category,
		"doc":      doc,
	}}, nil
}

func (t *Toolset) resolveProviderVersionID(ctx context.Context, registry, namespace, providerType, version string, allowPrerelease bool) (string, string, error) {
	endpoint := fmt.Sprintf("%s%s/%s/%s?include=provider-versions", registry, providersV2Path, namespace, providerType)
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return "", "", err
	}
	providerVersionID, latest := resolveProviderVersionInfo(payload, version, allowPrerelease)
	resolved := chooseVersion(version, latest)
	if providerVersionID == "" {
		return "", "", errors.New("provider version not found")
	}
	return providerVersionID, resolved, nil
}

func (t *Toolset) listProviderDocsPage(ctx context.Context, registry, providerVersionID, category string, pageSize, pageNumber int) ([]providerDocData, string, error) {
	query := url.Values{}
	query.Set("filter[provider-version]", providerVersionID)
	query.Set("filter[category]", category)
	setQueryInt(query, "page[size]", pageSize)
	setQueryInt(query, "page[number]", pageNumber)
	endpoint := fmt.Sprintf("%s%s?%s", registry, providerDocsV2Path, query.Encode())
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return nil, "", err
	}
	data, ok := payload.(map[string]any)
	if !ok {
		return nil, "", errors.New("unexpected provider docs response")
	}
	nextURL := ""
	if links, ok := data["links"].(map[string]any); ok {
		nextURL = toString(links["next"])
	}
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	var decoded providerDocsResponse
	if err := json.Unmarshal(buf, &decoded); err != nil {
		return nil, "", err
	}
	return decoded.Data, nextURL, nil
}

func (t *Toolset) fetchProviderDoc(ctx context.Context, registry, docID string) (providerDocData, error) {
	endpoint := fmt.Sprintf("%s%s/%s", registry, providerDocsV2Path, docID)
	payload, err := t.getJSON(ctx, endpoint)
	if err != nil {
		return providerDocData{}, err
	}
	data, ok := payload.(map[string]any)
	if !ok {
		return providerDocData{}, errors.New("unexpected provider doc response")
	}
	buf, err := json.Marshal(data)
	if err != nil {
		return providerDocData{}, err
	}
	var wrapper struct {
		Data providerDocData `json:"data"`
	}
	if err := json.Unmarshal(buf, &wrapper); err != nil {
		return providerDocData{}, err
	}
	return wrapper.Data, nil
}

func (t *Toolset) enrichDocsContent(ctx context.Context, registry string, list []providerDocData, limit int) ([]providerDocData, error) {
	count := len(list)
	if limit > 0 && limit < count {
		count = limit
	}
	for i := 0; i < count; i++ {
		doc, err := t.fetchProviderDoc(ctx, registry, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i] = doc
	}
	return list, nil
}

func resolveProviderVersionInfo(payload any, requested string, allowPrerelease bool) (string, string) {
	dataMap, ok := payload.(map[string]any)
	if !ok {
		return "", ""
	}
	buf, err := json.Marshal(dataMap)
	if err != nil {
		return "", ""
	}
	var decoded providerDetailResponse
	if err := json.Unmarshal(buf, &decoded); err != nil {
		return "", ""
	}
	latest := ""
	var available []string
	requestedID := ""
	for _, item := range decoded.Included {
		if item.Type != "provider-versions" {
			continue
		}
		available = append(available, item.Attributes.Version)
		if requested != "" && item.Attributes.Version == requested {
			requestedID = item.ID
		}
	}
	latest = pickLatestVersion(available, allowPrerelease)
	if requested != "" {
		return requestedID, latest
	}
	if latest == "" {
		return "", ""
	}
	for _, item := range decoded.Included {
		if item.Type != "provider-versions" {
			continue
		}
		if item.Attributes.Version == latest {
			return item.ID, latest
		}
	}
	return "", latest
}

func pickLatestVersion(versions []string, allowPrerelease bool) string {
	if len(versions) == 0 {
		return ""
	}
	var parsed []*semver.Version
	for _, raw := range versions {
		if raw == "" {
			continue
		}
		if v, err := semver.NewVersion(raw); err == nil {
			if len(v.Prerelease()) > 0 && !allowPrerelease {
				continue
			}
			parsed = append(parsed, v)
		}
	}
	if len(parsed) == 0 {
		sort.Strings(versions)
		return versions[len(versions)-1]
	}
	sort.Sort(semver.Collection(parsed))
	return parsed[len(parsed)-1].String()
}

func chooseVersion(requested, latest string) string {
	if requested != "" {
		return requested
	}
	return latest
}

func filterProviderVersionsPayload(payload any, allowPrerelease bool, limit int) any {
	root, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	versionsRaw, ok := root["versions"].([]any)
	if !ok {
		return payload
	}
	filtered := make([]any, 0, len(versionsRaw))
	for _, item := range versionsRaw {
		versionEntry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		version := toString(versionEntry["version"])
		if version == "" {
			continue
		}
		parsed, err := semver.NewVersion(version)
		if err != nil {
			filtered = append(filtered, versionEntry)
			continue
		}
		if len(parsed.Prerelease()) > 0 && !allowPrerelease {
			continue
		}
		filtered = append(filtered, versionEntry)
	}
	sort.Slice(filtered, func(i, j int) bool {
		left := toString(filtered[i].(map[string]any)["version"])
		right := toString(filtered[j].(map[string]any)["version"])
		lv, le := semver.NewVersion(left)
		rv, re := semver.NewVersion(right)
		if le != nil || re != nil {
			return left > right
		}
		return lv.GreaterThan(rv)
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	root["versions"] = filtered
	root["count"] = len(filtered)
	return root
}

func applyLimit(payload any, limit int) any {
	if limit <= 0 {
		return payload
	}
	data, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	items, ok := data["data"].([]any)
	if !ok {
		return payload
	}
	if limit < len(items) {
		data["data"] = items[:limit]
	}
	return data
}

func filterDocsByQuery(list []providerDocData, query string) []providerDocData {
	normalized := strings.ToLower(query)
	filtered := make([]providerDocData, 0, len(list))
	for _, doc := range list {
		if strings.Contains(strings.ToLower(doc.Attributes.Slug), normalized) ||
			strings.Contains(strings.ToLower(doc.Attributes.Title), normalized) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

func applyDocLimit(list []providerDocData, limit int) []providerDocData {
	if limit > 0 && limit < len(list) {
		return list[:limit]
	}
	return list
}

func findDocBySlug(list []providerDocData, slug string) *providerDocData {
	for _, doc := range list {
		if doc.Attributes.Slug == slug || doc.Attributes.Title == slug {
			copy := doc
			return &copy
		}
	}
	return nil
}

func (t *Toolset) getJSON(ctx context.Context, endpoint string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	client := t.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registry request failed: %s", strings.TrimSpace(string(body)))
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

func registryURL(args map[string]any) string {
	if val := strings.TrimSpace(toString(args["registryURL"])); val != "" {
		return strings.TrimRight(val, "/")
	}
	return defaultRegistryURL
}

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}
}

func setQueryInt(values url.Values, key string, value int) {
	if value > 0 {
		values.Set(key, fmt.Sprintf("%d", value))
	}
}

func parseInt(raw string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return parsed
}

func nextPageFromURL(raw string) int {
	if raw == "" {
		return 0
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	return parseInt(parsed.Query().Get("page[number]"))
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
