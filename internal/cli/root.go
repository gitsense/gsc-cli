/**
 * Component: Root CLI Command
 * Block-UUID: 9623f116-320e-4613-bce7-4a03d71cd60b
 * Parent-UUID: 9e61c0b7-26d5-4d7a-85a3-5fda37393d8c
 * Version: 1.32.3
 * Description: Registered the new 'contract' command and whitelisted it to bypass the .gitsense directory check, allowing it to run in any directory.
 * Language: Go
 * Created-at: 2026-03-07T23:48:19.630Z
 * Authors: GLM-4.7 (v1.32.2), GLM-4.7 (v1.32.3)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/cli/manifest"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/cli/ws"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// Global flags
var (
	bridgeCode   string
	forceInsert  bool
	showExamples bool
	rootFormat   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gsc",
	Short: "GitSense Chat CLI - Manage metadata manifests and SQLite databases",
	Long: `GitSense Chat CLI (gsc) is a command-line tool for managing codebase intelligence manifests.
It enables AI agents and developers to interact with structured metadata extracted from code repositories.

AI ASSISTANT DISCOVERY:
  To discover structured capabilities and command patterns for this repository, run:
  gsc --examples --format json`,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Debug: Log the command name and arguments
		logger.Debug("PersistentPreRunE invoked", "cmd_name", cmd.Name(), "args", args)

		// 1. Check for quiet flag first
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			logger.SetLogLevel(logger.LevelError)
		} else {
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

		// 2. Enforce GSC_HOME for ws and contract commands
		// cmd.Name() returns the command owning the hook (rootCmd), so we check args[0]
		targetCommand := ""
		if len(args) > 0 {
			targetCommand = args[0]
		}

		logger.Debug("Target command identified", "target", targetCommand)

		if targetCommand == "ws" || targetCommand == "contract" {
			logger.Debug("Enforcing GSC_HOME check", "command", targetCommand)
			gscHome, err := settings.GetGSCHome(true)
			logger.Debug("GetGSCHome check result", "path", gscHome, "error", err)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		// 3. Pre-flight Check: Ensure .gitsense directory exists
		// Skip for 'init', 'doctor', 'exec', 'contract', and global '--examples'
		if targetCommand != "init" && targetCommand != "doctor" && targetCommand != "exec" && targetCommand != "contract" && targetCommand != "ws" && !showExamples {
			root, err := git.FindProjectRoot()
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			gitsenseDir := filepath.Join(root, settings.GitSenseDir)
			if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
				cmd.SilenceUsage = true
				return fmt.Errorf("GitSense workspace not found. Run 'gsc manifest init' to initialize")
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if showExamples {
			startTime := time.Now()
			
			output, err := RenderExamples(rootFormat)
			if err != nil {
				return err
			}

			if bridgeCode != "" {
				fmt.Print(output)
				cmdStr := "gsc --examples --format " + rootFormat
				return bridge.Execute(bridgeCode, output, rootFormat, cmdStr, time.Since(startTime), "internal", 0, forceInsert)
			}

			fmt.Print(output)
			return nil
		}

		// If no subcommand and no examples flag, print help
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(manifest.Cmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(valuesCmd)
	rootCmd.AddCommand(InsightsCmd)
	rootCmd.AddCommand(CoverageCmd)
	rootCmd.AddCommand(BrainsCmd)
	RegisterGrepCommand(rootCmd)
	RegisterTreeCommand(rootCmd)
	RegisterInfoCommand(rootCmd)
	RegisterExecCommand(rootCmd)
	RegisterContractCommand(rootCmd)
	ws.RegisterCommand(rootCmd)

	rootCmd.PersistentFlags().CountP("verbose", "c", "Increase verbosity (-c for info, -cc for debug)")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress all output except errors")
	rootCmd.PersistentFlags().StringVar(&bridgeCode, "code", "", "Bridge code for chat integration (6 digits)")
	rootCmd.PersistentFlags().BoolVar(&forceInsert, "force", false, "Skip confirmation prompt")

	// Examples Flag
	rootCmd.Flags().BoolVar(&showExamples, "examples", false, "Show structured usage examples for humans and AI")
	rootCmd.Flags().StringVarP(&rootFormat, "format", "f", "human", "Output format (human, json)")

	logger.Debug("Root command initialized with examples support")
}

func Execute() error {
	return rootCmd.Execute()
}

func HandleExit(err error) {
	if err != nil {
		// Handle custom CLI errors with specific exit codes
		if cErr, ok := err.(*cliError); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", cErr.message)
			os.Exit(cErr.code)
		}

		if bErr, ok := err.(*bridge.BridgeError); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", bErr.Message)
			os.Exit(bErr.ExitCode)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
