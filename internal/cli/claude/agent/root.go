/**
 * Component: Agent CLI Root Command
 * Block-UUID: f671e9ac-60e8-4244-baee-b30fc7cf02ce
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parent command for Agent CLI (status, and future agent commands). Agent provides generic session management for discovery, validation, and change turns.
 * Language: Go
 * Created-at: 2026-04-16T15:31:47.219Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package agentcli

import (
	"github.com/spf13/cobra"
)

// GetAllAgentCommands returns all agent subcommands
func GetAllAgentCommands() []*cobra.Command {
	return []*cobra.Command{
		StatusCmd(),
	}
}

// RootCmd creates the root "agent" command
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent session management",
		Long: `Agent provides generic session management for AI-driven development tasks.

Agent sessions support multiple turn types:
1. Discovery: Searches working directories using contract insights and Code Intent brain
2. Validation: Re-scoring of candidates with Claude for deeper analysis
3. Change: In-place code editing based on validated results

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
