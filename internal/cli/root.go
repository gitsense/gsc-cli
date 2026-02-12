/**
 * Component: Root CLI Command
 * Block-UUID: cb398da7-0eaf-471e-8314-7d0c0400384f
 * Parent-UUID: ff0231c0-9527-4c65-8f3f-65e5a174373e
 * Version: 1.23.0
 * Description: Added global --examples flag to provide structured usage patterns for humans and AI. Integrated with the CLI Bridge to allow seamless capability discovery within GitSense Chat.
 * Language: Go
 * Created-at: 2026-02-12T04:48:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.19.0), Gemini 3 Flash (v1.20.0), Gemini 3 Flash (v1.21.0), GLM-4.7 (v1.22.0), Gemini 3 Flash (v1.23.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/bridge"
	"github.com/yourusername/gsc-cli/internal/cli/manifest"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
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

		// 2. Pre-flight Check: Ensure .gitsense directory exists
		// Skip for 'init', 'doctor', and global '--examples'
		commandName := cmd.Name()
		if commandName != "init" && commandName != "doctor" && !showExamples {
			root, err := git.FindProjectRoot()
			if err != nil {
				cmd.SilenceUsage = true
				return
			}

			gitsenseDir := filepath.Join(root, settings.GitSenseDir)
			if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
				cmd.SilenceUsage = true
				return
			}
		}
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
				return bridge.Execute(bridgeCode, output, rootFormat, cmdStr, time.Since(startTime), "internal", forceInsert)
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
	rootCmd.AddCommand(FieldsCmd)
	rootCmd.AddCommand(InsightsCmd)
	rootCmd.AddCommand(CoverageCmd)
	RegisterGrepCommand(rootCmd)
	RegisterTreeCommand(rootCmd)
	RegisterInfoCommand(rootCmd)

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
		if bErr, ok := err.(*bridge.BridgeError); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", bErr.Message)
			os.Exit(bErr.ExitCode)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
