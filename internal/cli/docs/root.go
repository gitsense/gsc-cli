/**
 * Component: Docs Root Command
 * Block-UUID: 74db9fcd-42fd-4b5b-bfb9-05eb29358613
 * Parent-UUID: 2042a278-0995-4592-808b-e1aa06c9d491
 * Version: 1.5.0
 * Description: Defines the root command for the 'gsc docs' command group. Provides AI-optimized documentation that serves as a behavioral contract between the CLI and AI agents. Added quickstart, git-analysis, brains, and experts command registrations.
 * Language: Go
 * Created-at: 2026-05-31T14:47:50.284Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), DeepSeek V4 Pro (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewCmd creates and returns the root command for the 'gsc docs' group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Access AI-optimized documentation for GitSense Chat",
		Long: `The 'gsc docs' command provides AI-optimized documentation that serves as a 
behavioral contract between the CLI and AI agents. Each document includes embedded 
"LLM Guidance" that instructs the agent on tone, behavior, and next steps.

This documentation is designed to be consumed by an AI agent within an active chat 
session. Use 'gsc docs help' to see the roadmap of available topics (also available 
as 'gsc docs init').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register subcommands for each documentation topic
	cmd.AddCommand(NewHelpCmd())
	cmd.AddCommand(NewInitCmd())
	cmd.AddCommand(NewAboutCmd())
	cmd.AddCommand(NewInstallCmd())
	cmd.AddCommand(NewAdminCmd())
	cmd.AddCommand(NewLifecycleCmd())
	cmd.AddCommand(NewImportGitCmd())
	cmd.AddCommand(NewLocateCmd())
	cmd.AddCommand(NewQuickstartCmd())
	cmd.AddCommand(NewGitAnalysisCmd())
	cmd.AddCommand(NewBrainsCmd())
	cmd.AddCommand(NewExpertsCmd())

	return cmd
}
