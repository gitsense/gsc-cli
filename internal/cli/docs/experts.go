/**
 * Component: Docs Experts Command
 * Block-UUID: 5a9e8d7c-4c3a-5b2f-9e1d-7c8d9e0f1a2b
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs experts' that displays the guide for connecting coding agents to the Brain intelligence layer.
 * Language: Go
 * Created-at: 2026-05-31T17:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewExpertsCmd creates and returns the 'gsc docs experts' subcommand.
func NewExpertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experts",
		Short: "Connect your coding agent to the Brain intelligence layer",
		Long: `Displays the experts guide that covers connecting coding agents (Claude Code,
Cursor, Aider, etc.) to the Brain intelligence layer. Covers gsc experts init,
the /gitsense skill (Claude Code only), and the universal method for all agents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("experts")
		},
	}

	return cmd
}
