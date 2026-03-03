/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: 762a78c7-7e7e-428f-82d2-59f9610c2ffa
 * Parent-UUID: 0e6efd67-e8f0-43aa-9a2a-3b423ff6cdcb
 * Version: 1.4.0
 * Description: Updated message processing loop to track a visibleIndex counter. This ensures that when system messages are excluded, the directory numbering starts at 002, providing a clear signal to the user about the hidden context.
 * Language: Go
 * Created-at: 2026-03-03T04:22:22.000Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.4.0)
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
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool) error {
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

		// Track the visible index for directory naming
		visibleIndex := 1

		for _, msg := range messages {
			if !msg.Message.Valid {
				continue
			}

			// Skip system messages unless explicitly requested
			if !includeSystem && msg.Role == "system" {
				continue
			}

			// Determine directory for this message using the visible index
			relMsgDir := writer.GetMessageDir(chat, msg, visibleIndex)
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

			// Increment visible index only for messages actually written
			visibleIndex++
		}
	}

	return nil
}
