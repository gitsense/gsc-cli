/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: 093b40ed-b5df-42c0-9786-ce5299ec76c7
 * Parent-UUID: 4bc1b270-cf8a-42d5-85c9-cfc594adc300
 * Version: 1.9.0
 * Description: Refactored ExecuteDump to implement a two-pass logic. Pass 1 indexes all code blocks and patches. Pass 2 generates the filesystem tree and applies patches. Includes generateCodeHeaderFromPatch to reconstruct language-specific GitSense headers for patched files.
 * Language: Go
 * Created-at: 2026-03-03T07:38:59.550Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0), Gemini 3 Flash (v1.8.1), Gemini 3 Flash (v1.8.2), Gemini 3 Flash (v1.9.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

				// 3. Persist the patched result with a reconstructed Code Block header
				header := generateCodeHeaderFromPatch(patch)
				if err := writer.WritePatchedFile(absMsgDir, patch, header, patchedCode); err != nil {
					return err
				}

				// 4. Update the map to support Patch Chaining
				if patch.TargetBlockUUID != "" {
					blockMap[patch.TargetBlockUUID] = patchedCode
				}
			}

			visibleIndex++
		}
	}

	return nil
}

// generateCodeHeaderFromPatch constructs a Code Block header from a PatchBlock.
// This is used to create the header for .patched files, which represent a new version of the code.
func generateCodeHeaderFromPatch(patch markdown.PatchBlock) string {
	lang := strings.ToLower(patch.Language)

	// Define comment styles based on GitSense specifications
	var start, end, linePrefix string

	switch {
	case lang == "python":
		start = `"""`
		end = `"""`
		linePrefix = ""
	case lang == "ruby":
		start = "=begin"
		end = "=end"
		linePrefix = ""
	case lang == "bash" || lang == "sh" || lang == "zsh" || lang == "fish":
		start = ""
		end = ""
		linePrefix = "# "
	case lang == "html" || lang == "xml" || lang == "svg" || lang == "markdown" || lang == "md":
		start = "<!--"
		end = "-->"
		linePrefix = ""
	case lang == "sql":
		start = ""
		end = ""
		linePrefix = "-- "
	default:
		// Default to C-style (C, C++, Go, Java, JS, TS, etc.)
		start = "/*"
		end = "*/"
		linePrefix = " * "
	}

	var sb strings.Builder

	// Start block
	if start != "" {
		sb.WriteString(start + "\n")
	}

	// Write fields
	writeField := func(key, value string) {
		if linePrefix != "" {
			sb.WriteString(fmt.Sprintf("%s%s: %s\n", linePrefix, key, value))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	writeField("Component", patch.Component)
	writeField("Block-UUID", patch.TargetBlockUUID)
	writeField("Parent-UUID", patch.SourceBlockUUID)
	writeField("Version", patch.TargetVersion)
	writeField("Description", patch.Description)
	writeField("Language", patch.Language)
	writeField("Created-at", patch.CreatedAt)
	writeField("Authors", strings.Join(patch.Authors, ", "))

	// End block
	if end != "" {
		// Align the closing delimiter for C-style comments
		if start == "/*" {
			sb.WriteString(" " + end)
		} else {
			sb.WriteString(end)
		}
	}

	return sb.String()
}
