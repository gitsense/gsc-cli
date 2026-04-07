/**
 * Component: Scout CLI Root Command
 * Block-UUID: 1ad22b86-0964-4228-8d9c-a3c8caf7c2d3
 * Parent-UUID: c1a645ff-177c-49f6-b2d2-2f8da1ea98a0
 * Version: 1.0.4
 * Description: Parent command for Scout CLI (start, status, stop subcommands)
 * Language: Go
 * Created-at: 2026-04-06T16:17:33.799Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), claude-haiku-4-5-20251001 (v1.0.2), claude-haiku-4-5-20251001 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package scoutcli

import (
	"github.com/spf13/cobra"
)

// GetAllScoutCommands returns all scout subcommands
func GetAllScoutCommands() []*cobra.Command {
	return []*cobra.Command{
		StartCmd(),
		StatusCmd(),
		StopCmd(),
	}
}

// RootCmd creates the root "scout" command
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scout",
		Short: "Scout file discovery tool",
		Long: `Scout is a fire-and-forget file discovery tool that finds relevant files across repositories.

Scout runs in two phases:
1. Discovery (Turn 1): Searches working directories using contract insights and Code Intent brain
2. Verification (Turn 2): Optional re-scoring of candidates with Claude for deeper analysis

Sessions run as background subprocesses and can be monitored independently of the chat.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register all subcommands
	for _, subCmd := range GetAllScoutCommands() {
		cmd.AddCommand(subCmd)
	}

	return cmd
}
