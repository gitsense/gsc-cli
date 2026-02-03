/**
 * Component: Root CLI Command
 * Block-UUID: f7845bb7-f012-4b51-b60a-4d67539b497e
 * Parent-UUID: dbc4b049-4f39-479a-bc33-234bbd38b913
 * Version: 1.8.0
 * Description: Root command for the gsc CLI, registering the manifest subcommand group, top-level usage commands, config command, and the new info command.
 * Language: Go
 * Created-at: 2026-02-02T19:10:57.816Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0), Claude Haiku 4.5 (v1.4.0), GLM-4.7 (v1.5.0), Claude Haiku 4.5 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0)
 */


package cli

import (
	"os"
	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/cli/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gsc",
	Short: "GitSense CLI - Manage metadata manifests and SQLite databases",
	Long: `GitSense CLI (gsc) is a command-line tool for managing codebase intelligence manifests.
It enables AI agents and developers to interact with structured metadata extracted from code repositories.

Top-Level Commands:
  info        Show current workspace context and status
  query       Find files by metadata value
  rg          Search code with metadata enrichment
  config      Manage context profiles and workspace settings

Management Commands:
  manifest     Initialize, import, and query metadata manifests`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

func init() {
	// Register the manifest subcommand group
	rootCmd.AddCommand(manifest.Cmd)

	// Register top-level usage commands
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(rgCmd)

	// Register the config command
	RegisterConfigCommand(rootCmd)

	// Register the info command
	RegisterInfoCommand(rootCmd)

	logger.Debug("Root command initialized with manifest, query, rg, config, and info commands")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// HandleExit handles the exit code from Execute()
func HandleExit(err error) {
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
