package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"rootcause/internal/skills/catalog"
)

type skillFormat string

const (
	formatSkillMD skillFormat = "skill_md"
	formatMDC     skillFormat = "mdc"
	formatMD      skillFormat = "markdown"
)

type agentTarget struct {
	Agent  string
	Dir    string
	Format skillFormat
}

var agentTargets = map[string]agentTarget{
	"claude":           {Agent: "Claude Code", Dir: ".claude/skills", Format: formatSkillMD},
	"cursor":           {Agent: "Cursor", Dir: ".cursor/skills", Format: formatMDC},
	"codex":            {Agent: "Codex", Dir: ".codex/skills", Format: formatSkillMD},
	"gemini":           {Agent: "Gemini CLI", Dir: ".gemini/skills", Format: formatSkillMD},
	"gemini-cli":       {Agent: "Gemini CLI", Dir: ".gemini/skills", Format: formatSkillMD},
	"opencode":         {Agent: "OpenCode", Dir: ".opencode/skills", Format: formatSkillMD},
	"copilot":          {Agent: "GitHub Copilot", Dir: ".github/skills", Format: formatMD},
	"github-copilot":   {Agent: "GitHub Copilot", Dir: ".github/skills", Format: formatMD},
	"windsurf":         {Agent: "Windsurf", Dir: ".windsurf/skills", Format: formatMD},
	"devin":            {Agent: "Devin", Dir: ".devin/skills", Format: formatMD},
	"aider":            {Agent: "Aider", Dir: ".aider/skills", Format: formatSkillMD},
	"cody":             {Agent: "Sourcegraph Cody", Dir: ".cody/skills", Format: formatSkillMD},
	"sourcegraph-cody": {Agent: "Sourcegraph Cody", Dir: ".cody/skills", Format: formatSkillMD},
	"amazonq":          {Agent: "Amazon Q", Dir: ".amazonq/skills", Format: formatSkillMD},
	"amazon-q":         {Agent: "Amazon Q", Dir: ".amazonq/skills", Format: formatSkillMD},
}

var canonicalAgentKeys = []string{
	"claude",
	"cursor",
	"codex",
	"gemini",
	"opencode",
	"copilot",
	"windsurf",
	"devin",
	"aider",
	"cody",
	"amazonq",
}

