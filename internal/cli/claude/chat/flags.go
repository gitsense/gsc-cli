/**
 * Component: Chat CLI Flags and Options
 * Block-UUID: 8ecd7aa0-853d-42dd-a9e6-6c26d4ccc8b7
 * Parent-UUID: 8f7a3b2c-1d4e-4f5a-9b8c-7d6e5f4a3b2c
 * Version: 1.1.0
 * Description: Added Mode and NoEvents flags to support different chat types and batch processing.
 * Language: Go
 * Created-at: 2026-04-01T15:31:15.123Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package chat

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ChatFlags contains all flags for the chat command
type ChatFlags struct {
	Message        string
	File           string
	Format         string
	Append         bool
	Save           bool
	Model          string
	AppendSave     bool
	ThinkingBudget int
	Mode           string
	NoEvents       bool
}

// RegisterChatFlags registers all chat command flags
func RegisterChatFlags(cmd *cobra.Command, flags *ChatFlags) {
	cmd.Flags().StringVar(&flags.Message, "message", "", "The user message to send")
	cmd.Flags().StringVar(&flags.File, "file", "", "Path to a file containing the user message")
	cmd.Flags().StringVar(&flags.Format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&flags.Append, "append", false, "Automatically append to the latest message in the chat")
	cmd.Flags().BoolVar(&flags.Save, "save", false, "Save the response to the database")
	cmd.Flags().StringVar(&flags.Model, "model", "", "The model to use (e.g., claude-3-5-sonnet)")
	cmd.Flags().BoolVar(&flags.AppendSave, "append-save", false, "Save the user message to the database and append to the latest message")
	cmd.Flags().IntVar(&flags.ThinkingBudget, "thinking", 0, "Thinking budget in tokens (0 = disabled)")
	cmd.Flags().StringVar(&flags.Mode, "mode", "coding-assistant", "The mode of the chat session (e.g., coding-assistant, analyze)")
	cmd.Flags().BoolVar(&flags.NoEvents, "no-events", false, "Suppress streaming events and output only the final response text")
}

// ValidateChatFlags validates chat command flags
func ValidateChatFlags(flags *ChatFlags) error {
	// Validate format
	if flags.Format != "text" && flags.Format != "json" {
		return fmt.Errorf("invalid format: %s (must be 'text' or 'json')", flags.Format)
	}

	// Validate thinking budget
	if flags.ThinkingBudget < 0 {
		return fmt.Errorf("thinking budget must be >= 0 (got %d)", flags.ThinkingBudget)
	}

	// Validate file exists if provided
	if flags.File != "" {
		if _, err := os.Stat(flags.File); err != nil {
			return fmt.Errorf("file not found: %s", flags.File)
		}
	}

	return nil
}
