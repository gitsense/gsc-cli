/*
 * Component: Contract Dump Orchestrator
 * Block-UUID: 0785a195-4756-4b2a-ad4b-dad9ad6ce9ed
 * Parent-UUID: 771fa35a-9dbc-4624-8da5-c437fc50efec
 * Version: 1.6.0
 * Description: Refactored ExecuteDump to implement a two-pass logic. Pass 1 indexes all code blocks and patches into a global map to resolve late-arriving context. Pass 2 generates the filesystem tree, applies patches using the index, and supports patch chaining.
 * Language: Go
 * Created-at: 2026-03-03T05:24:49.772Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
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
	
	shortUUID := uuid
	if len(shortUUID) > 12 {
		shortUUID = shortUUID[:12]
	}
	
	return filepath.Join(gscHome, "dumps", shortUUID)
}

// ExecuteDump coordinates the full dump process for a given contract using a two-pass approach.
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool, trim bool) error {
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

	// blockMap stores Block-UUID -> ExecutableCode content
	blockMap := make(map[string]string)

	// ==========================================
	// PASS 1: Indexing (The Intent Library)
	// ==========================================
	// We scan all messages to find every code block and patch to ensure 
	// we can resolve patches even if the source appears later in the chat.
	for _, chat := range chats {
		messages, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch messages for indexing chat %d: %w", chat.ID, err)
		}

		for _, msg := range messages {
			if !msg.Message.Valid {
				continue
			}

			result, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
			if err != nil {
				continue
			}

			for _, block := range result.Blocks {
				if block.BlockUUID != "" {
					blockMap[block.BlockUUID] = block.ExecutableCode
				}
			}
			// Note: We don't index patches in Pass 1 because their "content" 
			// is the result of an application, which happens in Pass 2.
		}
	}

	// ==========================================
	// PASS 2: Generation (The Filesystem Tree)
	// ==========================================
	for _, chat := range chats {
		logger.Info("Dumping chat", "name", chat.Name, "id", chat.ID)

		messages, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch messages for generating chat %d: %w", chat.ID, err)
		}

		visibleIndex := 1
		for _, msg := range messages {
			if !msg.Message.Valid {
				continue
			}

			if !includeSystem && msg.Role == "system" {
				continue
			}

			relMsgDir := writer.GetMessageDir(chat, msg, visibleIndex)
			absMsgDir := filepath.Join(outputDir, relMsgDir)

			if err := os.MkdirAll(absMsgDir, 0755); err != nil {
				return err
			}

			if err := writer.WriteMessage(absMsgDir, msg); err != nil {
				return err
			}

			result, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
			if err != nil {
				logger.Warning("Failed to parse markdown for message", "id", msg.ID, "error", err)
				continue
			}

			// Write standard code blocks
			for _, block := range result.Blocks {
				if err := writer.WriteBlock(absMsgDir, block, trim); err != nil {
					return err
				}
			}

			// Process patches and generate "patched" files
			for _, patch := range result.Patches {
				// 1. Write the .diff file
				if err := writer.WritePatch(absMsgDir, patch, trim); err != nil {
					return err
				}

				// 2. Attempt to generate the .patched file
				if patch.SourceBlockUUID == "" {
					continue
				}

				sourceCode, ok := blockMap[patch.SourceBlockUUID]
				if !ok {
					logger.Warning("Source block not found for patch", "source_uuid", patch.SourceBlockUUID, "msg_id", msg.ID)
					continue
				}

				patchedCode, err := ApplyPatch(sourceCode, patch.ExecutableCode)
				if err != nil {
					logger.Warning("Failed to apply patch", "target_uuid", patch.TargetBlockUUID, "error", err)
					continue
				}

				// 3. Persist the patched result
				if err := writer.WritePatchedFile(absMsgDir, patch, patchedCode); err != nil {
					return err
				}

				// 4. Update the map to support Patch Chaining
				// This allows a subsequent patch to use this result as its source.
				if patch.TargetBlockUUID != "" {
					blockMap[patch.TargetBlockUUID] = patchedCode
				}
			}

			visibleIndex++
		}
	}

	return nil
}
