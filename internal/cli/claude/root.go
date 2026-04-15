/**
 * Component: Claude Root Command
 * Block-UUID: 196e44ad-42f7-4530-9770-16ad020c60df
 * Parent-UUID: 8b2f4e9a-5c7d-4a3b-9e1f-6d7c8a5b3e2f
 * Version: 1.0.3
 * Description: Fixed invalid operation error by passing ChatCmd variable directly instead of calling it as a function.
 * Language: Go
 * Created-at: 2026-04-15T04:06:59.528Z
 * Authors: Gemini 3 Flash (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package claude

import (
	"github.com/spf13/cobra"

	"github.com/gitsense/gsc-cli/pkg/logger"
	claudescout "github.com/gitsense/gsc-cli/internal/cli/claude/scout"
	claudechat "github.com/gitsense/gsc-cli/internal/cli/claude/chat"
	claudechange "github.com/gitsense/gsc-cli/internal/cli/claude/change"
)

// Global flags
var (
	chatUUID    string
	chatParentID int64
)

// claudeCmd represents the base command for Claude Code integration
var claudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Manage Claude Code CLI integration for traceable API replacement",
	Long: `The claude command group provides tools to initialize the Claude Code environment
and execute chat sessions using the Claude Code CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

func init() {
	// Add persistent flags to the claude root command
	claudeCmd.PersistentFlags().StringVar(&chatUUID, "uuid", "", "The GitSense Chat UUID for the session")
	claudeCmd.PersistentFlags().Int64Var(&chatParentID, "parent-id", 0, "The ID of the parent message to reply to")

	// Register subcommands
	claudeCmd.AddCommand(initCmd)
	claudeCmd.AddCommand(claudechat.ChatCmd)
	claudeCmd.AddCommand(claudescout.RootCmd())
	claudeCmd.AddCommand(claudechange.ChangeCmd())

	logger.Debug("Claude root command initialized")
}

// RegisterCommand adds the claude command to the root CLI
func RegisterCommand(root *cobra.Command) {
	root.AddCommand(claudeCmd)
}
