/**
 * Component: Contract Send Command
 * Block-UUID: 1080fe53-8e6b-4c41-8174-25cd9de12cff
 * Parent-UUID: 00af2db7-08a3-4609-a8e7-97e80ea21b8a
 * Version: 1.1.1
 * Description: Exported SendCmd to allow registration as a top-level alias in the root CLI.
 * Language: Go
 * Created-at: 2026-03-10T22:44:11.039Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1)
 */


package contract

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/cli/send"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
	"github.com/AlecAivazis/survey/v2"
)

var (
	sendChatID         int64
	sendFile           string
	sendMdBefore       string
	sendMdAfter        string
	sendWrap           string
	sendVisibility     string
	sendNoSizeLimit    bool
	sendAutoSelect     bool
	sendNoConfirmation bool
)

// SendCmd represents the 'gsc contract send' command.
// It is exported to allow registration as a top-level alias ('gsc send').
var SendCmd = &cobra.Command{
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
	SendCmd.Flags().Int64Var(&sendChatID, "chat-id", 0, "The ID of the chat to send to")
	SendCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (optional if in workspace)")
	SendCmd.Flags().StringVar(&sendFile, "file", "", "Read content from a file")
	SendCmd.Flags().StringVar(&sendMdBefore, "md-before", "", "Prepend Markdown text")
	SendCmd.Flags().StringVar(&sendMdAfter, "md-after", "", "Append Markdown text")
	SendCmd.Flags().StringVar(&sendWrap, "wrap", "", "Wrap output in a code block")
	SendCmd.Flags().StringVar(&sendVisibility, "visibility", "human-public", "Message visibility")
	SendCmd.Flags().BoolVar(&sendNoSizeLimit, "no-size-limit", false, "Skip confirmation for large files")
	SendCmd.Flags().BoolVar(&sendNoConfirmation, "no-chat-confirmation", false, "Bypass UI confirmation")
	SendCmd.Flags().BoolVar(&sendAutoSelect, "auto-select", false, "Automatically select the chat if only one exists")

	contractCmd.AddCommand(SendCmd)
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
		// Smart Chat Selection
		gscHome, err := settings.GetGSCHome(true)
		if err != nil {
			return fmt.Errorf("failed to get GSC home: %w", err)
		}

		sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
		if err != nil {
			return fmt.Errorf("failed to open chats database: %w", err)
		}
		defer db.CloseDB(sqliteDB)

		chats, err := db.GetChatsByContractUUID(sqliteDB, uuid)
		if err != nil {
			return fmt.Errorf("failed to query chats: %w", err)
		}

		if len(chats) == 0 {
			return fmt.Errorf("no chats found for contract %s", uuid)
		} else if len(chats) == 1 {
			targetChat := chats[0]
			if sendAutoSelect {
				chatID = targetChat.ID
			} else {
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("Send to '%s' (ID: %d)?", targetChat.Name, targetChat.ID),
				}
				var confirm bool
				if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
					return fmt.Errorf("send cancelled")
				}
				chatID = targetChat.ID
			}
		} else {
			return fmt.Errorf("multiple chats found for contract %s. Please specify --chat-id.", uuid)
		}
	}

	if chatID == 0 {
		// Try environment (if in a workspace)
		if envChatID := os.Getenv("GSC_CHAT_ID"); envChatID != "" {
			fmt.Sscanf(envChatID, "%d", &chatID)
		}
	}

	if chatID == 0 {
		return fmt.Errorf("could not determine chat ID")
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
		NoSizeLimit:    sendNoSizeLimit,
		NoConfirmation: sendNoConfirmation,
	}

	return send.Perform(opts)
}
