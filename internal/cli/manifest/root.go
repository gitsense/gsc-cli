/**
 * Component: Manifest Root Command
 * Block-UUID: 52a6e21f-bbc1-4fda-b3d4-2241c3737d0c
 * Parent-UUID: 458d0659-00ee-4903-ac2d-9d4572a7db1a
 * Version: 1.3.0
 * Description: Defines the root command for the 'manifest' subcommand group, serving as the parent for init, import, and list.
 * Language: Go
 * Created-at: 2026-02-02T08:42:21.341Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0)
 */


package manifest

import (
	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// Cmd represents the manifest command
var Cmd = &cobra.Command{
	Use:   "manifest",
	Short: "Manage metadata manifests and SQLite databases",
	Long: `The manifest command group provides tools to initialize, import, and query
metadata manifests. These manifests serve as a queryable intelligence layer for
AI agents, allowing them to efficiently discover and analyze codebase structure.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

func init() {
	// Add shared flags to the manifest root command
	AddManifestFlags(Cmd)

	// Register subcommands
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(importCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(exportCmd)
	Cmd.AddCommand(bundleCmd)
	Cmd.AddCommand(schemaCmd)
	Cmd.AddCommand(doctorCmd)

	logger.Debug("Manifest root command initialized")
}
