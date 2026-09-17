package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed SKILL.md
var skillMD string

//go:embed CHECK_SKILL.md
var checkSkillMD string

func newSkillCmd() *cobra.Command {
	var stdout bool
	var agent string

	cmd := &cobra.Command{
		Use:   "add-skill [browser|check]",
		Short: "Install a Vibium skill for a coding agent",
		Example: `  vibium add-skill
  # Installs skill to ~/.claude/skills/browser/

  vibium add-skill --agent grok
  # Installs skill to ~/.grok/skills/browser/

  vibium add-skill check --agent grok
  # Installs skill to ~/.grok/skills/check/

  vibium add-skill check --stdout
  # Print skill content to stdout`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"browser", "check"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "browser"
			if len(args) == 1 {
				name = args[0]
			}
			content := skillMD
			switch name {
			case "browser":
			case "check":
				content = checkSkillMD
			default:
				return fmt.Errorf("unknown skill %q; choose browser or check", name)
			}
			if _, err := skillDirForAgent(agent); err != nil {
				return err
			}
			if stdout {
				fmt.Fprint(cmd.OutOrStdout(), content)
				return nil
			}
			return installSkill(name, agent)
		},
	}
	cmd.Flags().BoolVar(&stdout, "stdout", false, "Print skill content to stdout instead of installing")
	cmd.Flags().StringVar(&agent, "agent", "claude", "Agent skill directory: claude or grok")
	return cmd
}

func skillDirForAgent(agent string) (string, error) {
	switch agent {
	case "claude":
		return filepath.Join(".claude", "skills"), nil
	case "grok":
		return filepath.Join(".grok", "skills"), nil
	default:
		return "", fmt.Errorf("unknown agent %q; choose claude or grok", agent)
	}
}

func writeSkill(name, agent string) (string, string, error) {
	content := skillMD
	switch name {
	case "browser":
	case "check":
		content = checkSkillMD
	default:
		return "", "", fmt.Errorf("unknown skill %q; choose browser or check", name)
	}
	rel, err := skillDirForAgent(agent)
	if err != nil {
		return "", "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("could not find home directory: %w", err)
	}
	skillDir := filepath.Join(home, rel, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", "", fmt.Errorf("could not create skill directory: %w", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("could not write SKILL.md: %w", err)
	}
	return skillDir, skillPath, nil
}

func installSkill(name, agent string) error {
	skillDir, skillPath, err := writeSkill(name, agent)
	if err != nil {
		return err
	}

	if jsonOutput {
		printJSON(jsonEnvelope{OK: true, Result: map[string]interface{}{
			"skill": name,
			"agent": agent,
			"dir":   skillDir,
			"files": []string{skillPath},
		}})
		return nil
	}

	fmt.Printf("Installed Vibium skill to %s\n", skillDir)
	fmt.Println("Files:")
	fmt.Printf("  %s\n", skillPath)
	return nil
}
