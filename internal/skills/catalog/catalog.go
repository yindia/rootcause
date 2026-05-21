package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

//go:embed manifest.json
var manifestBytes []byte

type Manifest struct {
	SchemaVersion string  `json:"schemaVersion"`
	Version       string  `json:"version"`
	SourceFormat  string  `json:"sourceFormat"`
	Skills        []Skill `json:"skills"`
}

type Skill struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Tags        []string `json:"tags,omitempty"`
	Custom      bool     `json:"-"`
}

type skillFrontMatter struct {
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type CustomOptions struct {
	Dirs           []string
	AllowOverrides bool
}

func Load() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse skills manifest: %w", err)
	}
	if err := validate(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func LoadWithCustom(opts CustomOptions) (Manifest, error) {
	manifest, err := Load()
	if err != nil {
		return Manifest{}, err
	}
	custom, err := DiscoverCustom(opts.Dirs)
	if err != nil {
		return Manifest{}, err
	}
	merged, err := Merge(manifest.Skills, custom, opts.AllowOverrides)
	if err != nil {
		return Manifest{}, err
	}
	manifest.Skills = merged
	return manifest, nil
}

func DiscoverCustom(dirs []string) ([]Skill, error) {
	var skills []Skill
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		resolved, err := resolvePath(dir)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(resolved) == "" {
			continue
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read custom skills dir %s: %w", resolved, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			file := filepath.Join(resolved, name, "SKILL.md")
			data, err := os.ReadFile(file)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("custom skill %s missing SKILL.md at %s", name, file)
				}
				return nil, fmt.Errorf("read custom skill %s: %w", name, err)
			}
			meta, body := parseSkillFrontMatter(data)
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				return nil, fmt.Errorf("duplicate custom skill: %s", name)
			}
			seen[key] = struct{}{}
			category := strings.TrimSpace(meta.Category)
			if category == "" {
				category = "Custom"
			}
			description := strings.TrimSpace(meta.Description)
			if description == "" {
				description = customDescription(body, file)
			}
			skills = append(skills, Skill{
				Name:        name,
				Category:    category,
				Description: description,
				Path:        file,
				Tags:        normalizeTags(meta.Tags),
				Custom:      true,
			})
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

func Merge(builtin []Skill, custom []Skill, allowOverrides bool) ([]Skill, error) {
	merged := make([]Skill, 0, len(builtin)+len(custom))
	indexByName := map[string]int{}
	for _, skill := range builtin {
		key := strings.ToLower(skill.Name)
		if _, ok := indexByName[key]; ok {
			return nil, fmt.Errorf("duplicate built-in skill: %s", skill.Name)
		}
		indexByName[key] = len(merged)
		merged = append(merged, skill)
	}
	for _, skill := range custom {
		key := strings.ToLower(skill.Name)
		if idx, ok := indexByName[key]; ok {
			if !allowOverrides {
				return nil, fmt.Errorf("custom skill %q conflicts with built-in skill; rename it or enable custom overrides", skill.Name)
			}
			merged[idx] = skill
			continue
		}
		indexByName[key] = len(merged)
		merged = append(merged, skill)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
	return merged, nil
}

func validate(m Manifest) error {
	if strings.TrimSpace(m.SchemaVersion) == "" {
		return fmt.Errorf("skills manifest missing schemaVersion")
	}
	if len(m.Skills) == 0 {
		return fmt.Errorf("skills manifest has no skills")
	}
	seen := map[string]struct{}{}
	for _, s := range m.Skills {
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Path) == "" {
			return fmt.Errorf("invalid skill entry: name/path required")
		}
		if _, ok := seen[s.Name]; ok {
			return fmt.Errorf("duplicate skill in manifest: %s", s.Name)
		}
		seen[s.Name] = struct{}{}
	}
	return nil
}

func ByCategory(m Manifest) map[string][]Skill {
	out := map[string][]Skill{}
	for _, s := range m.Skills {
		out[s.Category] = append(out[s.Category], s)
	}
	for category := range out {
		sort.Slice(out[category], func(i, j int) bool {
			return out[category][i].Name < out[category][j].Name
		})
	}
	return out
}

func Categories(m Manifest) []string {
	seen := map[string]struct{}{}
	for _, s := range m.Skills {
		seen[s.Category] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func SkillDir(skill Skill) string {
	return filepath.Dir(skill.Path)
}

func SkillFile(sourceDir string, skill Skill) string {
	if skill.Custom || filepath.IsAbs(skill.Path) {
		return skill.Path
	}
	return filepath.Join(sourceDir, filepath.Base(SkillDir(skill)), "SKILL.md")
}

func resolvePath(path string) (string, error) {
	trimmed := strings.TrimSpace(os.ExpandEnv(path))
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "~/") || trimmed == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if trimmed == "~" {
			trimmed = home
		} else {
			trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return filepath.Abs(trimmed)
}

func customDescription(data []byte, file string) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if title, ok := strings.CutPrefix(trimmed, "# "); ok {
			return strings.TrimSpace(title)
		}
	}
	return fmt.Sprintf("Custom skill from %s", file)
}

func parseSkillFrontMatter(data []byte) (skillFrontMatter, []byte) {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return skillFrontMatter{}, data
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	rest := strings.TrimPrefix(normalized, "---\n")
	frontMatter, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return skillFrontMatter{}, data
	}
	var meta skillFrontMatter
	if err := yaml.Unmarshal([]byte(frontMatter), &meta); err != nil {
		return skillFrontMatter{}, []byte(body)
	}
	return meta, []byte(body)
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	for _, tag := range tags {
		trimmed := strings.ToLower(strings.TrimSpace(tag))
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}
