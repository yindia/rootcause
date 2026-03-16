package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Path        string `json:"path"`
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
