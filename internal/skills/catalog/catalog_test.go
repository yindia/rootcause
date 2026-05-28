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

func TestDiscoverCustomSkillsFallsBackToTitleAndDefaultCategory(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "local-runbook", "# Local Runbook\n\nUse local steps.\n")

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 custom skill, got %d", len(skills))
	}
	if skills[0].Category != "Custom" {
		t.Fatalf("expected default custom category, got %q", skills[0].Category)
	}
	if skills[0].Description != "Local Runbook" {
		t.Fatalf("expected title-derived description, got %q", skills[0].Description)
	}
	if len(skills[0].Tags) != 0 {
		t.Fatalf("expected no tags, got %#v", skills[0].Tags)
	}
}

func TestDiscoverCustomSkillsNormalizesAndDeduplicatesTags(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "tagged-runbook", "---\ntags: [' RootCause ', rootcause, PAYMENTS, '']\n---\n# Tagged\n")

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 custom skill, got %d", len(skills))
	}
	expected := []string{"payments", "rootcause"}
	if strings.Join(skills[0].Tags, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected normalized tags %#v, got %#v", expected, skills[0].Tags)
	}
}

func TestDiscoverCustomSkillsMalformedFrontMatterUsesBodyTitle(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "malformed-runbook", "---\ntags: [broken\n---\n# Body Title\n")

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 custom skill, got %d", len(skills))
	}
	if skills[0].Description != "Body Title" {
		t.Fatalf("expected body title fallback, got %q", skills[0].Description)
	}
	if len(skills[0].Tags) != 0 {
		t.Fatalf("expected malformed tags to be ignored, got %#v", skills[0].Tags)
	}
}

func TestDiscoverCustomSkillsFallbackDescriptionWithoutTitle(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "plain-runbook", "Use this runbook without a markdown title.\n")

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 custom skill, got %d", len(skills))
	}
	if !strings.Contains(skills[0].Description, filepath.Join("plain-runbook", "SKILL.md")) {
		t.Fatalf("expected file-based fallback description, got %q", skills[0].Description)
	}
}

