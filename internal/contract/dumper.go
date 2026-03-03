/*
 * Component: Contract Dump Orchestrator
 * Block-UUID: 05e93d1d-562d-4c89-863b-76d8f3f3c11f
 * Parent-UUID: b6245ce7-2d73-434d-aa84-42f85e85a5c0
 * Version: 1.2.0
 * Description: Updated GetDefaultDumpDir to use the first 12 characters of the Contract UUID. This provides 48 bits of entropy (1 in 281 trillion collision chance), which is safe for the global root directory namespace while significantly shortening the path.
 * Language: Go
 * Created-at: 2026-03-03T02:23:17.722Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// GetDefaultDumpDir returns the standard path for contract dumps: ~/.gitsense/dumps/<uuid>
// It truncates the UUID to 12 characters to shorten the path while maintaining safety (48 bits of entropy).
func GetDefaultDumpDir(uuid string) string {
	gscHome, _ := settings.GetGSCHome(false)
	
	// Truncate to 12 chars for the root directory (Safe: 1 in 281 trillion collision chance)
	shortUUID := uuid
	if len(shortUUID) > 12 {
		shortUUID = shortUUID[:12]
	}
	
	return filepath.Join(gscHome, "dumps", shortUUID)
}

// ExecuteDump coordinates the full dump process for a given contract.
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string) error {
	// 1. Initialize Output
	if err := writer.Prepare(outputDir); err != nil {
		return fmt.Errorf("failed to prepare dump directory: %w", err)
	}

	// 2. Open Database
	gscHome, _ := settings.GetGSCHome(false)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return err
	}
	defer sqliteDB.Close()

	// 3. Find all chats associated with this contract
	// We query the 'meta' JSON field for the contract_uuid
	query := `
		SELECT 
			id, uuid, name, type 
		FROM 
			chats 
		WHERE id IN (
			SELECT chat_id FROM messages WHERE type = 'gsc-cli-contract' AND json_extract(meta, '$.contract_uuid') = ? AND deleted = 0
		)`
		
	rows, err := sqliteDB.Query(query, contractUUID)
	if err != nil {
		return fmt.Errorf("failed to query chats for contract: %w", err)
	}
	defer rows.Close()

	var chats []db.Chat
	for rows.Next() {
		var c db.Chat
		if err := rows.Scan(&c.ID, &c.UUID, &c.Name, &c.Type); err != nil {
			return err
		}
		chats = append(chats, c)
	}

	if len(chats) == 0 {
		return fmt.Errorf("no chats found for contract %s", contractUUID)
	}

	// 4. Process each chat
	for _, chat := range chats {
		logger.Info("Dumping chat", "name", chat.Name, "id", chat.ID)

		// Fetch messages in recursive order
		messages, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch messages for chat %d: %w", chat.ID, err)
		}

		for pos, msg := range messages {
			if !msg.Message.Valid {
				continue
			}

			// Determine directory for this message
			relMsgDir := writer.GetMessageDir(chat, msg, pos)
			absMsgDir := filepath.Join(outputDir, relMsgDir)

			if err := os.MkdirAll(absMsgDir, 0755); err != nil {
				return err
			}

			// Write the message context
			if err := writer.WriteMessage(absMsgDir, msg); err != nil {
				return err
			}

			// Extract and write blocks
			result, err := markdown.ExtractCodeBlocks(msg.Message.String)
			if err != nil {
				logger.Warning("Failed to parse markdown for message", "id", msg.ID, "error", err)
				continue
			}

			for _, block := range result.Blocks {
				if err := writer.WriteBlock(absMsgDir, block); err != nil {
					return err
				}
			}

			for _, patch := range result.Patches {
				if err := writer.WritePatch(absMsgDir, patch); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
