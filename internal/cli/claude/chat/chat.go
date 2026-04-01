/**
 * Component: Claude Code Chat Command
 * Block-UUID: 1a39a9f0-b1c9-4a3c-af24-4358cdf969c1
 * Parent-UUID: e47eac57-65ab-4fbe-9a28-2239476559f3
 * Version: 1.8.0
 * Description: Fixed undefined variable errors by accessing persistent flags from parent command instead of using global variables.
 * Language: Go
 * Created-at: 2026-03-23T05:55:57.681Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0)
 */


package chat

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	chatint "github.com/gitsense/gsc-cli/internal/claude/chat"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var chatFlags = &ChatFlags{}

var ChatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Execute a chat completion using Claude Code CLI",
	Long: `Executes a chat completion request using the Claude Code CLI as a backend API. 
It reconstructs the conversation history from the GitSense Chat database, manages the 
file-based state, and streams the response back to stdout.`,

	Args: cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Validate Flags
		if err := ValidateChatFlags(chatFlags); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("invalid flags: %w", err)
		}

		// 2. Get persistent flags from parent command
		chatUUID, _ := cmd.Flags().GetString("uuid")
		chatParentID, _ := cmd.Flags().GetInt64("parent-id")

		// 3. Validate UUID
		if chatUUID == "" {
			return fmt.Errorf("--uuid is required")
		}
		
		var userMessage string

		// 4. Resolve User Message with Priority: Argument > Flag > File > Stdin
		if len(args) > 0 {
			// Priority 1: Positional Argument
			userMessage = args[0]
		} else if chatFlags.Message != "" {
			// Priority 2: --message Flag
			userMessage = chatFlags.Message
		} else if chatFlags.File != "" {
			// Priority 3: --file Flag
			data, err := os.ReadFile(chatFlags.File)
			if err != nil {
				return fmt.Errorf("failed to read message file: %w", err)
			}
			userMessage = string(data)
		} else {
			// Priority 4: Stdin (Pipe)
			// Check if data is available on stdin to prevent hanging in interactive mode if not intended
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				// Data is being piped in
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				userMessage = string(data)
			} else {
				return fmt.Errorf("no message provided. Use [message] argument, --message, --file, or pipe input")
			}
		}

		// 5. Execute Chat
		logger.Info("Executing Claude Code chat", "uuid", chatUUID, "parent_id", chatParentID, "append", chatFlags.Append, "save", chatFlags.Save, "append_save", chatFlags.AppendSave, "model", chatFlags.Model, "thinking", chatFlags.ThinkingBudget)
		if err := chatint.ExecuteChat(chatUUID, chatParentID, userMessage, chatFlags.Format, chatFlags.Append, chatFlags.Save, chatFlags.AppendSave, chatFlags.Model, chatFlags.ThinkingBudget); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("chat execution failed: %w", err)
		}

		return nil
	},
}

func init() {
	RegisterChatFlags(ChatCmd, chatFlags)
}
