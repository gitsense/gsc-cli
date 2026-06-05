/**
 * Component: Experts Root Command
 * Block-UUID: 8eb80d07-0a74-46e1-bd6b-07c3241e7d9b
 * Parent-UUID: 23fc6adc-d74f-495a-89b8-4406fa0a3219
 * Version: 1.0.2
 * Description: Defines the root command for the 'gsc experts' command group. Registers subcommands for initializing, forgetting, checking the status of expert context, and printing the consultation guide for AI assistants.
 * Language: Go
 * Created-at: 2026-05-02T01:08:01.717Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 2.5 Flash Lite (v1.0.2)
 */


package experts

import (
	"github.com/spf13/cobra"
)

// NewCmd creates and returns the root command for the 'gsc experts' group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experts",
		Short: "Manage expert context for AI agents",
		Long: `The 'gsc experts' command enables any coding agent (Claude Code, Aider, Cursor, etc.)
to become "Brain-Aware" by leveraging the GitSense Intelligence Layer.

It acts as a universal "Intelligence Handshake" by generating a context file
containing the active Brain schemas, query rules, and AI behavioral guidelines.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register subcommands
	cmd.AddCommand(NewInitCmd())
	cmd.AddCommand(NewForgetCmd())
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewSetupAgentCmd())
	cmd.AddCommand(NewGuideCmd())

	return cmd
}
