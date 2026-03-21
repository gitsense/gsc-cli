/**
 * Component: Contract CLI Chats
 * Block-UUID: ee44f8a9-7270-4942-b6b6-a7a2ac455d25
 * Parent-UUID: 8b15d10e-9ad3-4b02-b6f4-52c28a151180
 * Version: 1.2.0
 * Description: Implements the 'gsc contract chats' command to list chats associated with a contract, supporting type discovery and JSON output. Removed duplicate resolveContractUUID helper.
 * Language: Go
 * Created-at: 2026-03-21T04:04:36.288Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package contract

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ==========================================
// Flags
// ==========================================

var (
	contractChatsTypes  string
	contractChatsFormat string
)

// ==========================================
// Command Definition
// ==========================================

// ChatsCmd implements the 'gsc contract chats' command
var ChatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "List chats associated with the current contract",
	Long: `Lists all chats (threads) associated with the active contract. 
This is useful for identifying specific conversation forks or branches.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(contractDumpUUID)
		if err != nil {
			return err
		}

		// 2. Open Database
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// Resolve DB path (Native or Docker)
		dbPath := db.ResolveDBPath(settings.GetChatDatabasePath(gscHome))
		sqliteDB, err := db.OpenDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open chat database: %w", err)
		}
		defer sqliteDB.Close()

		// 3. Fetch Chats
		chats, err := db.GetChatsByContractUUID(sqliteDB, uuid)
		if err != nil {
			return fmt.Errorf("failed to query chats: %w", err)
		}

		if len(chats) == 0 {
			fmt.Println("No chats found for this contract.")
			return nil
		}

		// 4. Handle --types flag
		if contractChatsTypes != "" {
			return handleTypesOutput(sqliteDB, chats, contractChatsFormat)
		}

		// 5. Standard Output
		return handleChatsOutput(chats, contractChatsFormat)
	},
}

// ==========================================
// Logic Handlers
// ==========================================

// handleChatsOutput formats and prints the list of chats
func handleChatsOutput(chats []db.Chat, format string) error {
	if format == "json" {
		output.FormatJSON(chats)
		return nil
	}

	// Human Format: Table
	headers := []string{"ID", "UUID", "Name", "Type"}
	rows := make([][]string, len(chats))

	for i, c := range chats {
		rows[i] = []string{
			fmt.Sprintf("%d", c.ID),
			c.UUID,
			c.Name,
			c.Type,
		}
	}

	fmt.Print(output.FormatTable(headers, rows))
	return nil
}

// handleTypesOutput aggregates and prints unique message types
func handleTypesOutput(dbConn *sql.DB, chats []db.Chat, format string) error {
	// Aggregate unique types
	typeSet := make(map[string]bool)
	
	for _, chat := range chats {
		// Fetch messages for this chat to get types
		// Note: In a production system, we might want a specific DB query for this
		// to avoid fetching all message content, but for MVP this is acceptable.
		messages, err := db.GetMessagesRecursive(dbConn, chat.ID)
		if err != nil {
			// Log warning but continue to next chat
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch messages for chat %d: %v\n", chat.ID, err)
			continue
		}

		for _, msg := range messages {
			if msg.Type != "" {
				typeSet[msg.Type] = true
			}
		}
	}

	// Convert map to slice
	var types []string
	for t := range typeSet {
		types = append(types, t)
	}

	if format == "json" {
		output.FormatJSON(types)
		return nil
	}

	// Human Format: List
	fmt.Println("Unique Message Types:")
	for _, t := range types {
		fmt.Printf("  - %s\n", t)
	}
	return nil
}

// ==========================================
// Initialization
// ==========================================

func init() {
	ChatsCmd.Flags().StringVar(&contractChatsTypes, "types", "", "Show unique message types found in the chats (e.g., 'list' or 'count')")
	ChatsCmd.Flags().StringVarP(&contractChatsFormat, "format", "f", "human", "Output format: human or json")
}
