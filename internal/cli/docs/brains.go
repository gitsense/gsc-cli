/**
 * Component: Docs Brains Command
 * Block-UUID: 3e8d7c4a-1b5f-4a9e-8d2c-6b7a8c9d0e1f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs brains' that displays the guide for understanding Brains and Manifests, creating Brains, and publishing intelligence.
 * Language: Go
 * Created-at: 2026-05-31T17:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewBrainsCmd creates and returns the 'gsc docs brains' subcommand.
func NewBrainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brains",
		Short: "Understand Brains, Manifests, and how to create and share intelligence",
		Long: `Displays the brains guide that explains what Brains and Manifests are,
how they differ, how to create a Brain from a manifest, the "README for AI"
committed-manifest pattern, publishing, and enterprise centralization.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("brains")
		},
	}

	return cmd
}
