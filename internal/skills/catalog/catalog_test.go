package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverCustomSkills(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "team-runbook", "---\ncategory: Team\ndescription: Team incident runbook\ntags: [rootcause, rca, payments]\n---\n# Team Runbook\n")

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 custom skill, got %d", len(skills))
	}
	skill := skills[0]
	if skill.Name != "team-runbook" {
		t.Fatalf("unexpected skill name: %s", skill.Name)
	}
	if !skill.Custom {
		t.Fatalf("expected custom skill flag")
	}
	if skill.Category != "Team" {
		t.Fatalf("unexpected category: %s", skill.Category)
	}
	if skill.Description != "Team incident runbook" {
		t.Fatalf("unexpected description: %s", skill.Description)
	}
	if len(skill.Tags) != 3 || skill.Tags[0] != "payments" || skill.Tags[1] != "rca" || skill.Tags[2] != "rootcause" {
		t.Fatalf("unexpected tags: %#v", skill.Tags)
	}
	if !filepath.IsAbs(skill.Path) {
		t.Fatalf("expected absolute path, got %s", skill.Path)
	}
}

func TestDiscoverCustomSkillsRequiresSkillFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatalf("mkdir custom skill: %v", err)
	}

	_, err := DiscoverCustom([]string{dir})
	if err == nil {
		t.Fatalf("expected missing SKILL.md error")
	}
	if !strings.Contains(err.Error(), "missing SKILL.md") {
		t.Fatalf("expected missing SKILL.md error, got %v", err)
	}
}

func TestDiscoverCustomSkillsIgnoresMissingRootDir(t *testing.T) {
	skills, err := DiscoverCustom([]string{filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected no skills from missing root dir, got %#v", skills)
	}
}

func TestMergeRejectsDuplicateCustomSkillByDefault(t *testing.T) {
	_, err := Merge(
		[]Skill{{Name: "k8s-core", Path: "skills/claude/k8s-core/SKILL.md"}},
		[]Skill{{Name: "k8s-core", Path: "/tmp/k8s-core/SKILL.md", Custom: true}},
		false,
	)
	if err == nil {
		t.Fatalf("expected duplicate skill error")
	}
}

func TestMergeAllowsCustomOverride(t *testing.T) {
	merged, err := Merge(
		[]Skill{{Name: "k8s-core", Path: "skills/claude/k8s-core/SKILL.md"}},
		[]Skill{{Name: "k8s-core", Path: "/tmp/k8s-core/SKILL.md", Custom: true}},
		true,
	)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 skill after override, got %d", len(merged))
	}
	if !merged[0].Custom {
		t.Fatalf("expected custom skill to override built-in")
	}
}

func writeCustomSkill(t *testing.T, root string, name string, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir custom skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write custom skill: %v", err)
	}
}
