/*
 * Component: Manifest Root Command
 * Block-UUID: 79ae243d-d0d9-4881-8de6-5af8755c33c8
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the root command for the 'manifest' subcommand group, serving as the parent for init, import, and list.
 * Language: Go
 * Created-at: 2026-02-02T05:30:01.000Z
 * Authors: GLM-4.7 (v1.0.0)
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
	
	// Note: Subcommands (init, import, list) will register themselves 
	// in their respective files to avoid circular dependencies.
	logger.Debug("Manifest root command initialized")
}
