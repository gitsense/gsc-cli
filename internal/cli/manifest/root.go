/**
 * Component: Manifest Root Command
 * Block-UUID: 458d0659-00ee-4903-ac2d-9d4572a7db1a
 * Parent-UUID: 783977f9-ee5f-466f-9cdf-bef6b69f5791
 * Version: 1.2.0
 * Description: Defines the root command for the 'manifest' subcommand group, serving as the parent for init, import, and list.
 * Language: Go
 * Created-at: 2026-02-02T08:12:02.953Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
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

	logger.Debug("Manifest root command initialized")
}

