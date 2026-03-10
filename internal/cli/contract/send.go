/*
 * Component: Contract Send Command
 * Block-UUID: a8744461-de2b-47d9-b18f-6fb2211b0e97
 * Parent-UUID: N/A
 * Version: 1.0.1
 * Description: Implements the 'gsc contract send' command, allowing users to send messages to any chat associated with a contract.
 * Language: Go
 * Created-at: 2026-03-10T22:03:22.864Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1)
 */


package contract

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/cli/send"
	"github.com/spf13/cobra"
)

var (
	sendChatID         int64
	sendFile           string
	sendMdBefore       string
	sendMdAfter        string
	sendWrap           string
	sendVisibility     string
	sendForce          bool
	sendNoConfirmation bool
)

var sendContractCmd = &cobra.Command{
	Use:   "send [text]",
	Short: "Send a message to a contract chat",
	Long: `Sends a message to a specific chat associated with a contract.
If multiple chats exist, the --chat-id flag is required.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleContractSend(args)
	},
}

func init() {
	// Define flags locally to ensure self-containment
	sendContractCmd.Flags().Int64Var(&sendChatID, "chat-id", 0, "The ID of the chat to send to")
	sendContractCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (optional if in workspace)")
	sendContractCmd.Flags().StringVar(&sendFile, "file", "", "Read content from a file")
	sendContractCmd.Flags().StringVar(&sendMdBefore, "md-before", "", "Prepend Markdown text")
	sendContractCmd.Flags().StringVar(&sendMdAfter, "md-after", "", "Append Markdown text")
	sendContractCmd.Flags().StringVar(&sendWrap, "wrap", "", "Wrap output in a code block")
	sendContractCmd.Flags().StringVar(&sendVisibility, "visibility", "human-public", "Message visibility")
	sendContractCmd.Flags().BoolVar(&sendForce, "force", false, "Skip confirmation for large files")
	sendContractCmd.Flags().BoolVar(&sendNoConfirmation, "no-chat-confirmation", false, "Bypass UI confirmation")

	contractCmd.AddCommand(sendContractCmd)
}

func handleContractSend(args []string) error {
	// 1. Context Discovery (UUID)
	uuid := contractUUID
	if uuid == "" {
		// Try environment
		uuid = os.Getenv("GSC_CONTRACT_UUID")
	}
	if uuid == "" {
		// Try directory discovery
		var err error
		uuid, err = findContractUUIDByWorkdir()
		if err != nil {
			return fmt.Errorf("could not determine contract UUID. Use --uuid or run from a workspace: %w", err)
		}
	}

	// 2. Chat ID Validation
	chatID := sendChatID
	if chatID == 0 {
		// Try environment (if in a workspace)
		if envChatID := os.Getenv("GSC_CHAT_ID"); envChatID != "" {
			fmt.Sscanf(envChatID, "%d", &chatID)
		}
	}

	if chatID == 0 {
		return fmt.Errorf("--chat-id is required when sending from outside a specific workspace home")
	}

	// 3. Map to Shared Options
	text := ""
	if len(args) > 0 {
		text = args[0]
	}

	opts := send.Options{
		ContractUUID:   uuid,
		ChatID:         chatID,
		Text:           text,
		File:           sendFile,
		MdBefore:       sendMdBefore,
		MdAfter:        sendMdAfter,
		Wrap:           sendWrap,
		Visibility:     sendVisibility,
		Force:          sendForce,
		NoConfirmation: sendNoConfirmation,
	}

	return send.Perform(opts)
}
