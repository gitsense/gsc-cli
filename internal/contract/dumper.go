/*
 * Component: Contract Dump Orchestrator
 * Block-UUID: b6245ce7-2d73-434d-aa84-42f85e85a5c0
 * Parent-UUID: 834f1034-768b-4306-ba03-f292d5b7c5c8
 * Version: 1.1.0
 * Description: Added GetDefaultDumpDir helper to resolve the standard output path for conversational dumps.
 * Language: Go
 * Created-at: 2026-03-03T02:23:17.722Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
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
func GetDefaultDumpDir(uuid string) string {
	gscHome, _ := settings.GetGSCHome(false)
	return filepath.Join(gscHome, "dumps", uuid)
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
	query := `SELECT id, uuid, name, type FROM chats WHERE json_extract(meta, '$.contract_uuid') = ? AND deleted = 0`
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
