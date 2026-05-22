package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rootcause/internal/config"
	"rootcause/internal/policy"
)

func TestInvokerToolNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	invoker := NewToolInvoker(reg, ToolContext{})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "missing", nil)
	if err == nil {
		t.Fatalf("expected error for missing tool")
	}
	root, ok := result.Data.(map[string]any)
	if !ok || root["error"] == nil {
		t.Fatalf("expected error envelope in result data: %#v", result.Data)
	}
}

func TestInvokerHandlerError(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"error": "fail"}}, errors.New("fail")
		},
	})
	ctx := ToolContext{Policy: policy.NewAuthorizer()}
	invoker := NewToolInvoker(reg, ctx)
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected handler error")
	}
}

func TestInvokerMissingRegistry(t *testing.T) {
	invoker := &ToolInvoker{}
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected error for missing registry")
	}
	root, ok := result.Data.(map[string]any)
	if !ok || root["error"] == nil {
		t.Fatalf("expected error envelope in result data: %#v", result.Data)
	}
}

func TestInvokerSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(_ context.Context, _ ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer()})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatalf("expected result data")
	}
}

func TestInvokerAttachesTaggedCustomSkillGuidance(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [demo]\ndescription: Demo tool guidance\n---\n# Demo Skill\n"
	writeMCPTestCustomSkill(t, customRoot, "demo-skill", content)

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo.inspect",
		ToolsetID: "demo",
		Safety:    SafetyReadOnly,
		Handler: func(_ context.Context, _ ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo.inspect", nil)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if len(result.Metadata.CustomSkills) != 1 {
		t.Fatalf("expected one metadata custom skill, got %#v", result.Metadata.CustomSkills)
	}
	root := result.Data.(map[string]any)
	guidance := root["customSkillGuidance"].([]SkillGuidance)
	if len(guidance) != 1 || guidance[0].Name != "demo-skill" || guidance[0].Content != content {
		t.Fatalf("unexpected custom skill guidance: %#v", guidance)
	}
}

func TestCustomSkillGuidanceMatchesRootCauseToolsetTag(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [rootcause]\ndescription: RootCause issue runbook\n---\n# RootCause Runbook\n"
	writeMCPTestCustomSkill(t, customRoot, "rootcause-runbook", content)

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "rootcause.incident_bundle", ToolsetID: "rootcause", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 1 {
		t.Fatalf("expected one rootcause guidance item, got %#v", guidance)
	}
	if guidance[0].Name != "rootcause-runbook" || guidance[0].Content != content {
		t.Fatalf("unexpected rootcause guidance: %#v", guidance[0])
	}
}

