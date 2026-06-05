/**
 * Component: Intent Workflow CLI Root Command
 * Block-UUID: 387accc0-4a0a-4cdf-9dfb-a887eaeb61e4
 * Parent-UUID: ef8e2ac9-78a6-46ae-b28f-488976877720
 * Version: 1.3.0
 * Description: Parent command for Agent CLI (status, stop, delete, retry). Agent provides generic session management for discovery and change turns.
 * Language: Go
 * Created-at: 2026-04-20T15:19:34.988Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package intentworkflowcli

import (
	"github.com/spf13/cobra"
)

// GetAllAgentCommands returns all agent subcommands
func GetAllAgentCommands() []*cobra.Command {
	return []*cobra.Command{
		StatusCmd(),
		StopCmd(),
		DeleteCmd(),
		RetryCmd(),
	}
}

// RootCmd creates the root "intent-workflow" command
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "intent-workflow",
		Short: "Intent workflow session management",
		Long: `Agent provides generic session management for AI-driven development tasks.

Intent workflow sessions support multiple turn types:
1. Discovery: Searches working directories using contract insights and Code Intent brain
2. Change: In-place code editing based on discovery results

Sessions run as background subprocesses and can be monitored independently of the chat.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register all subcommands
	for _, subCmd := range GetAllAgentCommands() {
		cmd.AddCommand(subCmd)
	}

	return cmd
}
