/**
 * Component: Tree Dump Strategy
 * Block-UUID: fb69ffe3-70dd-4b68-9257-3fbde496c79e
 * Parent-UUID: 83116fba-cc68-4527-a081-447e984ddd12
 * Version: 1.0.2
 * Description: Implements the legacy 'tree' dump strategy, creating a hierarchical filesystem tree mirroring chat structure.
 * Language: Go
 * Created-at: 2026-03-10T01:14:29.620Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package contract

import (
	"errors"
	"fmt"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// executeTreeDump implements the logic for the legacy 'tree' dump type.
func executeTreeDump(chats []db.Chat, sqliteDB *sql.DB, writer DumpWriter, outputDir string, includeSystem bool, trim bool, debugPatch bool) error {
	// Prepare the output directory to ensure a clean state
	if err := writer.Prepare(outputDir); err != nil {
		return fmt.Errorf("failed to prepare output directory: %w", err)
	}

	// PASS 1: Indexing
	blockMap := make(map[string]string)

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

	// PASS 2: Generation
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
				if err := writer.WritePatch(absMsgDir, patch, trim); err != nil {
					return err
				}

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
					if debugPatch {
						var pErr *PatchError
						phase1Diff := patch.ExecutableCode
						phase2Diff := ""
						
						// Extract diffs from the error if it's a PatchError
						if errors.As(err, &pErr) {
							phase1Diff = pErr.Phase1Diff
							phase2Diff = pErr.Phase2Diff
						}

						debugPath, writeErr := WriteDebugArtifacts(msg, chat, sourceCode, phase1Diff, phase2Diff, patch.TargetBlockUUID, err)
						if writeErr != nil {
							logger.Error("Failed to write debug artifacts", "error", writeErr)
						} else {
							logger.Warning("Failed to apply patch", "target_uuid", patch.TargetBlockUUID, "error", err, "debug_dir", debugPath)
						}
					} else {
						logger.Warning("Failed to apply patch", "target_uuid", patch.TargetBlockUUID, "error", err)
					}
					continue
				}

				header := generateCodeHeaderFromPatch(patch)
				if err := writer.WritePatchedFile(absMsgDir, patch, header, patchedCode); err != nil {
					return err
				}

				if patch.TargetBlockUUID != "" {
					blockMap[patch.TargetBlockUUID] = patchedCode
				}
			}

			visibleIndex++
		}
	}
	return nil
}
