/**
 * Component: Workspace Send Command
 * Block-UUID: d5588b35-2841-40da-94a4-ce828c28eeea
 * Parent-UUID: 5aef8d06-32c3-48ec-a9ab-09f0a208c5bb
 * Version: 1.5.0
 * Description: Added support for message manipulation operations (replace, insert before, insert after) using workspace context and updated payload structure.
 * Language: Go
 * Created-at: 2026-03-10T22:35:55.838Z
 * Authors: Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.4.0), GLM-4.7 (v1.5.0)
 */


package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/gitsense/gsc-cli/internal/cli/send"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/spf13/cobra"
)

var (
	sendFile          string
	sendMdBefore      string
	sendMdAfter       string
	sendWrap          string
	sendVisibility    string
	sendNoSizeLimit   bool
	sendNoConfirmation bool
	
	// New flags for message manipulation
	sendReplace      bool
	sendInsertBefore bool
	sendInsertAfter  bool
)

// sendCmd represents the 'gsc ws send' command
var sendCmd = &cobra.Command{
	Use:   "send [text]",
	Short: "Send a message from the terminal to the chat",
	Long: `Sends a message to the active chat session via the contract events database.
Supports piping, file input, markdown formatting, and message manipulation operations.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleSend(args)
	},
}

func init() {
	sendCmd.Flags().StringVar(&sendFile, "file", "", "Read content from a file")
	sendCmd.Flags().StringVar(&sendMdBefore, "md-before", "", "Prepend Markdown text")
	sendCmd.Flags().StringVar(&sendMdAfter, "md-after", "", "Append Markdown text")
	sendCmd.Flags().StringVar(&sendWrap, "wrap", "", "Wrap output in a code block (e.g., 'bash', 'python')")
	sendCmd.Flags().StringVar(&sendVisibility, "visibility", "human-public", "Message visibility: 'human-public' or 'human-only'")
	sendCmd.Flags().BoolVar(&sendNoSizeLimit, "no-size-limit", false, "Skip confirmation for large files")
	sendCmd.Flags().BoolVar(&sendNoConfirmation, "no-chat-confirmation", false, "Bypass the UI confirmation modal")

	// Register new manipulation flags
	sendCmd.Flags().BoolVar(&sendReplace, "replace-message", false, "Replace the current workspace message")
	sendCmd.Flags().BoolVar(&sendInsertBefore, "insert-before-message", false, "Insert before the current workspace message")
	sendCmd.Flags().BoolVar(&sendInsertAfter, "insert-after-message", false, "Insert after the current workspace message")
}

func handleSend(args []string) error {
	// 1. Context Validation
	contractUUID := os.Getenv("GSC_CONTRACT_UUID")
	if contractUUID == "" {
		return fmt.Errorf("not in a GitSense workspace. GSC_CONTRACT_UUID not set.")
	}

	chatIDStr := os.Getenv("GSC_CHAT_ID")
	if chatIDStr == "" {
		return fmt.Errorf("not in a GitSense workspace. GSC_CHAT_ID not set.")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid GSC_CHAT_ID: %w", err)
	}

	// 2. Flag Validation (Mutual Exclusion)
	operationFlags := []bool{sendReplace, sendInsertBefore, sendInsertAfter}
	activeFlags := 0
	for _, flag := range operationFlags {
		if flag {
			activeFlags++
		}
	}
	if activeFlags > 1 {
		return fmt.Errorf("only one message manipulation flag can be used at a time")
	}

	// 3. Context Resolution (Reference Message ID)
	var referenceMessageID int64
	if activeFlags > 0 {
		// We need to read workspace.json to find the reference message ID
		workspacePath := "workspace.json"
		if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
			return fmt.Errorf("workspace.json not found. Cannot determine reference message ID for operation.")
		}

		data, err := os.ReadFile(workspacePath)
		if err != nil {
			return fmt.Errorf("failed to read workspace.json: %w", err)
		}

		var ws contract.ShadowWorkspace
		if err := json.Unmarshal(data, &ws); err != nil {
			return fmt.Errorf("failed to parse workspace.json: %w", err)
		}

		referenceMessageID = ws.MessageID
	}

	// 4. Map to Shared Options
	text := ""
	if len(args) > 0 {
		text = args[0]
	}

	opts := send.Options{
		ContractUUID:       contractUUID,
		ChatID:             chatID,
		Text:               text,
		File:               sendFile,
		MdBefore:           sendMdBefore,
		MdAfter:            sendMdAfter,
		Wrap:               sendWrap,
		Visibility:         sendVisibility,
		NoSizeLimit:        sendNoSizeLimit,
		NoConfirmation:     sendNoConfirmation,
		ReferenceMessageID: referenceMessageID,
		Replace:            sendReplace,
		InsertBefore:       sendInsertBefore,
		InsertAfter:        sendInsertAfter,
	}

	return send.Perform(opts)
}
