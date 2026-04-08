/**
 * Component: Scout CLI Root Command
 * Block-UUID: 072b0861-e21a-436c-99c6-3cb7e8e3fab6
 * Parent-UUID: 1ad22b86-0964-4228-8d9c-a3c8caf7c2d3
 * Version: 1.0.5
 * Description: Parent command for Scout CLI (start, status, stop subcommands). Scout supports multiple discovery turns followed by verification for iterative file discovery.
 * Language: Go
 * Created-at: 2026-04-08T23:11:57.135Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), claude-haiku-4-5-20251001 (v1.0.2), claude-haiku-4-5-20251001 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5)
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
1. Discovery: Searches working directories using contract insights and Code Intent brain (can run multiple discovery turns)
2. Verification: Optional re-scoring of candidates with Claude for deeper analysis

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

