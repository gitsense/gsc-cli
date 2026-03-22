/**
 * Component: Claude Code Chat Command
 * Block-UUID: 3957d13f-1a6e-434a-b0bd-1aee0cba0a34
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc claude chat' command to execute chat completions using the Claude Code CLI, handling input validation and orchestrating the execution flow.
 * Language: Go
 * Created-at: 2026-03-22T03:41:05.789Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package claude

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	// Alias the internal logic package to avoid naming conflict with the CLI package
	claudeint "github.com/gitsense/gsc-cli/internal/claude"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	chatMessage string
	chatFile    string
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Execute a chat completion using Claude Code CLI",
	Long: `Executes a chat completion request using the Claude Code CLI as a backend API. 
It reconstructs the conversation history from the GitSense Chat database, manages the 
file-based state, and streams the response back to stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Validate Inputs
		if chatUUID == "" {
			return fmt.Errorf("--uuid is required")
		}
		if chatParentID == 0 {
			return fmt.Errorf("--parent-id is required")
		}

		var userMessage string
		var err error

		// 2. Resolve User Message
		if chatFile != "" {
			// Read from file
			data, err := os.ReadFile(chatFile)
			if err != nil {
				return fmt.Errorf("failed to read message file: %w", err)
			}
			userMessage = string(data)
		} else if chatMessage != "" {
			// Use string
			userMessage = chatMessage
		} else {
			return fmt.Errorf("either --message or --file is required")
		}

		// 3. Execute Chat
		// Delegate to the internal logic layer
		logger.Info("Executing Claude Code chat", "uuid", chatUUID, "parent_id", chatParentID)
		if err := claudeint.ExecuteChat(chatUUID, chatParentID, userMessage); err != nil {
			return fmt.Errorf("chat execution failed: %w", err)
		}

		return nil
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatMessage, "message", "", "The user message to send")
	chatCmd.Flags().StringVar(&chatFile, "file", "", "Path to a file containing the user message")
}
