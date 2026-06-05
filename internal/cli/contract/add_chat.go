/**
 * Component: Contract CLI Add Chat
 * Block-UUID: 04dbe851-a962-4c81-baac-9ac293d81f66
 * Parent-UUID: 6dec88e2-f124-4262-92d3-290a68d8612e
 * Version: 1.3.0
 * Description: CLI command for adding an existing contract to a chat by inserting/updating the contract message.
 * Language: Go
 * Created-at: 2026-04-27T18:02:52.630Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package contract

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// addChatContractCmd handles 'gsc app contract add-chat'
var addChatContractCmd = &cobra.Command{
	Use:   "add-chat <chat-uuid>",
	Short: "Add an existing contract to a chat",
	Long: `Adds an existing contract to a chat by inserting/updating the contract message.
This is useful for auditing purposes or when you want to include a chat in a contract dump.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		
		chatUUID := args[0]
		// contractUUID := contractAddUUID // Replaced by resolveContractUUID
		force := contractAddForce

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(contractAddUUID)
		if err != nil {
			return err
		}

		// 2. Validate contract exists
		meta, err := contract.GetContract(uuid)
		if err != nil {
			return fmt.Errorf("failed to find contract with UUID '%s': %w", contractAddUUID, err)
		}

		// 2. Open chat database
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
		if err != nil {
			return fmt.Errorf("failed to open chat database: %w", err)
		}
		defer sqliteDB.Close()

		// 3. Validate chat exists
		chat, err := db.FindChatByUUID(sqliteDB, chatUUID)
		if err != nil {
			return fmt.Errorf("failed to query chat: %w", err)
		}
		if chat == nil {
			return fmt.Errorf("chat with UUID '%s' not found", chatUUID)
		}

		// 4. Check if chat already has a contract message
		existingMsg, err := db.FindMessageByRoleAndType(sqliteDB, chat.ID, "assistant", "gsc-cli-contract")
		if err != nil {
			return fmt.Errorf("failed to check for existing contract message: %w", err)
		}

		// 5. Handle existing contract message
		if existingMsg != nil && !force {
			confirm := false
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Chat '%s' already has a contract message. Do you want to update it?", chat.Name),
				Default: false,
			}
			if err := survey.AskOne(prompt, &confirm); err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// 6. Prepare contract message data from contract metadata
		dbData := db.ContractMessageData{ContractData: meta.ContractData}

		// 7. Upsert contract message
		_, err = db.UpsertContractMessage(sqliteDB, chat.ID, dbData)
		if err != nil {
			return fmt.Errorf("failed to add contract message to chat: %w", err)
		}

		// 8. Success message
		fmt.Printf("Contract '%s' successfully added to chat '%s'.\n", meta.UUID, chat.Name)
		fmt.Println("\n⚠️  IMPORTANT: You will need to reload the chat for the contract to start working.")
		fmt.Println("   If the chat is currently open in the GitSense Chat UI, please refresh the page.")

		return nil
	},
}

func init() {
	// Add Chat Flags
	addChatContractCmd.Flags().StringVar(&contractAddUUID, "contract", "", "Contract UUID (required)")
	addChatContractCmd.Flags().BoolVar(&contractAddForce, "force", false, "Force update without confirmation")

	// Register Subcommand
	contractCmd.AddCommand(addChatContractCmd)
}