func TestCustomSkillGuidanceMatchesExplicitCustomSkillTags(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [payments]\ndescription: Payments runbook\n---\n# Payments Runbook\n"
	writeMCPTestCustomSkill(t, customRoot, "payments-runbook", content)

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "k8s.events", ToolsetID: "k8s", Safety: SafetyReadOnly}
	cache := newCustomSkillCache()

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, cache)
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool without tag: %v", err)
	}
	if len(guidance) != 0 {
		t.Fatalf("expected no guidance without explicit payments tag, got %#v", guidance)
	}

	guidance, err = customSkillGuidanceForTool(&cfg, spec, map[string]any{"customSkillTags": []any{"payments"}}, cache)
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool with tag: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Name != "payments-runbook" {
		t.Fatalf("expected payments guidance from customSkillTags, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceMatchesSkillTagsString(t *testing.T) {
	customRoot := t.TempDir()
	writeMCPTestCustomSkill(t, customRoot, "case-runbook", "---\ntags: [Payments]\ndescription: Case runbook\n---\n# Case Runbook\n")

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "k8s.events", ToolsetID: "k8s", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, map[string]any{"skillTags": "other, payments"}, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Name != "case-runbook" {
		t.Fatalf("expected case-insensitive guidance from skillTags string, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceMatchesSkillTagsStringSlice(t *testing.T) {
	customRoot := t.TempDir()
	writeMCPTestCustomSkill(t, customRoot, "array-runbook", "---\ntags: [payments]\ndescription: Array runbook\n---\n# Array Runbook\n")

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "k8s.events", ToolsetID: "k8s", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, map[string]any{"skillTags": []string{"payments"}}, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Name != "array-runbook" {
		t.Fatalf("expected guidance from skillTags string slice, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceReturnsNilWithoutCustomSkillConfig(t *testing.T) {
	spec := ToolSpec{Name: "rootcause.rca_generate", ToolsetID: "rootcause", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(nil, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool nil config: %v", err)
	}
	if guidance != nil {
		t.Fatalf("expected nil guidance for nil config, got %#v", guidance)
	}

	cfg := config.Config{}
	guidance, err = customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool empty config: %v", err)
	}
	if guidance != nil {
		t.Fatalf("expected nil guidance without custom dirs, got %#v", guidance)
	}
}

func TestCachedCustomSkillCandidatesSupportsNilCacheAndCloneIsolation(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [rootcause]\ndescription: Clone runbook\n---\n# Clone Runbook\n"
	writeMCPTestCustomSkill(t, customRoot, "clone-runbook", content)

	candidates, err := cachedCustomSkillCandidates([]string{customRoot}, false, nil)
	if err != nil {
		t.Fatalf("cachedCustomSkillCandidates nil cache: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Guidance.Content != content {
		t.Fatalf("unexpected nil-cache candidates: %#v", candidates)
	}

	cache := newCustomSkillCache()
	candidates, err = cachedCustomSkillCandidates([]string{customRoot}, false, cache)
	if err != nil {
		t.Fatalf("cachedCustomSkillCandidates initial cache: %v", err)
	}
	const mutatedValue = "mutated"
	candidates[0].Guidance.Content = mutatedValue
	candidates[0].Guidance.Tags[0] = mutatedValue
	candidates[0].Tags[0] = mutatedValue

	candidates, err = cachedCustomSkillCandidates([]string{customRoot}, false, cache)
	if err != nil {
		t.Fatalf("cachedCustomSkillCandidates cached: %v", err)
	}
	if candidates[0].Guidance.Content != content || candidates[0].Guidance.Tags[0] != "rootcause" || candidates[0].Tags[0] != "rootcause" {
		t.Fatalf("expected cached candidates to be cloned, got %#v", candidates)
	}
}

func TestCustomSkillGuidanceReturnsMultipleMatchesInCatalogOrder(t *testing.T) {
	customRoot := t.TempDir()
	writeMCPTestCustomSkill(t, customRoot, "z-runbook", "---\ntags: [rootcause]\n---\n# Z Runbook\n")
	writeMCPTestCustomSkill(t, customRoot, "a-runbook", "---\ntags: [rootcause]\n---\n# A Runbook\n")

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "rootcause.rca_generate", ToolsetID: "rootcause", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 2 {
		t.Fatalf("expected two matching custom skills, got %#v", guidance)
	}
	if guidance[0].Name != "a-runbook" || guidance[1].Name != "z-runbook" {
		t.Fatalf("expected custom skills in catalog order, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceSkipsUntaggedCustomSkill(t *testing.T) {
	customRoot := t.TempDir()
	writeMCPTestCustomSkill(t, customRoot, "untagged-runbook", "# Untagged Runbook\n")

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "rootcause.rca_generate", ToolsetID: "rootcause", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 0 {
		t.Fatalf("expected untagged custom skill to stay discoverable but uninjected, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceMatchesToolNameTokenTag(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [timeline]\ndescription: Timeline runbook\n---\n# Timeline Runbook\n"
	writeMCPTestCustomSkill(t, customRoot, "timeline-runbook", content)

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "rootcause.change_timeline", ToolsetID: "rootcause", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Name != "timeline-runbook" || guidance[0].Content != content {
		t.Fatalf("expected timeline guidance from tool-name token, got %#v", guidance)
	}
}

func TestCustomSkillGuidanceTruncatesLargeCustomSkillContent(t *testing.T) {
	customRoot := t.TempDir()
	content := "---\ntags: [demo]\n---\n" + strings.Repeat("x", maxSkillGuidanceBytes)
	writeMCPTestCustomSkill(t, customRoot, "large-runbook", content)

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "demo.inspect", ToolsetID: "demo", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, newCustomSkillCache())
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool: %v", err)
	}
	if len(guidance) != 1 {
		t.Fatalf("expected one large guidance item, got %#v", guidance)
	}
	if !guidance[0].Truncated {
		t.Fatalf("expected large guidance to be marked truncated")
	}
	if len(guidance[0].Content) != maxSkillGuidanceBytes {
		t.Fatalf("expected truncated content length %d, got %d", maxSkillGuidanceBytes, len(guidance[0].Content))
	}
}

func TestInvokerAttachesCustomSkillErrorWithoutFailingTool(t *testing.T) {
	customRoot := t.TempDir()
	missingSkillDir := filepath.Join(customRoot, "broken-runbook")
	err := os.MkdirAll(missingSkillDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir broken custom skill: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	reg := NewRegistry(&cfg)
	err = reg.Add(ToolSpec{
		Name:      "demo.inspect",
		ToolsetID: "demo",
		Safety:    SafetyReadOnly,
		Handler: func(_ context.Context, _ ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("add tool: %v", err)
	}

	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo.inspect", nil)
	if err != nil {
		t.Fatalf("expected tool success despite custom skill error: %v", err)
	}
	if result.Metadata.CustomSkillError == "" {
		t.Fatalf("expected custom skill error metadata")
	}
	root, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result data, got %#v", result.Data)
	}
	if root["customSkillError"] == "" {
		t.Fatalf("expected payload customSkillError, got %#v", root)
	}
}

func TestAttachCustomSkillGuidanceDoesNotOverwriteExistingPayloadFields(t *testing.T) {
	existingGuidance := []SkillGuidance{{Name: "existing", Content: "keep"}}
	newGuidance := []SkillGuidance{{Name: "new", Content: "replace"}}
	result := ToolResult{Data: map[string]any{"customSkillGuidance": existingGuidance, "customSkillError": "existing error"}}

	updated := attachCustomSkillGuidance(result, newGuidance, errors.New("new error"))
	root, ok := updated.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result data, got %#v", updated.Data)
	}
	payloadGuidance, ok := root["customSkillGuidance"].([]SkillGuidance)
	if !ok {
		t.Fatalf("expected existing payload guidance slice, got %#v", root["customSkillGuidance"])
	}
	if len(payloadGuidance) != 1 || payloadGuidance[0].Name != "existing" {
		t.Fatalf("expected existing payload guidance to be preserved, got %#v", root["customSkillGuidance"])
	}
	if root["customSkillError"] != "existing error" {
		t.Fatalf("expected existing payload error to be preserved, got %#v", root["customSkillError"])
	}
	if len(updated.Metadata.CustomSkills) != 1 || updated.Metadata.CustomSkills[0].Name != "new" {
		t.Fatalf("expected metadata to receive new guidance, got %#v", updated.Metadata.CustomSkills)
	}
	if updated.Metadata.CustomSkillError != "new error" {
		t.Fatalf("expected metadata custom skill error, got %q", updated.Metadata.CustomSkillError)
	}
}

func TestAttachCustomSkillGuidanceHandlesNonMapData(t *testing.T) {
	guidance := []SkillGuidance{{Name: "skill", Content: "content"}}
	result := ToolResult{Data: "plain text"}

	updated := attachCustomSkillGuidance(result, guidance, errors.New("custom failure"))
	if updated.Data != "plain text" {
		t.Fatalf("expected non-map data to be preserved, got %#v", updated.Data)
	}
	if len(updated.Metadata.CustomSkills) != 1 || updated.Metadata.CustomSkills[0].Name != "skill" {
		t.Fatalf("expected metadata guidance for non-map data, got %#v", updated.Metadata.CustomSkills)
	}
	if updated.Metadata.CustomSkillError != "custom failure" {
		t.Fatalf("expected metadata custom skill error, got %q", updated.Metadata.CustomSkillError)
	}
}

func TestCustomSkillStateHelpersCoverMissingBlankAndMismatch(t *testing.T) {
	blankState, err := statPath(" ")
	if err != nil {
		t.Fatalf("statPath blank: %v", err)
	}
	if blankState.Path != " " || blankState.Exists {
		t.Fatalf("unexpected blank path state: %#v", blankState)
	}

	missingPath := filepath.Join(t.TempDir(), "missing", "SKILL.md")
	missingState, err := statPath(missingPath)
	if err != nil {
		t.Fatalf("statPath missing: %v", err)
	}
	if missingState.Path != missingPath || missingState.Exists {
		t.Fatalf("unexpected missing path state: %#v", missingState)
	}
	if fileStatesEqual([]fileState{missingState}, nil) {
		t.Fatalf("expected fileStatesEqual to reject length mismatch")
	}
	changedState := missingState
	changedState.Size++
	if fileStatesEqual([]fileState{missingState}, []fileState{changedState}) {
		t.Fatalf("expected fileStatesEqual to reject state mismatch")
	}
	if fileStatesStillCurrent([]fileState{changedState}) {
		t.Fatalf("expected stale file state to be detected")
	}
}

func TestResolveSkillPathExpandsEnvironmentAndBlank(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOTCAUSE_MCP_SKILLS_DIR", dir)

	blankPath, err := resolveSkillPath(" ")
	if err != nil {
		t.Fatalf("resolveSkillPath blank: %v", err)
	}
	if blankPath != "" {
		t.Fatalf("expected blank skill path to resolve to empty string, got %q", blankPath)
	}
	resolved, err := resolveSkillPath("$ROOTCAUSE_MCP_SKILLS_DIR")
	if err != nil {
		t.Fatalf("resolveSkillPath env: %v", err)
	}
	if resolved != dir {
		t.Fatalf("expected env-expanded path %q, got %q", dir, resolved)
	}
}

func TestRegistryListExposesGlobalSkillTagArguments(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	err := reg.Add(ToolSpec{
		Name:      "demo.inspect",
		ToolsetID: "demo",
		Safety:    SafetyReadOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("add tool: %v", err)
	}

	infos := reg.List()
	if len(infos) != 1 {
		t.Fatalf("expected one tool, got %#v", infos)
	}
	props, ok := infos[0].InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %#v", infos[0].InputSchema["properties"])
	}
	if _, exists := props["namespace"]; !exists {
		t.Fatalf("expected original namespace property to be preserved: %#v", props)
	}
	for _, field := range []string{"skillTags", "customSkillTags"} {
		fieldSchema, exists := props[field]
		if !exists {
			t.Fatalf("expected %s schema property in %#v", field, props)
		}
		fieldMap, ok := fieldSchema.(map[string]any)
		if !ok {
			t.Fatalf("expected %s schema map, got %#v", field, fieldSchema)
		}
		if _, exists := fieldMap["oneOf"]; !exists {
			t.Fatalf("expected %s to accept string or string array, got %#v", field, fieldMap)
		}
	}
}

func TestCustomSkillGuidanceCacheRefreshesOnFileChange(t *testing.T) {
	cache := newCustomSkillCache()

	customRoot := t.TempDir()
	skillFile := writeMCPTestCustomSkill(t, customRoot, "demo-skill", "")
	first := "---\ntags: [demo]\ndescription: Demo guidance\n---\n# First\n"
	err := os.WriteFile(skillFile, []byte(first), 0o600)
	if err != nil {
		t.Fatalf("write first skill: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Skills.CustomDirs = []string{customRoot}
	spec := ToolSpec{Name: "demo.inspect", ToolsetID: "demo", Safety: SafetyReadOnly}

	guidance, err := customSkillGuidanceForTool(&cfg, spec, nil, cache)
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool first: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Content != first {
		t.Fatalf("expected first guidance, got %#v", guidance)
	}
	cache.mu.Lock()
	cacheEntries := len(cache.entries)
	cache.mu.Unlock()
	if cacheEntries != 1 {
		t.Fatalf("expected one cache entry, got %d", cacheEntries)
	}

	second := "---\ntags: [demo]\ndescription: Demo guidance\n---\n# Second\n"
	err = os.WriteFile(skillFile, []byte(second), 0o600)
	if err != nil {
		t.Fatalf("write second skill: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	err = os.Chtimes(skillFile, future, future)
	if err != nil {
		t.Fatalf("touch second skill: %v", err)
	}

	guidance, err = customSkillGuidanceForTool(&cfg, spec, nil, cache)
	if err != nil {
		t.Fatalf("customSkillGuidanceForTool second: %v", err)
	}
	if len(guidance) != 1 || guidance[0].Content != second {
		t.Fatalf("expected refreshed guidance, got %#v", guidance)
	}
}

func writeMCPTestCustomSkill(t *testing.T, root string, name string, content string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir custom skill: %v", err)
	}

	file := filepath.Join(dir, "SKILL.md")
	err = os.WriteFile(file, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write custom skill: %v", err)
	}
	return file
}

func TestInvokerRunsMutationPreflight(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.safe_mutation_preflight",
		ToolsetID: "k8s",
		Safety:    SafetyReadOnly,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			if op, _ := req.Arguments["operation"].(string); op != "apply" {
				t.Fatalf("expected apply operation, got %q", op)
			}
			return ToolResult{Data: map[string]any{"safeToProceed": false}}, nil
		},
	})
	_ = reg.Add(ToolSpec{
		Name:      "k8s.apply",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			t.Fatalf("mutation handler should not run when preflight fails")
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})

	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.apply", map[string]any{"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"})
	if err == nil {
		t.Fatalf("expected preflight failure")
	}
}

func TestInvokerFailsWhenPreflightToolMissing(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.apply",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.apply", map[string]any{"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"})
	if err == nil {
		t.Fatalf("expected error when preflight tool is missing")
	}
}

func TestInvokerFailsWhenPreflightResponseMalformed(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.safe_mutation_preflight",
		ToolsetID: "k8s",
		Safety:    SafetyReadOnly,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"checks": []any{}}}, nil
		},
	})
	_ = reg.Add(ToolSpec{
		Name:      "k8s.patch",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.patch", map[string]any{"name": "x", "patch": "{}"})
	if err == nil {
		t.Fatalf("expected malformed preflight response error")
	}
}

func TestInvokerChecksNamespaceForNamespaceRole(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.namespaced_demo",
		ToolsetID: "k8s",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"team-a"}}, "k8s.namespaced_demo", map[string]any{"namespace": "team-b"})
	if err == nil {
		t.Fatalf("expected namespace policy error")
	}
}

func TestInvokerDeniesClusterScopedForNamespaceRole(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "cluster.info",
		ToolsetID: "cluster",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"team-a"}}, "cluster.info", map[string]any{})
	if err == nil {
		t.Fatalf("expected cluster-scoped policy error")
	}
}
