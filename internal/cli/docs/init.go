/**
 * Component: Docs Init Command
 * Block-UUID: 7a81a418-133a-45d6-b400-0fbeffe3a7c8
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs init' that displays the documentation roadmap and index of available topics.
 * Language: Go
 * Created-at: 2026-05-30T02:57:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewInitCmd creates and returns the 'gsc docs init' subcommand.
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Show the documentation roadmap",
		Long: `Displays the GitSense Chat documentation roadmap, which provides a comprehensive 
index of available guides and instructs the AI on how to navigate users to the right 
information.

This is the recommended starting point for new users and AI agents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("init")
		},
	}

	return cmd
}
