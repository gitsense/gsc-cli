/**
 * Component: Contract CLI Messages
 * Block-UUID: 293a90d0-3ec8-49ff-b0e0-23e31665eebc
 * Parent-UUID: ea0ab162-f666-453f-9da7-47ed781148f4
 * Version: 1.1.1
 * Description: Implements the 'gsc contract messages' command to list and filter messages, supporting truncation, latest slicing, and visual block formatting.
 * Language: Go
 * Created-at: 2026-03-10T16:19:29.714Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.1)
 */


package contract

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/internal/output"
)

// ==========================================
// Flags
// ==========================================

var (
	contractMessagesFormat      string
	contractMessagesID          int64
	contractMessagesIDs         string
	contractMessagesLatest      int
	contractMessagesFullContent bool
	contractMessagesType        string
	contractMessagesRole        string
	contractMessagesVisibility  string
	contractMessagesChatID      int64
)

// ==========================================
// Command Definition
// ==========================================

// MessagesCmd implements the 'gsc contract messages' command
var MessagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "List and filter messages in the current chat",
	Long: `Displays messages from the active chat, supporting filtering by type, role, 
and visibility. By default, content is truncated to 5 lines for readability.`,
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

		sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
		if err != nil {
			return fmt.Errorf("failed to open chat database: %w", err)
		}
		defer sqliteDB.Close()

		// 3. Resolve Chat ID
		chatID, err := resolveChatID(sqliteDB, uuid, contractMessagesChatID)
		if err != nil {
			return err
		}

		// 4. Fetch Messages
		messages, err := db.GetMessagesRecursive(sqliteDB, chatID)
		if err != nil {
			return fmt.Errorf("failed to query messages: %w", err)
		}

		// 5. Filter Messages
		messages = filterMessages(messages, contractMessagesType, contractMessagesRole, contractMessagesVisibility)

		// 6. Select Specific Messages (ID or IDs)
		if contractMessagesID > 0 {
			messages = filterByID(messages, contractMessagesID)
		} else if contractMessagesIDs != "" {
			ids, err := parseIDs(contractMessagesIDs)
			if err != nil {
				return fmt.Errorf("invalid --message-ids format: %w", err)
			}
			messages = filterByIDs(messages, ids)
		}

		// 7. Slice Latest
		if contractMessagesLatest > 0 {
			messages = sliceLatest(messages, contractMessagesLatest)
		}

		// 8. Output
		if contractMessagesFormat == "json" {
			output.FormatJSON(messages)
			return nil
		}

		// Human Format: Visual Blocks
		printVisualBlocks(messages, contractMessagesFullContent)
		return nil
	},
}

// ==========================================
// Logic Handlers
// ==========================================

// resolveChatID determines the chat ID based on priority:
// 1. Explicit --chat-id flag
// 2. GSC_CHAT_ID environment variable
// 3. Auto-detect (1 chat = ok, >1 = error)
func resolveChatID(dbConn *sql.DB, contractUUID string, explicitID int64) (int64, error) {
	if explicitID > 0 {
		return explicitID, nil
	}

	// Check Environment Variable
	if envID := os.Getenv("GSC_CHAT_ID"); envID != "" {
		id, err := strconv.ParseInt(envID, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid GSC_CHAT_ID environment variable: %w", err)
		}
		return id, nil
	}

	// Auto-detect
	chats, err := db.GetChatsByContractUUID(dbConn, contractUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to query chats for auto-detection: %w", err)
	}

	if len(chats) == 0 {
		return 0, fmt.Errorf("no chats found for this contract")
	}

	if len(chats) > 1 {
		var sb strings.Builder
		sb.WriteString("Multiple chats found for this contract. Please specify --chat-id:\n")
		for _, c := range chats {
			sb.WriteString(fmt.Sprintf("  - ID: %d | Name: %s\n", c.ID, c.Name))
		}
		return 0, fmt.Errorf(sb.String())
	}

	return chats[0].ID, nil
}

// filterMessages applies type, role, and visibility filters
func filterMessages(messages []db.Message, msgType, role, visibility string) []db.Message {
	var result []db.Message
	for _, msg := range messages {
		if msgType != "" && msg.Type != msgType {
			continue
		}
		if role != "" && msg.Role != role {
			continue
		}
		if visibility != "" && msg.Visibility != visibility {
			continue
		}
		result = append(result, msg)
	}
	return result
}

// filterByID returns a single message if found
func filterByID(messages []db.Message, id int64) []db.Message {
	for _, msg := range messages {
		if msg.ID == id {
			return []db.Message{msg}
		}
	}
	return []db.Message{}
}

// parseIDs converts a comma-separated string to a slice of int64
func parseIDs(idsStr string) ([]int64, error) {
	parts := strings.Split(idsStr, ",")
	var ids []int64
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// filterByIDs returns messages matching the provided IDs
func filterByIDs(messages []db.Message, ids []int64) []db.Message {
	// Create a map for O(1) lookup
	idMap := make(map[int64]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	var result []db.Message
	for _, msg := range messages {
		if idMap[msg.ID] {
			result = append(result, msg)
		}
	}
	return result
}

// sliceLatest returns the last N messages, preserving chronological order
func sliceLatest(messages []db.Message, n int) []db.Message {
	if len(messages) <= n {
		return messages
	}
	return messages[len(messages)-n:]
}

// printVisualBlocks prints messages in the "Visual Block" format
func printVisualBlocks(messages []db.Message, fullContent bool) {
	for _, msg := range messages {
		if !msg.Message.Valid {
			continue
		}

		fmt.Println("==========================================")
		fmt.Printf("ID:      %d\n", msg.ID)
		fmt.Printf("Role:    %s\n", msg.Role)
		fmt.Printf("Type:    %s\n", msg.Type)
		fmt.Printf("View:    %s\n", msg.Visibility)
		fmt.Printf("Created: %s\n", msg.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println("-----------------------------------------")
		
		content := msg.Message.String
		if !fullContent {
			content = truncateContent(content)
		}
		
		fmt.Println(content)
		fmt.Println() // Extra newline for separation
	}
}

// truncateContent limits content to the first 5 lines
func truncateContent(content string) string {
	if content == "" {
		return content
	}
	
	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		return strings.Join(lines[:10], "\n") + "\n\n... (truncated)"
	}
	return content
}

// ==========================================
// Initialization
// ==========================================

func init() {
	MessagesCmd.Flags().StringVarP(&contractMessagesFormat, "format", "f", "human", "Output format: human or json")
	MessagesCmd.Flags().Int64Var(&contractMessagesID, "id", 0, "Show a specific message by ID")
	MessagesCmd.Flags().StringVar(&contractMessagesIDs, "message-ids", "", "Show specific messages (comma-separated IDs)")
	MessagesCmd.Flags().IntVar(&contractMessagesLatest, "latest", 0, "Show the last N messages")
	MessagesCmd.Flags().BoolVar(&contractMessagesFullContent, "full-content", false, "Show full message content (overrides truncation)")
	MessagesCmd.Flags().StringVar(&contractMessagesType, "type", "", "Filter by message type")
	MessagesCmd.Flags().StringVar(&contractMessagesRole, "role", "", "Filter by role (user, assistant)")
	MessagesCmd.Flags().StringVar(&contractMessagesVisibility, "visibility", "", "Filter by visibility")
	MessagesCmd.Flags().Int64Var(&contractMessagesChatID, "chat-id", 0, "Explicitly select a chat ID")
}
