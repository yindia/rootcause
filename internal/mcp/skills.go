package mcp

import (
	"fmt"
	"os"
	"strings"

	"rootcause/internal/config"
	"rootcause/internal/skills/catalog"
)

const maxSkillGuidanceBytes = 4000

func customSkillGuidanceForTool(cfg *config.Config, spec ToolSpec, args map[string]any) ([]SkillGuidance, error) {
	if cfg == nil || len(cfg.Skills.CustomDirs) == 0 {
		return nil, nil
	}
	manifest, err := catalog.LoadWithCustom(catalog.CustomOptions{
		Dirs:           cfg.Skills.CustomDirs,
		AllowOverrides: cfg.Skills.AllowCustomOverrides,
	})
	if err != nil {
		return nil, err
	}
	callTags := toolCallTags(spec, args)
	guidance := make([]SkillGuidance, 0)
	for _, skill := range manifest.Skills {
		if !skill.Custom || !tagsIntersect(callTags, skill.Tags) {
			continue
		}
		data, err := os.ReadFile(skill.Path)
		if err != nil {
			return nil, fmt.Errorf("read custom skill %s: %w", skill.Name, err)
		}
		content := string(data)
		truncated := false
		if len(content) > maxSkillGuidanceBytes {
			content = content[:maxSkillGuidanceBytes]
			truncated = true
		}
		guidance = append(guidance, SkillGuidance{
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        append([]string{}, skill.Tags...),
			Content:     content,
			Truncated:   truncated,
		})
	}
	return guidance, nil
}

func attachCustomSkillGuidance(result ToolResult, guidance []SkillGuidance, guidanceErr error) ToolResult {
	if len(guidance) > 0 {
		result.Metadata.CustomSkills = guidance
	}
	if guidanceErr != nil {
		result.Metadata.CustomSkillError = guidanceErr.Error()
	}
	root, ok := result.Data.(map[string]any)
	if !ok {
		return result
	}
	if len(guidance) > 0 {
		if _, exists := root["customSkillGuidance"]; !exists {
			root["customSkillGuidance"] = guidance
		}
	}
	if guidanceErr != nil {
		if _, exists := root["customSkillError"]; !exists {
			root["customSkillError"] = guidanceErr.Error()
		}
	}
	return result
}

func toolCallTags(spec ToolSpec, args map[string]any) map[string]struct{} {
	tags := map[string]struct{}{}
	addSkillTag(tags, spec.ToolsetID)
	addSkillTag(tags, "toolset:"+spec.ToolsetID)
	addSkillTag(tags, spec.Name)
	addSkillTag(tags, "tool:"+spec.Name)
	addSkillTag(tags, string(spec.Safety))
	for _, token := range strings.FieldsFunc(spec.Name, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	}) {
		addSkillTag(tags, token)
	}
	addTagsFromArg(tags, args["skillTags"])
	addTagsFromArg(tags, args["customSkillTags"])
	return tags
}

func tagsIntersect(callTags map[string]struct{}, skillTags []string) bool {
	for _, tag := range skillTags {
		if _, ok := callTags[normalizeSkillTag(tag)]; ok {
			return true
		}
	}
	return false
}

func addTagsFromArg(tags map[string]struct{}, value any) {
	switch typed := value.(type) {
	case string:
		for _, part := range strings.Split(typed, ",") {
			addSkillTag(tags, part)
		}
	case []string:
		for _, part := range typed {
			addSkillTag(tags, part)
		}
	case []any:
		for _, item := range typed {
			if part, ok := item.(string); ok {
				addSkillTag(tags, part)
			}
		}
	}
}

func addSkillTag(tags map[string]struct{}, tag string) {
	normalized := normalizeSkillTag(tag)
	if normalized != "" {
		tags[normalized] = struct{}{}
	}
}

func normalizeSkillTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
