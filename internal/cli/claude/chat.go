/**
 * Component: Claude Code Chat Command
 * Block-UUID: bf92e3cb-0300-4d93-935b-cdf5c19d9c3e
 * Parent-UUID: 32ba2926-fc23-4ee5-a3f3-abeeb610d7ba
 * Version: 1.1.0
 * Description: Updated validation to allow 'parent-id' to be 0, enabling the creation of new chat sessions.
 * Language: Go
 * Created-at: 2026-03-22T04:39:43.440Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.1.0)
 */


package claude

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		
		// parent-id can be 0 for new chats
		var userMessage string

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
