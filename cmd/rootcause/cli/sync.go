package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	rccfg "rootcause/internal/config"
	rcmcp "rootcause/internal/mcp"
	"rootcause/internal/skills/catalog"
)

// newSyncCmd is the single user-facing command for syncing custom prompts and
// skills into agent-native directories. It replaces the older separate
// `sync-skills` and `sync-commands` commands.
func newSyncCmd(stderr io.Writer) *cobra.Command {
	var (
		agent         string
		allAgents     bool
		user          bool
		projectDir    string
		promptsOnly   bool
		skillsOnly    bool
		listAgents    bool
		listAll       bool
		promptFilters []string
		skillFilters  []string
		overwrite     bool
		builtinOnly   bool
		dryRun        bool
		configPath    string
		customDirs    []string
		allowOverride bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync custom prompts and skills into agent-native directories",
		Long: `Sync prompts (as slash commands) and skills into your AI client's native
directories. By default both surfaces are synced; pass --prompts-only or
--skills-only for granular control.

Per-agent target directories (the ones agents that support both surfaces):
  claude  -> .claude/commands/        + .claude/skills/
  cursor  -> .cursor/commands/        + .cursor/skills/
  codex   -> .codex/commands/         + .codex/skills/
  copilot -> .github/prompts/         + .github/skills/
  gemini  -> .gemini/commands/        + .gemini/skills/
  opencode-> .opencode/commands/      + .opencode/skills/
  windsurf-> .windsurf/commands/      + .windsurf/skills/
  aider   -> .aider/commands/         + .aider/skills/

Skills-only agents (no slash-command directory yet): devin, cody, amazonq.

By default, custom prompts/skills from your ~/.rootcause/ directories are
included. Use --builtin-only to sync only ships-with-the-binary content.
Existing files are NOT overwritten unless --overwrite is passed.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if listAgents || listAll {
				return nil
			}
			if !allAgents && strings.TrimSpace(agent) == "" {
				return fmt.Errorf("--agent is required unless --all-agents, --list-agents, or --list is set")
			}
			if promptsOnly && skillsOnly {
				return fmt.Errorf("--prompts-only and --skills-only are mutually exclusive")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			out := stderr
			if out == nil {
				out = os.Stdout
			}

			if listAgents {
				return listUnifiedAgentTargets(out)
			}

			if user {
				projectDir = "~"
			}
			includeCustom := !builtinOnly

			// Load specs / manifest up-front so --list and --dry-run share the
			// same source of truth as a real sync.
			ctx, err := buildToolContextForSync(configPath)
			if err != nil {
				return err
			}

			var promptSpecs []rcmcp.PromptSpec
			if !skillsOnly {
				if includeCustom {
					promptSpecs, err = rcmcp.LoadPromptSpecsForCLI(ctx)
				} else {
					promptSpecs = rcmcp.BuiltinPromptSpecs()
				}
				if err != nil {
					return err
				}
				promptSpecs, err = selectedPrompts(promptSpecs, promptFilters)
				if err != nil {
					return err
				}
			}

			var skillManifest catalog.Manifest
			var selectedSkillsList []catalog.Skill
			if !promptsOnly {
				skillManifest, err = loadSkillManifest(includeCustom || len(customDirs) > 0, customDirs, allowOverride)
				if err != nil {
					return err
				}
				selectedSkillsList, err = selectedSkills(skillManifest, skillFilters)
				if err != nil {
					return err
				}
			}

			if listAll {
				return listUnifiedCatalog(out, promptSpecs, skillManifest, promptsOnly, skillsOnly)
			}

			targetKeys := []string{strings.ToLower(strings.TrimSpace(agent))}
			if allAgents {
				targetKeys = unifiedAgentKeys()
			}

			totalPrompts := 0
			totalSkills := 0
			actionVerb := "Synced"
			if dryRun {
				actionVerb = "Would sync"
			}

			for _, key := range targetKeys {
				promptTarget, promptOK := commandTargets[key]
				skillTarget, skillOK := agentTargets[key]
				if !promptOK && !skillOK {
					return fmt.Errorf("unsupported agent %q; use --list-agents to view supported values", key)
				}

				if !skillsOnly && promptOK && len(promptSpecs) > 0 {
					count, dest, err := syncCommandsForTarget(projectDir, promptTarget, promptSpecs, overwrite, dryRun)
					if err != nil {
						return err
					}
					totalPrompts += count
					fmt.Fprintf(out, "%s %d slash command(s) for %s into %s\n", actionVerb, count, promptTarget.Agent, dest)
				} else if !skillsOnly && !promptOK {
					fmt.Fprintf(out, "Skipping prompts for %s (no slash command directory mapping)\n", key)
				}

				if !promptsOnly && skillOK && len(selectedSkillsList) > 0 {
					count, dest, err := syncSkillsForTarget("skills/claude", projectDir, skillTarget, selectedSkillsList, overwrite, dryRun)
					if err != nil {
						return err
					}
					totalSkills += count
					fmt.Fprintf(out, "%s %d skill(s) for %s into %s\n", actionVerb, count, skillTarget.Agent, dest)
				} else if !promptsOnly && !skillOK {
					fmt.Fprintf(out, "Skipping skills for %s (no skill directory mapping)\n", key)
				}
			}

			if !dryRun {
				fmt.Fprintf(out, "Total: %d slash command(s), %d skill(s)\n", totalPrompts, totalSkills)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "target agent (see --list-agents)")
	cmd.Flags().BoolVar(&allAgents, "all-agents", false, "sync to every agent that supports both surfaces")
	cmd.Flags().BoolVar(&user, "user", false, "install user-globally (shortcut for --project-dir ~)")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory root (use '~' for user-globally)")
	cmd.Flags().BoolVar(&promptsOnly, "prompts-only", false, "only sync prompts (skip skills)")
	cmd.Flags().BoolVar(&skillsOnly, "skills-only", false, "only sync skills (skip prompts)")
	cmd.Flags().BoolVar(&listAgents, "list-agents", false, "list supported agent targets and exit")
	cmd.Flags().BoolVar(&listAll, "list", false, "list available prompts and skills and exit")
	cmd.Flags().StringSliceVar(&promptFilters, "prompt", nil, "sync only selected prompt name(s); can be repeated")
	cmd.Flags().StringSliceVar(&skillFilters, "skill", nil, "sync only selected skill name(s); can be repeated")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing files (default: skip files that already exist)")
	cmd.Flags().BoolVar(&builtinOnly, "builtin-only", false, "only sync built-in prompts and skills, ignore custom directories")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned writes without touching disk")
	cmd.Flags().StringVar(&configPath, "config", "", "config file path (used to resolve [prompts].dir / [skills].custom_dirs)")
	cmd.Flags().StringSliceVar(&customDirs, "custom-skill-dir", nil, "additional custom skills directory (in addition to ~/.rootcause/skills); can be repeated")
	cmd.Flags().BoolVar(&allowOverride, "allow-skill-overrides", false, "allow custom skills to override built-in skill names")

	return cmd
}

// buildToolContextForSync loads the config file so directory fields like
// [prompts].dir and [skills].custom_dirs are honored by the downstream
// loaders. Resolution order matches the server: --config flag, then
// ROOTCAUSE_CONFIG env, then standard paths (~/.rootcause/config.yaml,
// ~/.config/rootcause/config.yaml, ./config.yaml). When no config is
// discoverable the loaders fall back to env vars and default filesystem
// paths.
func buildToolContextForSync(configPath string) (rcmcp.ToolContext, error) {
	resolved := resolveSyncConfigPath(configPath)
	if resolved == "" {
		return rcmcp.ToolContext{}, nil
	}
	cfg, err := rccfg.Load(resolved, "", rccfg.Overrides{})
	if err != nil {
		return rcmcp.ToolContext{}, fmt.Errorf("load config %s: %w", resolved, err)
	}
	return rcmcp.ToolContext{Config: &cfg}, nil
}

// resolveSyncConfigPath finds the first existing config path. Returns "" when
// none exists — callers should fall back to env / default search behavior.
func resolveSyncConfigPath(explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	if env := strings.TrimSpace(os.Getenv("ROOTCAUSE_CONFIG")); env != "" {
		return env
	}
	for _, candidate := range []string{
		"~/.rootcause/config.yaml",
		"~/.config/rootcause/config.yaml",
		"./config.yaml",
	} {
		expanded, _ := expandPromptHome(candidate)
		if expanded == "" {
			continue
		}
		if info, err := os.Stat(expanded); err == nil && !info.IsDir() {
			return expanded
		}
	}
	return ""
}

// unifiedAgentKeys returns the intersection of agents that support both prompt
// and skill targets. --all-agents iterates this list so we don't get partial
// half-synced setups across the board.
func unifiedAgentKeys() []string {
	keys := make([]string, 0)
	for _, k := range canonicalCommandAgentKeys {
		if _, ok := agentTargets[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func listUnifiedAgentTargets(w io.Writer) error {
	fmt.Fprintf(w, "AGENT\tNAME\tCOMMANDS\tSKILLS\n")
	allKeys := map[string]struct{}{}
	for k := range commandTargets {
		allKeys[k] = struct{}{}
	}
	for k := range agentTargets {
		allKeys[k] = struct{}{}
	}
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ct, hasCmd := commandTargets[key]
		st, hasSkill := agentTargets[key]
		cmdDir := "-"
		if hasCmd {
			cmdDir = ct.Dir
		}
		skillDir := "-"
		if hasSkill {
			skillDir = st.Dir
		}
		name := key
		if hasCmd {
			name = ct.Agent
		} else if hasSkill {
			name = st.Agent
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key, name, cmdDir, skillDir)
	}
	return nil
}

func listUnifiedCatalog(w io.Writer, prompts []rcmcp.PromptSpec, skills catalog.Manifest, promptsOnly, skillsOnly bool) error {
	if !skillsOnly {
		fmt.Fprintln(w, "Prompts:")
		sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
		for _, p := range prompts {
			fmt.Fprintf(w, "  - %s\t%s\n", p.Name, p.Description)
		}
	}
	if !promptsOnly {
		fmt.Fprintln(w, "Skills:")
		byCategory := catalog.ByCategory(skills)
		for _, cat := range catalog.Categories(skills) {
			fmt.Fprintf(w, "  %s:\n", cat)
			for _, s := range byCategory[cat] {
				fmt.Fprintf(w, "    - %s\t%s\n", s.Name, s.Description)
			}
		}
	}
	return nil
}
