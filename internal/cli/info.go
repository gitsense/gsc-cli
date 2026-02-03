/*
 * Component: Info Command
 * Block-UUID: 2984e482-61c2-4204-9300-8dac9ff40905
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for 'gsc info', displaying the current workspace context, active profile, and available databases.
 * Language: Go
 * Created-at: 2026-02-03T03:15:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package cli

import (
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
		logger.Info("Gathering workspace information...")
		info, err := manifest.GetWorkspaceInfo(ctx)
		if err != nil {
			logger.Error("Failed to gather workspace info: %v", err)
			return err
		}

		// 2. Format and Output
		output := manifest.FormatWorkspaceInfo(info, infoFormat, infoVerbose)
		print(output)

		return nil
	},
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
	print(s)
}

// RegisterInfoCommand registers the info command with the root command.
func RegisterInfoCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(infoCmd)
}