func TestDiscoverCustomSkillsIgnoresFilesAndBlankDirs(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a skill"), 0o600)
	if err != nil {
		t.Fatalf("write non-skill file: %v", err)
	}
	writeCustomSkill(t, dir, "team-runbook", "# Team Runbook\n")

	skills, err := DiscoverCustom([]string{" ", dir})
	if err != nil {
		t.Fatalf("DiscoverCustom: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "team-runbook" {
		t.Fatalf("expected only directory-backed custom skill, got %#v", skills)
	}
}

func TestDiscoverCustomSkillsSkipsDuplicateAcrossDirs(t *testing.T) {
	// A duplicate custom skill name across directories is a warning, not an
	// error: the first occurrence wins and the second is skipped silently so
	// one misconfigured directory doesn't poison every tool call.
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	writeCustomSkill(t, firstDir, "team-runbook", "# First\n")
	writeCustomSkill(t, secondDir, "team-runbook", "# Second\n")

	skills, err := DiscoverCustom([]string{firstDir, secondDir})
	if err != nil {
		t.Fatalf("DiscoverCustom must not hard-fail on duplicate names: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected the first occurrence to win, got %d skills", len(skills))
	}
}

func TestLoadWithCustomIncludesConfiguredCustomSkills(t *testing.T) {
	dir := t.TempDir()
	writeCustomSkill(t, dir, "zz-team-runbook", "---\ntags: [rootcause]\n---\n# Team Runbook\n")

	manifest, err := LoadWithCustom(CustomOptions{Dirs: []string{dir}})
	if err != nil {
		t.Fatalf("LoadWithCustom: %v", err)
	}
	var found Skill
	for _, skill := range manifest.Skills {
		if skill.Name == "zz-team-runbook" {
			found = skill
			break
		}
	}
	if found.Name == "" || !found.Custom {
		t.Fatalf("expected custom skill in merged manifest, got %#v", found)
	}
	if len(found.Tags) != 1 || found.Tags[0] != "rootcause" {
		t.Fatalf("expected custom tags in merged manifest, got %#v", found.Tags)
	}
}

func TestEmbeddedManifestIncludesGCPSkill(t *testing.T) {
	manifest, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	var found Skill
	for _, s := range manifest.Skills {
		if s.Name == "k8s-observability" {
			found = s
			break
		}
	}
	if found.Name == "" {
		t.Fatalf("expected k8s-observability skill in embedded manifest")
	}
	if found.Category != "Cloud Observability" {
		t.Errorf("expected category 'Cloud Observability', got %q", found.Category)
	}
	if found.Path != "skills/claude/k8s-observability/SKILL.md" {
		t.Errorf("unexpected path: %s", found.Path)
	}
}

func TestCatalogHelpersCoverCustomAndBuiltInSkills(t *testing.T) {
	manifest := Manifest{Skills: []Skill{
		{Name: "beta", Category: "Ops", Path: "skills/claude/beta/SKILL.md"},
		{Name: "alpha", Category: "Ops", Path: "/tmp/alpha/SKILL.md", Custom: true},
		{Name: "gamma", Category: "RCA", Path: "skills/claude/gamma/SKILL.md"},
	}}

	byCategory := ByCategory(manifest)
	if len(byCategory["Ops"]) != 2 || byCategory["Ops"][0].Name != "alpha" || byCategory["Ops"][1].Name != "beta" {
		t.Fatalf("expected category groups sorted by skill name, got %#v", byCategory["Ops"])
	}
	categories := Categories(manifest)
	if strings.Join(categories, ",") != "Ops,RCA" {
		t.Fatalf("expected sorted categories, got %#v", categories)
	}
	if SkillDir(manifest.Skills[0]) != filepath.Join("skills", "claude", "beta") {
		t.Fatalf("unexpected built-in skill dir: %s", SkillDir(manifest.Skills[0]))
	}
	if SkillFile("skills/claude", manifest.Skills[0]) != filepath.Join("skills", "claude", "beta", "SKILL.md") {
		t.Fatalf("unexpected built-in skill file: %s", SkillFile("skills/claude", manifest.Skills[0]))
	}
	if SkillFile("skills/claude", manifest.Skills[1]) != "/tmp/alpha/SKILL.md" {
		t.Fatalf("unexpected custom skill file: %s", SkillFile("skills/claude", manifest.Skills[1]))
	}
}

func TestValidateRejectsInvalidManifests(t *testing.T) {
	cases := []struct {
		name     string
		manifest Manifest
		want     string
	}{
		{name: "missing schema", manifest: Manifest{Skills: []Skill{{Name: "skill", Path: "path"}}}, want: "schemaVersion"},
		{name: "no skills", manifest: Manifest{SchemaVersion: "v1"}, want: "no skills"},
		{name: "missing path", manifest: Manifest{SchemaVersion: "v1", Skills: []Skill{{Name: "skill"}}}, want: "name/path"},
		{name: "duplicate", manifest: Manifest{SchemaVersion: "v1", Skills: []Skill{{Name: "skill", Path: "a"}, {Name: "skill", Path: "b"}}}, want: "duplicate"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validate(testCase.manifest)
			if err == nil {
				t.Fatalf("expected validation error containing %q", testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("expected validation error containing %q, got %v", testCase.want, err)
			}
		})
	}
}

func TestResolvePathExpandsEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOTCAUSE_TEST_SKILLS_DIR", dir)

	resolved, err := resolvePath("$ROOTCAUSE_TEST_SKILLS_DIR")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if resolved != dir {
		t.Fatalf("expected env-expanded path %q, got %q", dir, resolved)
	}
}

func TestMergeRejectsDuplicateBuiltInSkill(t *testing.T) {
	_, err := Merge(
		[]Skill{{Name: "k8s-core", Path: "a"}, {Name: "k8s-core", Path: "b"}},
		nil,
		false,
	)
	if err == nil {
		t.Fatalf("expected duplicate built-in skill error")
	}
	if !strings.Contains(err.Error(), "duplicate built-in") {
		t.Fatalf("expected duplicate built-in error, got %v", err)
	}
}

func TestDiscoverCustomSkillsSkipsSubdirMissingSkillFile(t *testing.T) {
	dir := t.TempDir()
	// "broken" is missing SKILL.md — must be skipped, not fatal.
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatalf("mkdir broken skill: %v", err)
	}
	// "good" is valid — must still load alongside the skipped one.
	good := filepath.Join(dir, "good")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatalf("mkdir good skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(good, "SKILL.md"), []byte("---\ntags: [test]\n---\n# Good\n"), 0o644); err != nil {
		t.Fatalf("write good SKILL.md: %v", err)
	}

	skills, err := DiscoverCustom([]string{dir})
	if err != nil {
		t.Fatalf("DiscoverCustom must not hard-fail on bad subdir: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "good" {
		t.Fatalf("expected only the valid skill to load, got %#v", skills)
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
	err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write custom skill: %v", err)
	}
}
