/**
 * Component: Manifest Root Command
 * Block-UUID: d7a8e30f-790a-4898-8e11-2525080a1fe7
 * Parent-UUID: 62834949-e5fa-4002-a389-15c2886e1698
 * Version: 1.7.0
 * Description: Defines the root command for the 'manifest' subcommand group, serving as the parent for init, import, list, and delete.
 * Language: Go
 * Created-at: 2026-02-13T04:59:45.865Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.4.5 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0)
 */


package manifest

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var manifestCode string

// Cmd represents the manifest command
var Cmd = &cobra.Command{
	Use:   "manifest",
	Short: "Manage metadata manifests and SQLite databases (Setup & Maintenance)",
	Long: `The manifest command group provides tools to initialize, import, and manage
metadata manifests. These manifests serve as a queryable intelligence layer for
AI agents. Use 'gsc query' and 'gsc rg' for daily operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

func init() {
	// Add shared flags to the manifest root command
	AddManifestFlags(Cmd)

	// Add --code flag for future support
	Cmd.PersistentFlags().StringVar(&manifestCode, "code", "", "CLI Bridge code (not yet supported for manifest commands)")

	// Intercept --code flag to inform user of future support
	Cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if manifestCode != "" {
			return fmt.Errorf("the --code flag is not yet supported for manifest commands. It will be available in a future release")
		}
		return nil
	}

	// Register subcommands
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(importCmd)
	Cmd.AddCommand(listCmd)

	// Temporarily hidden commands due to lack of time for validation.
	// The code logic remains in bundle.go and export.go.
	// Uncomment these lines when ready to re-enable.
	// Cmd.AddCommand(exportCmd)
	// Cmd.AddCommand(bundleCmd)

	Cmd.AddCommand(schemaCmd)
	Cmd.AddCommand(doctorCmd)
	Cmd.AddCommand(deleteCmd)

	logger.Debug("Manifest root command initialized")
}
