/*
 * Component: Root CLI Command
 * Block-UUID: 35188e09-96a9-4648-829b-4812d26019e2
 * Parent-UUID: 1e3f1e6d-c0e3-4725-90a7-3fbf20101b02
 * Version: 1.1.0
 * Description: Defines the root 'gsc' command, sets up global flags, and registers top-level subcommands.
 * Language: Go
 * Created-at: 2026-02-02T05:30:02.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0)
 */


package cli

import (
	"os"
	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/cli/manifest"
	"github.com/yourusername/gsc-cli/internal/version"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

var rootCmd = &cobra.Command{
	Use:   "gsc",
	Short: "GitSense CLI - Agent-first codebase intelligence",
	Long: `GSC (GitSense CLI) is a command-line tool designed to make codebases 
queryable for AI agents. It manages metadata manifests, SQLite databases, and 
context bundles to enable efficient, large-scale code analysis.`,
	Version: version.Version,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Register subcommands
	rootCmd.AddCommand(manifest.Cmd)
	
	// Global flags
	rootCmd.PersistentFlags().StringVar(&settings.GitSenseDir, "gitsense-dir", settings.DefaultGitSenseDir, "Directory containing .gitsense configuration")
	
	logger.Debug("Root command initialized")
}

// HandleExit is a helper to handle exit errors gracefully
func HandleExit(err error) {
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
