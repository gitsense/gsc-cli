/*
 * Component: Root CLI Command
 * Block-UUID: 31ee9181-ff62-4922-adb3-9ba8535f7652
 * Parent-UUID: 8c145aab-bdb6-4f7e-b254-ea58d24c29a5
 * Version: 1.5.0
 * Description: Root command for the gsc CLI, registering the manifest subcommand group and the new top-level query and rg commands.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0), Claude Haiku 4.5 (v1.4.0), GLM-4.7 (v1.5.0)
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
  query       Find files by metadata value
  rg          Search code with metadata enrichment

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
	RegisterQueryCommand(rootCmd)
	RegisterRgCommand(rootCmd)

	logger.Debug("Root command initialized with manifest, query, and rg commands")
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
