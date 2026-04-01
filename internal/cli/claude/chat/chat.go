/**
 * Component: Claude Code Chat Command
 * Block-UUID: 1a39a9f0-b1c9-4a3c-af24-4358cdf969c1
 * Parent-UUID: e47eac57-65ab-4fbe-9a28-2239476559f3
 * Version: 1.6.0
 * Description: Added --thinking flag to allow users to set the extended thinking budget for Claude Code CLI.
 * Language: Go
 * Created-at: 2026-03-23T05:55:57.681Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0)
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

var (
	chatMessage   string
	chatFile      string
	chatFormat    string
	chatAppend    bool
	chatSave      bool
	chatModel     string
	chatAppendSave bool
	chatThinkingBudget int
)

var ChatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Execute a chat completion using Claude Code CLI",
	Long: `Executes a chat completion request using the Claude Code CLI as a backend API. 
It reconstructs the conversation history from the GitSense Chat database, manages the 
file-based state, and streams the response back to stdout.`,

	Args: cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Validate Inputs
		if chatUUID == "" {
			return fmt.Errorf("--uuid is required")
		}
		
		var userMessage string

		// 2. Resolve User Message with Priority: Argument > Flag > File > Stdin
		if len(args) > 0 {
			// Priority 1: Positional Argument
			userMessage = args[0]
		} else if chatMessage != "" {
			// Priority 2: --message Flag
			userMessage = chatMessage
		} else if chatFile != "" {
			// Priority 3: --file Flag
			data, err := os.ReadFile(chatFile)
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

		// 3. Execute Chat
		logger.Info("Executing Claude Code chat", "uuid", chatUUID, "parent_id", chatParentID, "append", chatAppend, "save", chatSave, "append_save", chatAppendSave, "model", chatModel, "thinking", chatThinkingBudget)
		if err := chatint.ExecuteChat(chatUUID, chatParentID, userMessage, chatFormat, chatAppend, chatSave, chatAppendSave, chatModel, chatThinkingBudget); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("chat execution failed: %w", err)
		}

		return nil
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatMessage, "message", "", "The user message to send")
	chatCmd.Flags().StringVar(&chatFile, "file", "", "Path to a file containing the user message")
	chatCmd.Flags().StringVar(&chatFormat, "format", "text", "Output format: text or json")
	chatCmd.Flags().BoolVar(&chatAppend, "append", false, "Automatically append to the latest message in the chat")
	chatCmd.Flags().BoolVar(&chatSave, "save", false, "Save the response to the database")
	chatCmd.Flags().StringVar(&chatModel, "model", "", "The model to use (e.g., claude-3-5-sonnet)")
	chatCmd.Flags().BoolVar(&chatAppendSave, "append-save", false, "Save the user message to the database and append to the latest message")
	chatCmd.Flags().IntVar(&chatThinkingBudget, "thinking", 0, "Thinking budget in tokens (0 = disabled)")
}
