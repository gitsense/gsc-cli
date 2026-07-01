/**
 * Component: Root CLI Command
 * Block-UUID: 31e4f003-b411-4160-8a8e-774fcc30fa85
 * Parent-UUID: f31e3478-033a-4dc4-be11-f93e57d3ee93
 * Version: 1.51.0
 * Description: Registered the top-level lessons command group and excluded it from workspace preflight so lesson drafts can initialize GitSense state.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: GLM-4.7 (v1.34.0), Gemini 3 Flash (v1.35.0), Gemini 3 Flash (v1.36.0), GLM-4.7 (v1.37.0), Gemini 3 Flash (v1.38.0), Gemini 3 Flash (v1.39.0), GLM-4.7 (v1.40.0), claude-haiku-4-5-20251001 (v1.40.1), GLM-4.7 (v1.41.0), GLM-4.7 (v1.42.0), GLM-4.7 (v1.43.0), GLM-4.7 (v1.44.0), GLM-4.7 (v1.45.0), GLM-4.7 (v1.46.0), GLM-4.7 (v1.47.0), GLM-4.7 (v1.48.0), GLM-4.7 (v1.49.0), GLM-4.7 (v1.50.0), Codex GPT-5 (v1.51.0)
 */


package cli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/cli/app"
	"github.com/gitsense/gsc-cli/internal/cli/claude"
	"github.com/gitsense/gsc-cli/internal/cli/docs"
	"github.com/gitsense/gsc-cli/internal/cli/experts"
	"github.com/gitsense/gsc-cli/internal/cli/gitignore"
	"github.com/gitsense/gsc-cli/internal/cli/knowledge"
	"github.com/gitsense/gsc-cli/internal/cli/lessons"
	"github.com/gitsense/gsc-cli/internal/cli/notes"
	"github.com/gitsense/gsc-cli/internal/cli/rules"
	"github.com/gitsense/gsc-cli/internal/cli/topics"
	"github.com/gitsense/gsc-cli/internal/cli/manifest"
	"github.com/gitsense/gsc-cli/internal/cli/pi"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	manifestpkg "github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/version"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/spf13/cobra"
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
	Use:     "gsc",
	Short:   "GitSense Chat CLI - Chat bridge and intelligence manager for AI-driven development.",
	Version: version.GetVersion(),
	Long: `GitSense Chat CLI (gsc) is a chat bridge and intelligence manager for AI-driven development. 
It enables deterministic code discovery via structured metadata and establishes auditable 
"Traceability Contracts" between your local repository and the GitSense Chat app. 

AI ASSISTANT DISCOVERY:
  To discover structured capabilities and command patterns for this repository, run:
  gsc --examples --format json`,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	SilenceErrors:     true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Debug: Log the command name and arguments
		logger.Debug("PersistentPreRunE invoked", "cmd_name", cmd.Name(), "args", args)

		// 1. Check for quiet flag first
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			logger.SetLogLevel(logger.LevelError)
			logger.Debug("Log level set to ERROR (quiet mode)")
		} else {
			verbose, _ := cmd.Flags().GetCount("verbose")
			logger.Debug("Verbose count detected", "count", verbose)
			switch verbose {
			case 0:
				logger.SetLogLevel(logger.LevelWarning)
			case 1:
				logger.SetLogLevel(logger.LevelInfo)
			default:
				logger.SetLogLevel(logger.LevelDebug)
				logger.Debug("Log level set to DEBUG")
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
		// Skip for excluded commands (init, doctor, app, etc.) and examples
		if cmd.Name() != "gsc" && !isExcludedCommand(cmd) && !showExamples {
			if err := manifestpkg.ValidateWorkspace(); err != nil {
				cmd.SilenceUsage = true
				return err
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
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(manifest.Cmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(valuesCmd)
	rootCmd.AddCommand(InsightsCmd)
	rootCmd.AddCommand(CoverageCmd)
	rootCmd.AddCommand(BrainsCmd)
	RegisterGrepCommand(rootCmd)
	RegisterTreeCommand(rootCmd)
	RegisterInfoCommand(rootCmd)

	// Commands moved to 'app' group are now registered there
	// contract.RegisterContractCommand(rootCmd) // REMOVED
	// ws.RegisterCommand(rootCmd)             // REMOVED
	// RegisterExecCommand(rootCmd)            // REMOVED

	// docker.RegisterCommand(rootCmd) // Removed: docker is now nested under app
	rootCmd.AddCommand(experts.NewCmd())
	rootCmd.AddCommand(docs.NewCmd())
	app.RegisterCommand(rootCmd)
	claude.RegisterCommand(rootCmd)

	// Register gitignore command group
	rootCmd.AddCommand(gitignore.Cmd)
	rootCmd.AddCommand(lessons.NewCmd())
	rootCmd.AddCommand(rules.NewCmd())
	rootCmd.AddCommand(notes.NewCmd())
	rootCmd.AddCommand(topics.NewCmd())
	rootCmd.AddCommand(knowledge.NewCmd())
	rootCmd.AddCommand(pi.NewCmd())
	rootCmd.AddCommand(newVersionCmd())

	// Aliases removed
	// rootCmd.AddCommand(contract.ChatsCmd)     // REMOVED
	// rootCmd.AddCommand(contract.MessagesCmd)  // REMOVED
	// rootCmd.AddCommand(contract.SendCmd)      // REMOVED

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
// This allows us to skip the .gitsense check for entire command trees (e.g., 'app')
// as well as specific top-level commands (e.g., 'init', 'doctor').
func isExcludedCommand(cmd *cobra.Command) bool {
	// Removed "contract", "ws", "exec", "chats", "messages", "send" as they are now under "app"
	excludedRoots := []string{"init", "doctor", "tree", "docker", "app", "claude", "import", "manifest", "docs", "gitignore", "lessons", "rules", "pi", "brains", "version", "experts"}
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
