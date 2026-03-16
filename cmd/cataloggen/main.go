package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rootcause/internal/skills/catalog"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	m, err := catalog.Load()
	if err != nil {
		return err
	}
	for _, skill := range m.Skills {
		if _, err := os.Stat(skill.Path); err != nil {
			return fmt.Errorf("skills manifest path invalid for %s: %w", skill.Name, err)
		}
	}
	out := buildSkillsCatalogMarkdown(m)
	if err := os.WriteFile(filepath.Join("skills", "CATALOG.md"), []byte(out), 0o644); err != nil {
		return fmt.Errorf("write skills/CATALOG.md: %w", err)
	}
	return nil
}

func buildSkillsCatalogMarkdown(m catalog.Manifest) string {
	var b strings.Builder
	b.WriteString("# Skills Catalog\n\n")
	b.WriteString("Auto-generated from `internal/skills/catalog/manifest.json`.\n")
	b.WriteString("Do not edit manually; run `go run ./cmd/cataloggen`.\n\n")
	b.WriteString(fmt.Sprintf("Schema: `%s` | Catalog version: `%s` | Total skills: `%d`\n\n", m.SchemaVersion, m.Version, len(m.Skills)))

	byCategory := catalog.ByCategory(m)
	for _, category := range catalog.Categories(m) {
		b.WriteString("## " + category + "\n\n")
		b.WriteString("| Skill | Description | Source Path |\n")
		b.WriteString("|---|---|---|\n")
		for _, skill := range byCategory[category] {
			b.WriteString(fmt.Sprintf("| `%s` | %s | `%s` |\n", skill.Name, skill.Description, skill.Path))
		}
		b.WriteString("\n")
	}
	return b.String()
}
