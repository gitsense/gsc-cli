/**
 * Component: Root CLI Command
 * Block-UUID: 024fef31-9013-45fe-a8dd-214afed013e8
 * Parent-UUID: 13154028-d260-4444-a9f6-fd0a4549791c
 * Version: 1.39.0
 * Description: Integrated IsInContainer check into PersistentPreRunE to prevent recursive proxy loops when running inside the Docker container.
 * Language: Go
 * Created-at: 2026-03-22T03:36:51.889Z
 * Authors: GLM-4.7 (v1.34.0), Gemini 3 Flash (v1.35.0), Gemini 3 Flash (v1.36.0), GLM-4.7 (v1.37.0), Gemini 3 Flash (v1.38.0), Gemini 3 Flash (v1.39.0)
 */


package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/cli/contract"
	"github.com/gitsense/gsc-cli/internal/cli/app"
	"github.com/gitsense/gsc-cli/internal/cli/manifest"
	"github.com/gitsense/gsc-cli/internal/cli/claude"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/cli/ws"
	"github.com/gitsense/gsc-cli/internal/cli/docker"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
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

		// 2. Smart Proxy Interceptor
		// If a Docker context is active and the command is proxyable, redirect to the container.
		// Check if we are already inside a container to prevent recursive loops.
		if !docker_internal.IsInContainer() && docker_internal.IsProxyableCommand(cmd) && docker_internal.HasContext() {
			proxied, err := docker_internal.ProxyCommand(cmd, args)
			if err != nil {
				// If it's an exit error from the container, exit with that code
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return err
			}
			if proxied {
				// If the command was successfully proxied, we exit the host process.
				// The exit code was already handled by ProxyCommand if it returned an error.
				// If it returned nil error and proxied=true, we assume success (0).
				os.Exit(0)
			}
		}

		// 3. Pre-flight Check: Ensure .gitsense directory exists
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
	docker.RegisterCommand(rootCmd)
	app.RegisterCommand(rootCmd)
	claude.RegisterCommand(rootCmd)
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
	excludedRoots := []string{"init", "doctor", "exec", "ws", "contract", "chats", "messages", "send", "tree", "docker", "app", "claude"}
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
