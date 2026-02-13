/**
 * Component: Info Command
 * Block-UUID: 0ca81879-f2d3-4719-8472-78465957ae09
 * Parent-UUID: 3f73ed1a-0e4a-4507-9b22-10da1bf75391
 * Version: 1.0.6
 * Description: CLI command definition for 'gsc info', displaying the current workspace context and available databases. Updated help text to remove references to profiles and configuration features, which are now internal/hidden. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted Info logs to Debug, removed redundant Error logs, and set SilenceUsage to true. Integrated CLI Bridge: if --code is provided, output is captured and sent to the bridge orchestrator for chat insertion.
 * Language: Go
 * Created-at: 2026-02-09T02:48:32.741Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.1.0 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), Gemini 3 Flash (v1.0.4), Gemini 3 Flash (v1.0.5), GLM-4.7 (v1.0.6)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	infoVerbose bool
	infoFormat  string
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current workspace information",
	Long: `Display a summary of the current GitSense Chat workspace and available databases.
This command helps you understand your current context without needing to run
multiple commands.`,
	Example: `  # Show basic workspace info
  gsc info

  # Show detailed information
  gsc info --verbose

  # Output as JSON for scripts
  gsc info --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		ctx := cmd.Context()

		// 0. Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				return err
			}
		}

		// 1. Gather Workspace Information
		// INTERNAL: This function loads internal profile data, but the formatter hides it.
		logger.Debug("Gathering workspace information")
		info, err := manifest.GetWorkspaceInfo(ctx)
		if err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		// 2. Format and Output
		// If bridgeCode is present, we enforce noColor to ensure clean output for the chat
		noColor := bridgeCode != ""
		output := manifest.FormatWorkspaceInfo(info, infoFormat, infoVerbose, noColor)

		// 3. CLI Bridge Integration
		if bridgeCode != "" {
			// Print to stdout (as per spec: "display output as we normally would")
			fmt.Print(output)

			// Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			// info command does not target a specific database, so dbName is empty
			return bridge.Execute(bridgeCode, output, infoFormat, cmdStr, time.Since(startTime), "", forceInsert)
		}

		// Standard Output Mode
		print(output)

		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors
}

func init() {
	// Add flags
	infoCmd.Flags().BoolVarP(&infoVerbose, "verbose", "v", false, "Show detailed information")
	infoCmd.Flags().StringVarP(&infoFormat, "format", "f", "table", "Output format (table, json)")
}

// print is a helper to print output (can be extended for file output later)
func print(s string) {
	// For now, just print to stdout
	// In the future, we might support --output flag
	fmt.Println(s)
}

// RegisterInfoCommand registers the info command with the root command.
func RegisterInfoCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(infoCmd)
}
