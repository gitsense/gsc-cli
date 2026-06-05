/**
 * Component: Docs Quickstart Command
 * Block-UUID: 9d1a6b28-3c55-49e8-acf5-b959224cab51
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs quickstart' that displays the choose-your-path quickstart guide from zero to a fully intelligent coding session.
 * Language: Go
 * Created-at: 2026-05-31T17:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewQuickstartCmd creates and returns the 'gsc docs quickstart' subcommand.
func NewQuickstartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "Choose your path from zero to a fully intelligent session",
		Long: `Displays the quickstart guide that routes users through four paths:
Smart Repo (fastest), Fresh Start, Human Developer (gsc rg), and Agentic Worktree.
This is the recommended entry point for new users.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("quickstart")
		},
	}

	return cmd
}
