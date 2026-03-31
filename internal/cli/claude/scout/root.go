/**
 * Component: Scout CLI Root Command
 * Block-UUID: 98666b7f-80b0-4f02-b476-09435f24046d
 * Parent-UUID: 41df1421-1aef-4ab6-b67b-75be7376604c
 * Version: 1.0.2
 * Description: Parent command for Scout CLI (start, status, stop subcommands)
 * Language: Go
 * Created-at: 2026-03-31T02:09:02.329Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), claude-haiku-4-5-20251001 (v1.0.2)
 */


package scoutcli

import (
	"github.com/spf13/cobra"
)

// RootCmd creates the root "scout" command
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scout",
		Short: "Scout file discovery tool",
		Long: `Scout is a fire-and-forget file discovery tool that finds relevant files across repositories.

Scout runs in two phases:
1. Discovery (Turn 1): Searches working directories using contract insights and Tiny Overview brain
2. Verification (Turn 2): Optional re-scoring of candidates with Claude for deeper analysis

Sessions run as background subprocesses and can be monitored independently of the chat.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register subcommands
	cmd.AddCommand(StartCmd())
	cmd.AddCommand(StatusCmd())
	cmd.AddCommand(ResultsCmd())
	cmd.AddCommand(StopCmd())

	return cmd
}

// GetAllScoutCommands returns all scout subcommands
func GetAllScoutCommands() []*cobra.Command {
	return []*cobra.Command{
		StartCmd(),
		StatusCmd(),
		ResultsCmd(),
		StopCmd(),
	}
}
