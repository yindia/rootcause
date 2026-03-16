package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

	cmd := &cobra.Command{
		Use:   "sync-skills",
		Short: "Sync skills into agent-specific project directories",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if listOnly {
				return nil
			}
			if strings.TrimSpace(agent) == "" {
				return fmt.Errorf("--agent is required unless --list-agents is set")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if listOnly {
				return listAgentTargets(stderr)
			}
			source := sourceDir
			if !filepath.IsAbs(source) {
				source = filepath.Join(projectDir, source)
			}
			agentKey := strings.ToLower(strings.TrimSpace(agent))
			target, ok := agentTargets[agentKey]
			if !ok {
				return fmt.Errorf("unsupported agent %q; use --list-agents to view supported values", agent)
			}
			count, dest, err := syncSkillsForTarget(source, projectDir, target, overwrite)
			if err != nil {
				return err
			}
			if stderr == nil {
				stderr = os.Stdout
			}
			_, _ = fmt.Fprintf(stderr, "Synced %d skill(s) for %s into %s\n", count, target.Agent, dest)
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "target agent: claude|cursor|codex|gemini|opencode|copilot|windsurf|devin|aider|cody|amazonq")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory root")
	cmd.Flags().StringVar(&sourceDir, "source", "skills/claude", "source skills directory")
	cmd.Flags().BoolVar(&overwrite, "overwrite", true, "overwrite existing files")
	cmd.Flags().BoolVar(&listOnly, "list-agents", false, "list supported agent targets and exit")

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

func syncSkillsForTarget(sourceDir, projectDir string, target agentTarget, overwrite bool) (int, string, error) {
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return 0, "", err
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return 0, "", fmt.Errorf("read source skills: %w", err)
	}
	destRoot := filepath.Join(absProject, filepath.FromSlash(target.Dir))
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return 0, "", fmt.Errorf("create destination dir: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		srcFile := filepath.Join(sourceDir, skillName, "SKILL.md")
		data, err := os.ReadFile(srcFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return count, destRoot, fmt.Errorf("read %s: %w", srcFile, err)
		}
		destFile := targetFilePath(destRoot, skillName, target.Format)
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
		count++
	}
	return count, destRoot, nil
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
