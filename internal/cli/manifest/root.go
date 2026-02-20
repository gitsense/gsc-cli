/**
 * Component: Manifest Root Command
 * Block-UUID: 571a8f1e-1e46-404c-8ef7-bf439f0cd9e7
 * Parent-UUID: 1c56f16f-be76-427a-99e2-bbbbb363bf43
 * Version: 1.9.0
 * Description: Registered the new 'publish' and 'unpublish' subcommands to enable GitSense Chat app integration.
 * Language: Go
 * Created-at: 2026-02-20T00:40:01.327Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.4.5 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0)
 */


package manifest

import (
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/logger"
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
	Cmd.PersistentFlags().StringVar(&manifestCode, "code", "", "CLI Bridge code (not yet supported for manifest commands")

	// Register subcommands
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(importCmd)
	Cmd.AddCommand(listCmd)

	// Subcommands for GitSense Chat app integration
	Cmd.AddCommand(publishCmd)
	Cmd.AddCommand(unpublishCmd)

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
