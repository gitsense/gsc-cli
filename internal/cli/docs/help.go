/**
 * Component: Docs Help Command
 * Block-UUID: 6451e182-ea9f-4872-8fe3-0b18455de58c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs help' that displays the documentation roadmap. This is the user-facing entry point that serves as an alias for 'gsc docs init'.
 * Language: Go
 * Created-at: 2026-05-31T00:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewHelpCmd creates and returns the 'gsc docs help' subcommand.
func NewHelpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help",
		Short: "Get help with GitSense Chat - your personal success manager",
		Long: `Displays the GitSense Chat documentation roadmap, which provides a comprehensive 
index of available guides and instructs the AI on how to navigate users to the right 
information.

This is the recommended starting point for new users. It serves as a user-friendly 
alias for 'gsc docs init' - both commands display the same content.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("init")
		},
	}

	return cmd
}
