/**
 * Component: Root CLI Command
 * Block-UUID: 77cc4ba0-06ce-495f-aa0d-e02dafe0630a
 * Parent-UUID: b15dd7d1-bd74-4c5a-9974-ee5b1591e7e1
 * Version: 1.34.0
 * Description: Registered contract.SendCmd as a top-level alias 'gsc send'.
 * Language: Go
 * Created-at: 2026-03-10T16:15:31.493Z
 * Authors: GLM-4.7 (v1.32.2), GLM-4.7 (v1.32.3), GLM-4.7 (v1.32.4), GLM-4.7 (v1.32.5), GLM-4.7 (v1.32.6), GLM-4.7 (v1.32.7), Gemini 3 Flash (v1.32.8), GLM-4.7 (v1.32.9), GLM-4.7 (v1.32.10), GLM-4.7 (v1.33.0), GLM-4.7 (v1.34.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/cli/contract"
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
	Short: "GitSense Chat CLI - Chat bridge and intelligence manager for AI-driven development.",
	Long: `GitSense Chat CLI (gsc) is a chat bridge and intelligence manager for AI-driven development. 
It enables deterministic code discovery via structured metadata and establishes auditable 
"Traceability Contracts" between your local repository and the GitSense Chat app. 

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

		// 2. Pre-flight Check: Ensure .gitsense directory exists
		// Skip for excluded commands (init, doctor, exec, ws, contract) and examples
		if cmd.Name() != "gsc" && !isExcludedCommand(cmd) && !showExamples {
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
	contract.RegisterContractCommand(rootCmd)
	ws.RegisterCommand(rootCmd)
	rootCmd.AddCommand(contract.ChatsCmd)
	rootCmd.AddCommand(contract.MessagesCmd)
	
	// Register the alias for 'gsc contract send'
	rootCmd.AddCommand(contract.SendCmd)

	rootCmd.PersistentFlags().CountP("verbose", "c", "Increase verbosity (-c for info, -cc for debug)")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress all output except errors")
	rootCmd.PersistentFlags().StringVar(&bridgeCode, "code", "", "Bridge code for chat integration (6 digits)")
	rootCmd.PersistentFlags().BoolVar(&forceInsert, "force", false, "Skip confirmation prompt")

	// Examples Flag
	rootCmd.Flags().BoolVar(&showExamples, "examples", false, "Show structured usage examples for humans and AI")
	rootCmd.Flags().StringVarP(&rootFormat, "format", "f", "human", "Output format (human, json)")

	logger.Debug("Root command initialized with examples support")
}

// isExcludedCommand checks if the command or any of its parents are in the exclusion list.
// This allows us to skip the .gitsense check for entire command trees (e.g., 'ws' and 'contract')
// as well as specific top-level commands (e.g., 'init', 'doctor', 'exec').
func isExcludedCommand(cmd *cobra.Command) bool {
	excludedRoots := []string{"init", "doctor", "exec", "ws", "contract", "chats", "messages", "send", "tree"}
	current := cmd

	for current != nil {
		for _, root := range excludedRoots {
			if current.Name() == root {
				return true
			}
		}
		current = current.Parent()
	}
	return false
}

func Execute() error {
	return rootCmd.Execute()
}

// cliError wraps an error with a specific exit code for Cobra
type cliError struct {
	code    int
	message string
}

func (e *cliError) Error() string {
	return e.message
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
