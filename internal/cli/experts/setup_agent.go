/**
 * Component: Experts Setup Agent Command
 * Block-UUID: fa831317-0f9f-45d5-a633-dfed0ca5348a
 * Parent-UUID: 00c1f877-749e-444d-8246-39a94c291d09
 * Version: 1.0.3
 * Description: Implements the 'gsc experts setup-agent' command. Updated to require the agent name as a positional argument to avoid defaulting to specific vendors.
 * Language: Go
 * Created-at: 2026-05-18T15:24:38.774Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.0.3)
 */


package experts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	// claudeSkillName is the filename for the Claude Code skill.
	claudeSkillName = "gitsense.md"
	
	// claudeSkillContent is the markdown content of the skill.
	claudeSkillContent = `I will now initialize the GitSense Expert context for this repository.

1. Execute this exact command to generate the context:
   gsc experts init 2>&1

2. If the command above succeeds, execute this exact command to load the expertise:
   cat .gitsense/experts-context.md

Note: Do not substitute 'go run' for 'gsc'. Do not attempt to guess command names. 
Follow the instructions in the context file once loaded.
`
)

// NewSetupAgentCmd creates and returns the 'gsc experts setup-agent' command.
func NewSetupAgentCmd() *cobra.Command {
	var flags SetupAgentFlags

	cmd := &cobra.Command{
		Use:   "setup-agent <agent>",
		Short: "Install the /gsc-experts skill for your AI agent",
		Long: `Installs a global skill (e.g., /gitsense for Claude Code) that simplifies
the process of initializing the GitSense Expert context in any repository.

This command writes a skill file to the agent's configuration directory.

Supported agents:
  - claude`,
		SilenceUsage: true,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupAgent(&flags, args[0])
		},
	}

	AddSetupAgentFlags(cmd, &flags)
	return cmd
}

// runSetupAgent executes the logic for the setup-agent command.
func runSetupAgent(flags *SetupAgentFlags, agent string) error {
	var targetDir string
	var skillName string
	var content string

	// Resolve agent-specific paths
	switch agent {
	case "claude":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve home directory: %w", err)
		}
		targetDir = filepath.Join(homeDir, ".claude", "commands")
		skillName = claudeSkillName
		content = claudeSkillContent
	default:
		return fmt.Errorf("unsupported agent: %s. Currently only 'claude' is supported", agent)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent config directory: %w", err)
	}

	skillPath := filepath.Join(targetDir, skillName)

	// Check if file exists
	if _, err := os.Stat(skillPath); err == nil {
		if !flags.Force {
			fmt.Printf("⚠️  Skill file already exists at: %s\n", skillPath)
			fmt.Println("Use --force to overwrite.")
			return nil
		}
	}

	// Write the skill file
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	fmt.Printf("✅ Successfully installed /%s skill for %s\n", skillName[:len(skillName)-3], agent)
	fmt.Printf("   Location: %s\n", skillPath)
	fmt.Println("\nYou can now run '/gitsense' in your agent to initialize the context.")

	return nil
}
