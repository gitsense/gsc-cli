/**
 * Component: Claude Root Command
 * Block-UUID: 1ed31564-2d2b-4cd7-857b-52ef82512259
 * Parent-UUID: 196e44ad-42f7-4530-9770-16ad020c60df
 * Version: 1.0.4
 * Description: Fixed invalid operation error by passing ChatCmd variable directly instead of calling it as a function.
 * Language: Go
 * Created-at: 2026-04-16T15:33:28.224Z
 * Authors: Gemini 3 Flash (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package claude

import (
	"github.com/spf13/cobra"

	"github.com/gitsense/gsc-cli/pkg/logger"
	scoutcli "github.com/gitsense/gsc-cli/internal/cli/claude/scout"
	chatcli "github.com/gitsense/gsc-cli/internal/cli/claude/chat"
	changecli "github.com/gitsense/gsc-cli/internal/cli/claude/change"
	agentcli "github.com/gitsense/gsc-cli/internal/cli/claude/agent"
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
	claudeCmd.AddCommand(chatcli.ChatCmd)
	claudeCmd.AddCommand(scoutcli.RootCmd())
	claudeCmd.AddCommand(changecli.ChangeCmd())
	claudeCmd.AddCommand(agentcli.RootCmd())

	logger.Debug("Claude root command initialized")
}

// RegisterCommand adds the claude command to the root CLI
func RegisterCommand(root *cobra.Command) {
	root.AddCommand(claudeCmd)
}
