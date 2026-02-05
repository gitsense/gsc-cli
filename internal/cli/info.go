/**
 * Component: Info Command
 * Block-UUID: 1dc4eb6b-2a29-4f0b-adba-9a95abe54779
 * Parent-UUID: aa749e28-6941-4a69-8176-f281bc04d2cd
 * Version: 1.0.3
 * Description: CLI command definition for 'gsc info', displaying the current workspace context, active profile, and available databases. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted Info logs to Debug, removed redundant Error logs, and set SilenceUsage to true.
 * Language: Go
 * Created-at: 2026-02-03T03:16:25.331Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	infoVerbose bool
	infoFormat  string
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current workspace context and status",
	Long: `Display a summary of the current GitSense workspace, including the active profile,
available databases, and project configuration. This command helps you understand
your current context without needing to run multiple commands.`,
	Example: `  # Show basic workspace info
  gsc info

  # Show detailed information
  gsc info --verbose

  # Output as JSON for scripts
  gsc info --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// 1. Gather Workspace Information
		logger.Debug("Gathering workspace information")
		info, err := manifest.GetWorkspaceInfo(ctx)
		if err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		// 2. Format and Output
		output := manifest.FormatWorkspaceInfo(info, infoFormat, infoVerbose)
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
	fmt.Print(s)
}

// RegisterInfoCommand registers the info command with the root command.
func RegisterInfoCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(infoCmd)
}