func newSyncSkillsCmd(stderr io.Writer) *cobra.Command {
	var agent string
	var projectDir string
	var sourceDir string
	var overwrite bool
	var listOnly bool
	var listSkills bool
	var allAgents bool
	var dryRun bool
	var skillFilters []string

	cmd := &cobra.Command{
		Use:   "sync-skills",
		Short: "Sync skills into agent-specific project directories",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if listOnly || listSkills {
				return nil
			}
			if !allAgents && strings.TrimSpace(agent) == "" {
				return fmt.Errorf("--agent is required unless --all-agents, --list-agents, or --list-skills is set")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if listOnly {
				return listAgentTargets(stderr)
			}
			manifest, err := catalog.Load()
			if err != nil {
				return err
			}
			if listSkills {
				return listSkillsCatalog(stderr, manifest)
			}
			skills, err := selectedSkills(manifest, skillFilters)
			if err != nil {
				return err
			}
			source := sourceDir
			if !filepath.IsAbs(source) {
				source = filepath.Join(projectDir, source)
			}
			targetKeys := []string{strings.ToLower(strings.TrimSpace(agent))}
			if allAgents {
				targetKeys = append([]string{}, canonicalAgentKeys...)
			}
			total := 0
			for _, key := range targetKeys {
				target, ok := agentTargets[key]
				if !ok {
					return fmt.Errorf("unsupported agent %q; use --list-agents to view supported values", key)
				}
				count, dest, err := syncSkillsForTarget(source, projectDir, target, skills, overwrite, dryRun)
				if err != nil {
					return err
				}
				total += count
				if stderr == nil {
					stderr = os.Stdout
				}
				action := "Synced"
				if dryRun {
					action = "Would sync"
				}
				_, _ = fmt.Fprintf(stderr, "%s %d skill(s) for %s into %s\n", action, count, target.Agent, dest)
			}
			if stderr == nil {
				stderr = os.Stdout
			}
			if !dryRun {
				_, _ = fmt.Fprintf(stderr, "Total synced skill copies: %d\n", total)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "target agent: claude|cursor|codex|gemini|opencode|copilot|windsurf|devin|aider|cody|amazonq")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory root")
	cmd.Flags().StringVar(&sourceDir, "source", "skills/claude", "source skills directory")
	cmd.Flags().BoolVar(&overwrite, "overwrite", true, "overwrite existing files")
	cmd.Flags().BoolVar(&listOnly, "list-agents", false, "list supported agent targets and exit")
	cmd.Flags().BoolVar(&listSkills, "list-skills", false, "list embedded skills catalog and exit")
	cmd.Flags().BoolVar(&allAgents, "all-agents", false, "sync skills to all supported agents")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned copies without writing files")
	cmd.Flags().StringSliceVar(&skillFilters, "skill", nil, "sync only selected skill name(s); can be repeated")

	return cmd
}

func listAgentTargets(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	keys := append([]string{}, canonicalAgentKeys...)
	for _, key := range keys {
		t := agentTargets[key]
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", key, t.Agent, t.Dir)
	}
	return nil
}

func syncSkillsForTarget(sourceDir, projectDir string, target agentTarget, skills []catalog.Skill, overwrite bool, dryRun bool) (int, string, error) {
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return 0, "", err
	}
	destRoot := filepath.Join(absProject, filepath.FromSlash(target.Dir))
	if !dryRun {
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return 0, "", fmt.Errorf("create destination dir: %w", err)
		}
	}
	count := 0
	for _, skill := range skills {
		srcFile := filepath.Join(sourceDir, filepath.Base(catalog.SkillDir(skill)), "SKILL.md")
		data, err := os.ReadFile(srcFile)
		if err != nil {
			if os.IsNotExist(err) {
				return count, destRoot, fmt.Errorf("missing source skill file: %s", srcFile)
			}
			return count, destRoot, fmt.Errorf("read %s: %w", srcFile, err)
		}
		destFile := targetFilePath(destRoot, skill.Name, target.Format)
		if !dryRun {
			if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
				return count, destRoot, err
			}
			if !overwrite {
				if _, err := os.Stat(destFile); err == nil {
					continue
				}
			}
			if err := os.WriteFile(destFile, data, 0o644); err != nil {
				return count, destRoot, fmt.Errorf("write %s: %w", destFile, err)
			}
		}
		count++
	}
	return count, destRoot, nil
}

func listSkillsCatalog(w io.Writer, manifest catalog.Manifest) error {
	if w == nil {
		w = os.Stdout
	}
	byCategory := catalog.ByCategory(manifest)
	for _, cat := range catalog.Categories(manifest) {
		_, _ = fmt.Fprintf(w, "%s:\n", cat)
		for _, skill := range byCategory[cat] {
			_, _ = fmt.Fprintf(w, "  - %s\t%s\n", skill.Name, skill.Description)
		}
	}
	return nil
}

func selectedSkills(manifest catalog.Manifest, filters []string) ([]catalog.Skill, error) {
	if len(filters) == 0 {
		out := append([]catalog.Skill{}, manifest.Skills...)
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	}
	allowed := map[string]struct{}{}
	for _, f := range filters {
		trimmed := strings.TrimSpace(strings.ToLower(f))
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	var out []catalog.Skill
	for _, s := range manifest.Skills {
		if _, ok := allowed[strings.ToLower(s.Name)]; ok {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no matching skills for filters: %v", filters)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func targetFilePath(destRoot, skillName string, format skillFormat) string {
	switch format {
	case formatMDC:
		return filepath.Join(destRoot, skillName+".mdc")
	case formatMD:
		return filepath.Join(destRoot, skillName+".md")
	default:
		return filepath.Join(destRoot, skillName, "SKILL.md")
	}
}
