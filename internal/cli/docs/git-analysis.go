/**
 * Component: Docs Git Analysis Command
 * Block-UUID: 7f4c9e3d-2a6b-4c8d-9e1f-5a6b7c8d9e0f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs git-analysis' that displays the guide for managing AI-generated metadata across branches, rebuilds, and worktrees.
 * Language: Go
 * Created-at: 2026-05-31T17:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewGitAnalysisCmd creates and returns the 'gsc docs git-analysis' subcommand.
func NewGitAnalysisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git-analysis",
		Short: "Preserve and transfer AI-generated metadata across branches and worktrees",
		Long: `Displays the git analysis management guide that covers dump, load, and copy
commands for preserving AI-generated metadata across branches, rebuilds, and worktrees.
The primary use case is the agentic worktree workflow.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("git-analysis")
		},
	}

	return cmd
}
