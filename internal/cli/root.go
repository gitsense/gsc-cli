/**
 * Component: Root CLI Command
 * Block-UUID: afdf533d-9cb0-407e-9224-b87644db9dd7
 * Parent-UUID: 00fa3c7e-452a-4142-95c5-80ebc6bdd028
 * Version: 1.32.8
 * Description: Updated imports to reference the new contract subpackage (internal/cli/contract) following the refactoring of contract commands.
 * Language: Go
 * Created-at: 2026-03-08T03:08:17.799Z
 * Authors: GLM-4.7 (v1.32.2), GLM-4.7 (v1.32.3), GLM-4.7 (v1.32.4), GLM-4.7 (v1.32.5), GLM-4.7 (v1.32.6), GLM-4.7 (v1.32.7), Gemini 3 Flash (v1.32.8)
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
		// Skip for 'init', 'doctor', 'exec', 'contract', 'ws', and global '--examples'
		// Note: 'contract' and 'ws' handle their own GSC_HOME validation in their respective command groups.
		name := cmd.Name()
		if name != "gsc" && name != "init" && name != "doctor" && name != "exec" && name != "contract" && name != "ws" && !showExamples {
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
