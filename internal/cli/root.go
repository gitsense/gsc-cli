/**
 * Component: Root CLI Command
 * Block-UUID: 3bac1959-9c33-4adb-9074-dab9a8297084
 * Parent-UUID: 67d4326c-8f7e-4434-91c2-dcd1fb9d79ba
 * Version: 1.17.0
 * Description: Root command for the gsc CLI, registering the manifest subcommand group, top-level usage commands, config command, and the new info command. Replaced 'rg' with 'grep' command. Added pre-flight check in PersistentPreRun to ensure .gitsense directory exists for all commands except 'init' and 'doctor', preventing misleading errors. Updated to support professional CLI output: modified HandleExit to print clean error messages without logger prefixes, and refactored PersistentPreRun to return errors for pre-flight checks while silencing usage output for logic errors. Added persistent global flags for CLI Bridge integration: --code (6-digit bridge code) and --force (skip confirmation prompt).
 * Language: Go
 * Created-at: 2026-02-08T07:30:01.169Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), ..., Claude Haiku 4.5 (v1.16.0), Gemini 3 Flash (v1.17.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/bridge"
	"github.com/yourusername/gsc-cli/internal/cli/manifest"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// Global bridge flags
var (
	bridgeCode  string
	forceInsert bool
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
  grep        Search code with metadata enrichment
  config      Manage context profiles and workspace settings

Management Commands:
  manifest     Initialize, import, and query metadata manifests`,
	// Disable the default 'completion' command to reduce test scope.
	// Shell completion functionality exists in Cobra but is hidden for now.
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 1. Check for quiet flag first
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			logger.SetLogLevel(logger.LevelError)
		} else {
			// Check verbosity count to set log level
			verbose, _ := cmd.Flags().GetCount("verbose")
			switch verbose {
			case 0:
				logger.SetLogLevel(logger.LevelWarning)
			case 1:
				logger.SetLogLevel(logger.LevelInfo)
			default:
				logger.SetLogLevel(logger.LevelDebug)
			}
		}

		// 2. Pre-flight Check: Ensure .gitsense directory exists
		// Skip for 'init' (creates it) and 'doctor' (diagnostic tool)
		commandName := cmd.Name()
		if commandName != "init" && commandName != "doctor" {
			root, err := git.FindProjectRoot()
			if err != nil {
				// Logic error: Not in a git repository. Silence usage and return error.
				cmd.SilenceUsage = true
				return
			}

			gitsenseDir := filepath.Join(root, settings.GitSenseDir)
			if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
				// Logic error: Workspace not initialized. Silence usage and return error.
				cmd.SilenceUsage = true
				return
			}
		}
	},
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
	// Replaced rgCmd with grepCmd
	RegisterGrepCommand(rootCmd)

	// Register the config command
	RegisterConfigCommand(rootCmd)

	// Register the info command
	RegisterInfoCommand(rootCmd)

	// Add global verbose flag
	// -v for Info level, -vv for Debug level
	rootCmd.PersistentFlags().CountP("verbose", "c", "Increase verbosity (-c for info, -cc for debug)")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress all output except errors")

	// Add CLI Bridge flags
	rootCmd.PersistentFlags().StringVar(&bridgeCode, "code", "", "Bridge code for chat integration (6 digits)")
	rootCmd.PersistentFlags().BoolVar(&forceInsert, "force", false, "Skip confirmation prompt (only if under size limit)")

	logger.Debug("Root command initialized with manifest, query, grep, config, and info commands")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// HandleExit handles the exit code from Execute()
func HandleExit(err error) {
	if err != nil {
		// Check for Bridge-specific exit codes
		if bErr, ok := err.(*bridge.BridgeError); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", bErr.Message)
			os.Exit(bErr.ExitCode)
		}

		// Print clean error message without [ERROR] prefix or timestamp
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
