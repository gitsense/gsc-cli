/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: 9bb7741e-7be2-4f31-8a9f-460b24430a8a
 * Parent-UUID: b4e4a55a-a653-45f0-8800-d856cee9537b
 * Version: 2.18.0
 * Description: Updated generateHelpFiles to write help files to the parent mapped directory and removed GSC_MAPPED_WS_HASH from replacements.
 * Language: Go
 * Created-at: 2026-03-08T15:52:30.904Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v2.11.0), GLM-4.7 (v2.12.0), GLM-4.7 (v2.13.0), GLM-4.7 (v2.14.0), GLM-4.7 (v2.14.1), GLM-4.7 (v2.15.0), GLM-4.7 (v2.16.0), GLM-4.7 (v2.17.0), GLM-4.7 (v2.18.0)
 */


package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
	"github.com/gitsense/gsc-cli/internal/search"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// MergedNode represents a unique message in the merged tree.
type MergedNode struct {
	Message          db.Message
	Hash             string
	Chats            []db.Chat
	Children         []*MergedNode
	ChatCount        int
	MaxSubtreeTime   time.Time
}

// ExecuteDump coordinates the full dump process for a given contract.
// It supports 'tree', 'merged', and 'mapped' strategies.
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool, trim bool, dumpType string, sortMode string, debugPatch bool, messageID int64, validate bool, activeChatID int64) (*MappedDumpResult, error) {
	// 1. Initialize Output (Skip if validating to avoid deleting the workspace)
	if !validate {
		if err := writer.Prepare(outputDir); err != nil {
			return nil, fmt.Errorf("failed to prepare output directory: %w", err)
		}
	}

	// 2. Open Database
	gscHome, _ := settings.GetGSCHome(false)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to query chats for contract: %w", err)
	}
	defer rows.Close()

	var chats []db.Chat
	for rows.Next() {
		var c db.Chat
		if err := rows.Scan(&c.ID, &c.UUID, &c.Name, &c.Type); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}

	if len(chats) == 0 {
		return nil, fmt.Errorf("no chats found for contract %s", contractUUID)
	}

	// ==========================================
	// STRATEGY SELECTION
	// ==========================================
	if dumpType == "mapped" {
		return executeMappedDump(contractUUID, chats, sqliteDB, writer, outputDir, includeSystem, trim, debugPatch, messageID, validate, activeChatID)
	}

	if dumpType == "merged" {
		// Merged dump doesn't return MappedDumpResult
		err := executeMergedDump(chats, sqliteDB, writer, outputDir, includeSystem, trim, sortMode, make(map[string]string), debugPatch)
		return nil, err
	}

	// ==========================================
	// LEGACY 'TREE' STRATEGY
	// ==========================================
	// PASS 1: Indexing
	blockMap := make(map[string]string)

	for _, chat := range chats {
		messages, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch messages for indexing chat %d: %w", chat.ID, err)
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
			return nil, fmt.Errorf("failed to fetch messages for generating chat %d: %w", chat.ID, err)
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
				return nil, err
			}

			if err := writer.WriteMessage(absMsgDir, msg); err != nil {
				return nil, err
			}

			result, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
			if err != nil {
				logger.Warning("Failed to parse markdown for message", "id", msg.ID, "error", err)
				continue
			}

			// Write standard code blocks
			for _, block := range result.Blocks {
				if err := writer.WriteBlock(absMsgDir, block, trim); err != nil {
					return nil, err
				}
			}

			// Process patches and generate "patched" files
			for _, patch := range result.Patches {
				if err := writer.WritePatch(absMsgDir, patch, trim); err != nil {
					return nil, err
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
					return nil, err
				}

				if patch.TargetBlockUUID != "" {
					blockMap[patch.TargetBlockUUID] = patchedCode
				}
			}

			visibleIndex++
		}
	}

	return nil, nil
}

// executeMappedDump implements the logic for the 'mapped' dump type.
// It creates a shadow workspace where code blocks are mapped to their project paths.
func executeMappedDump(contractUUID string, chats []db.Chat, sqliteDB *sql.DB, writer DumpWriter, outputDir string, includeSystem bool, trim bool, debugPatch bool, messageID int64, validate bool, activeChatID int64) (*MappedDumpResult, error) {
	// ==========================================
	// PHASE 0: Hash Calculation & Path Resolution
	// ==========================================
	// We need the hash to determine the specific workspace directory, 
	// regardless of whether we are validating or generating.
	var dumpHash string
	var messages []db.Message

	if messageID > 0 {
		// Single Message Mode
		msg, err := db.GetMessage(sqliteDB, messageID)
		if err != nil {
			return nil, fmt.Errorf("message ID %d not found: %w", messageID, err)
		}
		messages = []db.Message{*msg}
		dumpHash = calculateMessageHash(*msg)
		logger.Info("Single Message Mode", "message_id", messageID, "hash", dumpHash)
	} else {
		// Full Contract Mode
		for _, chat := range chats {
			chatMsgs, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch messages for chat %d: %w", chat.ID, err)
			}
			messages = append(messages, chatMsgs...)
		}
		// For full contract, use a fixed hash or the contract UUID itself
		dumpHash = "full-contract"
		logger.Info("Full Contract Mode", "total_messages", len(messages))
	}

	// Construct the specific workspace directory: <outputDir>/<hash>
	// outputDir is already .../dumps/<uuid>/mapped
	workspaceDir := filepath.Join(outputDir, dumpHash)

	// ==========================================
	// VALIDATION PHASE (If --validate is set)
	// ==========================================
	if validate {
		manifestPath := filepath.Join(workspaceDir, "workspace.json")
		
		// Check if manifest exists
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			// Workspace does not exist
			return &MappedDumpResult{
				Success: false,
				Exists:  false,
				Valid:   false,
				Error:   &DumpError{Code: "NOT_FOUND", Message: "Shadow workspace not found"},
			}, nil
		}

		// Read manifest
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return &MappedDumpResult{
				Success: false,
				Exists:  false,
				Valid:   false,
				Error:   &DumpError{Code: "READ_ERROR", Message: "Failed to read workspace manifest"},
			}, nil
		}

		var ws ShadowWorkspace
		if err := json.Unmarshal(data, &ws); err != nil {
			return &MappedDumpResult{
				Success: false,
				Exists:  false,
				Valid:   false,
				Error:   &DumpError{Code: "CORRUPT_MANIFEST", Message: "Failed to parse workspace manifest"},
			}, nil
		}

		// Check Expiration
		expiresAt, _ := time.Parse(time.RFC3339, ws.ExpiresAt)
		isExpired := time.Now().After(expiresAt)

		if isExpired {
			logger.Info("Shadow workspace expired, auto-extending", "hash", ws.Hash)
			// Auto-extend by 24 hours
			newExpiry := time.Now().Add(24 * time.Hour)
			ws.ExpiresAt = newExpiry.Format(time.RFC3339)
			
			// Write updated manifest back
			newData, err := json.MarshalIndent(ws, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal updated manifest: %w", err)
			}
			if err := os.WriteFile(manifestPath, newData, 0644); err != nil {
				return nil, fmt.Errorf("failed to write updated manifest: %w", err)
			}
		}

		// Return cached data
		return &MappedDumpResult{
			Success:  true,
			Exists:   true,
			Valid:    true,
			Hash:     ws.Hash,
			RootDir:  workspaceDir, // Return the specific workspace dir
			ExpiresAt: ws.ExpiresAt,
			Stats:    ws.Stats,    // Include stats from manifest
			Files:    ws.Files,
		}, nil
	}

	// ==========================================
	// GENERATION PHASE
	// ==========================================
	
	// 1. Prepare the specific workspace directory
	if err := writer.Prepare(workspaceDir); err != nil {
		return nil, fmt.Errorf("failed to prepare workspace directory: %w", err)
	}

	// PASS 0: Pre-scan for Total Artifact Count
	totalArtifacts := 0
	for _, msg := range messages {
		if !msg.Message.Valid {
			continue
		}
		if !includeSystem && msg.Role == "system" {
			continue
		}
		result, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
		if err != nil {
			continue
		}
		totalArtifacts += len(result.Blocks) + len(result.Patches)
	}
	logger.Info("Pre-scan Complete", "total_artifacts_to_process", totalArtifacts)

	// 2. Discovery Pass: Resolve Parent-UUIDs to Paths
	// Collect all unique Parent-UUIDs from the messages
	parentUUIDs := make(map[string]bool)
	for _, msg := range messages {
		if !msg.Message.Valid {
			continue
		}
		result, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
		if err != nil {
			continue
		}
		for _, block := range result.Blocks {
			if block.ParentUUID != "" && block.ParentUUID != "N/A" {
				parentUUIDs[block.ParentUUID] = true
			}
		}
		// Also check patches for SourceBlockUUIDs
		for _, patch := range result.Patches {
			if patch.SourceBlockUUID != "" && patch.SourceBlockUUID != "N/A" {
				parentUUIDs[patch.SourceBlockUUID] = true
			}
		}
	}

	// Convert map to slice
	var uuidList []string
	for uuid := range parentUUIDs {
		uuidList = append(uuidList, uuid)
	}

	logger.Info("Discovery Phase: Starting", "unique_parent_uuids", len(uuidList))

	// Resolve paths using ripgrep
	// We need the workdir from the contract metadata to support execution from any directory.
	meta, err := GetContract(contractUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract metadata: %w", err)
	}
	workdir := meta.Workdir

	logger.Debug("Using contract workdir for discovery", "uuid", contractUUID, "workdir", workdir)

	engine := &search.RipgrepEngine{}
	pathMap, err := engine.ResolvePathsByUUIDs(context.Background(), workdir, uuidList)
	if err != nil {
		logger.Warning("Batch discovery failed", "error", err)
		// Continue with empty map (everything will be unmapped)
		pathMap = make(map[string]string)
	}

	logger.Info("Discovery Phase: Complete", "resolved_count", len(pathMap), "unresolved_count", len(uuidList)-len(pathMap))

	// Inject PathMap into MappedWriter
	if mw, ok := writer.(*MappedWriter); ok {
		mw.SetPathMap(pathMap)
	}

	// 3. Generation Pass
	result := &MappedDumpResult{
		Success: true,
		Hash:    dumpHash,
		RootDir: workspaceDir, // Use the specific workspace dir
		Files:   []MappedFileEntry{},
	}

	// blockMap stores Block-UUID -> ExecutableCode content for patching
	blockMap := make(map[string]string)
	
	processedArtifacts := 0

	// Determine if we are in single message mode for naming convention
	isSingleMessage := (messageID > 0)

	for msgIndex, msg := range messages {
		if !msg.Message.Valid {
			continue
		}

		if !includeSystem && msg.Role == "system" {
			continue
		}

		// Write message.md and message.json to the workspace root
		// This is primarily useful for Single Message Mode (--message-id)
		if messageID > 0 {
			if err := writer.WriteMessage(workspaceDir, msg); err != nil {
				return nil, err
			}
		}

		// Parse message
		parseResult, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
		if err != nil {
			logger.Warning("Failed to parse markdown", "msg_id", msg.ID, "error", err)
			continue
		}

		logger.Info("Processing Message", "msg_id", msg.ID, "blocks_found", len(parseResult.Blocks), "patches_found", len(parseResult.Patches))

		// Track files processed in this message for the checklist
		var msgChecklist []string

		// Process Code Blocks
		for _, block := range parseResult.Blocks {
			// Update blockMap for future patches
			if block.BlockUUID != "" {
				blockMap[block.BlockUUID] = block.ExecutableCode
			}

			// Determine status
			relPath, isMapped := pathMap[block.ParentUUID]
			status := MappedStatusUnmapped
			reason := ""
			if isMapped {
				status = MappedStatusMapped
				logger.Debug("Block Mapped", "uuid", block.BlockUUID, "path", relPath)
			} else {
				if block.ParentUUID == "" || block.ParentUUID == "N/A" {
					reason = "no_parent_uuid"
				} else {
					reason = "parent_not_found"
				}
				logger.Debug("Block Unmapped", "uuid", block.BlockUUID, "reason", reason)
			}

			// Format the component name for unmapped files
			var originalPath string
			var formattedPath string

			if isMapped {
				formattedPath = relPath
				originalPath = relPath // For mapped files, the project path is the original reference
			} else {
				originalPath = block.Component // Capture the AI-generated name
				// Use the new naming convention
				formattedPath = formatArtifactPath(isSingleMessage, msgIndex, block.Index, block.Component)
				// Update the block's component field so the writer uses it
				block.Component = formattedPath
			}

			// Add to result list
			entry := MappedFileEntry{
				Path:         formattedPath,
				OriginalPath: originalPath,
				Status:       status,
				BlockUUID:    block.BlockUUID,
				Reason:       reason,
				Position:     block.Index, // 0-indexed position in the message
			}
			result.Files = append(result.Files, entry)

			// Write Source File if mapped
			if isMapped {
				// Read source from workdir
				sourcePath := filepath.Join(workdir, relPath)
				sourceContent, err := os.ReadFile(sourcePath)
				if err != nil {
					logger.Warning("Failed to read source file", "path", relPath, "error", err)
					// Fallback: treat as unmapped
					continue
				}
				if err := writer.WriteSourceFile(workspaceDir, relPath, string(sourceContent)); err != nil {
					return nil, err
				}
			}

			// Write Proposed File
			if err := writer.WriteBlock(workspaceDir, block, trim); err != nil {
				return nil, err
			}

			// Log Full Code
			logger.Debug("Writing Full Code", "uuid", block.BlockUUID, "language", block.Language, "code_length", len(block.ExecutableCode))
			logger.Debug("Code Content", "uuid", block.BlockUUID, "content", block.ExecutableCode)

			// Write Provenance
			prov := Provenance{
				FilePath:     relPath,
				BlockUUID:    block.BlockUUID,
				ParentUUID:   block.ParentUUID,
				Version:      block.Version,
				ChatID:       msg.ChatID,
				MessageID:    msg.ID,
				ContractUUID: contractUUID,
				Model:        msg.RealModel.String,
				Timestamp:    msg.CreatedAt.Format(time.RFC3339),
				Action:       "full_code",
				Authors:      block.Authors,
			}
			if err := writer.WriteProvenanceJSON(workspaceDir, prov); err != nil {
				logger.Warning("Failed to write provenance", "block_uuid", block.BlockUUID, "error", err)
			}

			// Add to checklist
			checkItem := fmt.Sprintf("[x] %s (%s)", entry.Path, status)
			msgChecklist = append(msgChecklist, checkItem)
			processedArtifacts++
		}

		// Process Patches
		for _, patch := range parseResult.Patches {
			// Update blockMap
			if patch.TargetBlockUUID != "" {
				// We need the patched code for the map, but we haven't applied it yet.
				// This is tricky. For mapped dump, we rely on the 'proposed' file written by WritePatchedFile.
				// We don't need to update blockMap for subsequent patches in the same message because
				// patches usually apply to the *original* source, not the result of previous patches in the same message.
				// However, if there are multiple patches to the same file in one message, we might need to chain them.
				// For simplicity, we assume patches apply to the source file found via ParentUUID.
			}

			// Determine status
			relPath, isMapped := pathMap[patch.SourceBlockUUID]
			status := MappedStatusUnmapped
			reason := ""
			if isMapped {
				status = MappedStatusMapped
				logger.Debug("Patch Mapped", "target_uuid", patch.TargetBlockUUID, "path", relPath)
			} else {
				reason = "parent_not_found"
				logger.Debug("Patch Unmapped", "target_uuid", patch.TargetBlockUUID, "reason", reason)
			}

			// Format the component name for unmapped files
			var originalPath string
			var formattedPath string

			if isMapped {
				formattedPath = relPath
				originalPath = relPath
			} else {
				originalPath = patch.Component
				// Use the new naming convention
				formattedPath = formatArtifactPath(isSingleMessage, msgIndex, patch.Index, patch.Component)
				// Update the patch's component field so the writer uses it
				patch.Component = formattedPath
			}

			// Add to result list
			entry := MappedFileEntry{
				Path:         formattedPath,
				OriginalPath: originalPath,
				Status:       status,
				BlockUUID:    patch.TargetBlockUUID,
				Reason:       reason,
				Position:     patch.Index, // 0-indexed position in the message
			}
			result.Files = append(result.Files, entry)

			// Write Source File if mapped
			var sourceCode string
			if isMapped {
				sourcePath := filepath.Join(workdir, relPath)
				sourceBytes, err := os.ReadFile(sourcePath)
				if err != nil {
					logger.Warning("Failed to read source file for patch", "path", relPath, "error", err)
					continue
				}
				sourceCode = string(sourceBytes)
				if err := writer.WriteSourceFile(workspaceDir, relPath, sourceCode); err != nil {
					return nil, err
				}
			} else {
				// If unmapped, we can't patch it against a source file easily.
				// We just store the patch in unmapped/patches.
				if err := writer.WritePatch(workspaceDir, patch, trim); err != nil {
					return nil, err
				}
				continue
			}

			// Apply Patch
			patchedCode, err := ApplyPatch(sourceCode, patch.ExecutableCode)
			if err != nil {
				if debugPatch {
					var pErr *PatchError
					phase1Diff := patch.ExecutableCode
					phase2Diff := ""
					if errors.As(err, &pErr) {
						phase1Diff = pErr.Phase1Diff
						phase2Diff = pErr.Phase2Diff
					}
					
					// We need the chat for this message. 
					// In mapped dump, we iterate over messages, but we don't have the chat object directly in the loop scope easily.
					// We can look it up or pass it down. 
					// Since we have the list of chats, we can find it.
					var associatedChat db.Chat
					for _, c := range chats {
						if c.ID == msg.ChatID {
							associatedChat = c
							break
						}
					}

					debugPath, writeErr := WriteDebugArtifacts(msg, associatedChat, sourceCode, phase1Diff, phase2Diff, patch.TargetBlockUUID, err)
					if writeErr != nil {
						logger.Error("Failed to write debug artifacts", "error", writeErr)
					} else {
						logger.Warning("Failed to apply patch", "target_uuid", patch.TargetBlockUUID, "error", err, "debug_dir", debugPath)
					}
				}
				// If patch fails, we can't write the proposed file.
				// We write the raw patch to unmapped/patches for manual review.
				if err := writer.WritePatch(workspaceDir, patch, trim); err != nil {
					return nil, err
				}
				continue
			}

			// Write Patched File (Proposed)
			header := generateCodeHeaderFromPatch(patch)
			if err := writer.WritePatchedFile(workspaceDir, patch, header, patchedCode); err != nil {
				return nil, err
			}

			// Log Full Code (Patched)
			logger.Debug("Writing Patched Code", "uuid", patch.TargetBlockUUID, "language", patch.Language, "code_length", len(patchedCode))
			logger.Debug("Code Content", "uuid", patch.TargetBlockUUID, "content", patchedCode)

			// Write Provenance
			prov := Provenance{
				FilePath:     relPath,
				BlockUUID:    patch.TargetBlockUUID,
				ParentUUID:   patch.SourceBlockUUID,
				Version:      patch.TargetVersion,
				ChatID:       msg.ChatID,
				MessageID:    msg.ID,
				ContractUUID: contractUUID,
				Model:        msg.RealModel.String,
				Timestamp:    msg.CreatedAt.Format(time.RFC3339),
				Action:       "patch_applied",
				Authors:      patch.Authors,
			}
			if err := writer.WriteProvenanceJSON(workspaceDir, prov); err != nil {
				logger.Warning("Failed to write provenance", "block_uuid", patch.TargetBlockUUID, "error", err)
			}

			// Add to checklist
			checkItem := fmt.Sprintf("[x] %s (%s)", entry.Path, status)
			msgChecklist = append(msgChecklist, checkItem)
			processedArtifacts++
		}

		// ==========================================
		// MESSAGE CHECKLIST
		// ==========================================
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("Message ID: %d Checklist\n", msg.ID)
		fmt.Println("--------------------------------------------------")
		for _, item := range msgChecklist {
			fmt.Println(item)
		}
		fmt.Printf("Files Processed: %d | Files Remaining: %d\n", processedArtifacts, totalArtifacts-processedArtifacts)
		fmt.Println("--------------------------------------------------\n")
	}

	// Calculate Stats
	for _, f := range result.Files {
		if f.Status == MappedStatusMapped {
			result.Stats.Mappable++
		} else {
			result.Stats.Unmappable++
		}
	}

	logger.Info("Mapped Dump Complete", "hash", dumpHash, "mappable", result.Stats.Mappable, "unmappable", result.Stats.Unmappable)

	// ==========================================
	// WRITE WORKSPACE MANIFEST
	// ==========================================
	manifest := ShadowWorkspace{
		Hash:         dumpHash,
		MessageID:    messageID, // 0 if full contract
		ContractUUID: contractUUID,
		CreatedAt:    time.Now().Format(time.RFC3339),
		ExpiresAt:    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		Stats:        result.Stats, // Persist stats
		Files:        result.Files,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workspace manifest: %w", err)
	}

	manifestPath := filepath.Join(workspaceDir, "workspace.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write workspace manifest: %w", err)
	}

	logger.Info("Workspace manifest written", "path", manifestPath)

	// ==========================================
	// GENERATE HELP FILES
	// ==========================================
	if err := generateHelpFiles(workspaceDir, contractUUID, dumpHash, activeChatID, workdir, sqliteDB); err != nil {
		logger.Warning("Failed to generate help files", "error", err)
		// Non-fatal error, continue
	}

	return result, nil
}

// sanitizeComponentName sanitizes a component name for use in file paths.
// It replaces spaces with underscores, removes special characters, and lowercases the result.
func sanitizeComponentName(name string) string {
	// Replace spaces with underscores
	sanitized := strings.ReplaceAll(name, " ", "_")
	
	// Remove special characters (keep only: a-z, A-Z, 0-9, -, _)
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized = reg.ReplaceAllString(sanitized, "")
	
	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)
	
	// Ensure non-empty result
	if sanitized == "" {
		sanitized = "component"
	}
	
	return sanitized
}

// formatArtifactPath formats the path for an artifact based on the dump mode and position.
// Single Message Mode: "01_sanitized_name"
// Full Contract Mode: "1_01_sanitized_name" (msgIndex_position_name)
func formatArtifactPath(isSingleMessage bool, msgIndex int, position int, name string) string {
	sanitized := sanitizeComponentName(name)
	
	if isSingleMessage {
		return fmt.Sprintf("%02d_%s", position+1, sanitized)
	}
	
	return fmt.Sprintf("%d_%02d_%s", msgIndex, position+1, sanitized)
}

// generateHelpFiles creates .gsc-welcome and .gsc-help in the workspace root.
func generateHelpFiles(workspaceDir, contractUUID, dumpHash string, activeChatID int64, workdir string, db *sql.DB) error {
	// 1. Fetch Chat Name
	chatName := "Unknown Chat"
	if activeChatID > 0 {
		err := db.QueryRow("SELECT name FROM chats WHERE id = ?", activeChatID).Scan(&chatName)
		if err != nil {
			logger.Debug("Failed to fetch chat name for help files", "active_chat_id", activeChatID, "error", err)
			chatName = "Unknown Chat"
		}
	}

	// 2. Load Templates
	gscHome, _ := settings.GetGSCHome(false)
	templateDir := filepath.Join(gscHome, "data", "templates", "help")

	welcomePath := filepath.Join(templateDir, "welcome.txt")
	helpPath := filepath.Join(templateDir, "help.txt")

	welcomeContent, err := os.ReadFile(welcomePath)
	if err != nil {
		return fmt.Errorf("failed to read welcome template: %w", err)
	}

	helpContent, err := os.ReadFile(helpPath)
	if err != nil {
		return fmt.Errorf("failed to read help template: %w", err)
	}

	// 3. Substitute Variables
	replacements := map[string]string{
		"{{MSG_HASH}}":        dumpHash,
		"{{CHAT_NAME}}":       chatName,
		"{{GSC_CHAT_ID}}":     fmt.Sprintf("%d", activeChatID),
		"{{GSC_PROJECT_ROOT}}": workdir,
		"{{GSC_CONTRACT_UUID}}": contractUUID,
	}

	processedWelcome := string(welcomeContent)
	processedHelp := string(helpContent)

	for key, val := range replacements {
		processedWelcome = strings.ReplaceAll(processedWelcome, key, val)
		processedHelp = strings.ReplaceAll(processedHelp, key, val)
	}

	// 4. Write Files to Parent Directory (mapped)
	// workspaceDir is <uuid>/mapped/<hash>, so parent is <uuid>/mapped
	mappedDir := filepath.Dir(workspaceDir)

	if err := os.WriteFile(filepath.Join(mappedDir, ".gsc-welcome"), []byte(processedWelcome), 0644); err != nil {
		return fmt.Errorf("failed to write .gsc-welcome: %w", err)
	}

	if err := os.WriteFile(filepath.Join(mappedDir, ".gsc-help"), []byte(processedHelp), 0644); err != nil {
		return fmt.Errorf("failed to write .gsc-help: %w", err)
	}

	logger.Info("Help files generated", "workspace", workspaceDir, "mapped_dir", mappedDir)
	return nil
}

// executeMergedDump implements the logic for the 'merged' dump type.
func executeMergedDump(chats []db.Chat, sqliteDB *sql.DB, writer DumpWriter, outputDir string, includeSystem bool, trim bool, sortMode string, blockMap map[string]string, debugPatch bool) error {

	// ==========================================
	// PASS 0: Build Merged Tree & Calculate Metrics
	// ==========================================
	// Map: Hash -> MergedNode
	nodeMap := make(map[string]*MergedNode)
	// Map: ChatID -> MessageID -> Message (for parent lookup)
	chatMessages := make(map[int64]map[int64]db.Message)

	// 1. Fetch and Index all messages
	for _, chat := range chats {
		messages, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch messages for chat %d: %w", chat.ID, err)
		}

		chatMessages[chat.ID] = make(map[int64]db.Message)
		for _, msg := range messages {
			chatMessages[chat.ID][msg.ID] = msg

			hash := calculateMessageHash(msg)
			if node, exists := nodeMap[hash]; exists {
				// Node already exists (duplicate message), just add chat
				node.Chats = append(node.Chats, chat)
				node.ChatCount++
			} else {
				// Create new node
				nodeMap[hash] = &MergedNode{
					Message:        msg,
					Hash:           hash,
					Chats:          []db.Chat{chat},
					ChatCount:      1,
					MaxSubtreeTime: msg.UpdatedAt,
				}
			}
		}
	}

	// 2. Build Tree Structure (Link Children to Parents)
	// We iterate through all nodes and find their parent in the global map
	var roots []*MergedNode
	for _, node := range nodeMap {
		// If parent_id is 0, it's a root (System message)
		if node.Message.ParentID == 0 {
			roots = append(roots, node)
			continue
		}

		// Find parent message in the same chat
		parentMsg, ok := chatMessages[node.Message.ChatID][node.Message.ParentID]
		if !ok {
			logger.Warning("Parent message not found in chat", "msg_id", node.Message.ID, "parent_id", node.Message.ParentID)
			continue
		}

		// Find parent node in global map using parent's hash
		parentHash := calculateMessageHash(parentMsg)
		parentNode, ok := nodeMap[parentHash]
		if !ok {
			logger.Warning("Parent node not found in global map", "msg_id", node.Message.ID, "parent_hash", parentHash)
			continue
		}

		// Link
		parentNode.Children = append(parentNode.Children, node)
	}

	// 3. Calculate Metrics (MaxSubtreeTime) recursively
	calculateMetrics(roots)

	// ==========================================
	// PASS 1: Indexing (Global)
	// ==========================================
	// We traverse the merged tree to populate the blockMap
	var traverseForIndexing func(nodes []*MergedNode)
	traverseForIndexing = func(nodes []*MergedNode) {
		for _, node := range nodes {
			if !node.Message.Message.Valid {
				continue
			}

			result, err := markdown.ExtractCodeBlocks(node.Message.Message.String, trim)
			if err != nil {
				continue
			}

			for _, block := range result.Blocks {
				if block.BlockUUID != "" {
					blockMap[block.BlockUUID] = block.ExecutableCode
				}
			}

			traverseForIndexing(node.Children)
		}
	}
	traverseForIndexing(roots)

	// ==========================================
	// PASS 2: Generation (Sorted)
	// ==========================================
	globalRank := 0
	var traverseForGeneration func(nodes []*MergedNode, rank int)
	traverseForGeneration = func(nodes []*MergedNode, rank int) {
		// Sort children based on sortMode
		sort.Slice(nodes, func(i, j int) bool {
			switch sortMode {
			case settings.SortRecency:
				return nodes[i].MaxSubtreeTime.After(nodes[j].MaxSubtreeTime)
			case settings.SortPopularity:
				return nodes[i].ChatCount > nodes[j].ChatCount
			case settings.SortChronological:
				return nodes[i].Message.CreatedAt.Before(nodes[j].Message.CreatedAt)
			default:
				return nodes[i].Message.CreatedAt.Before(nodes[j].Message.CreatedAt)
			}
		})

		for _, node := range nodes {
			if !node.Message.Message.Valid {
				continue
			}

			if !includeSystem && node.Message.Role == "system" {
				traverseForGeneration(node.Children, 1) // Continue recursion but don't write
				continue
			}

			// Inject metrics into MergedWriter if applicable
			if mw, ok := writer.(*MergedWriter); ok {
				globalRank++
				mw.SetMetrics(globalRank, node.ChatCount)
			}

			// Use a dummy chat for GetMessageDir since MergedWriter ignores it
			dummyChat := db.Chat{ID: 0, Name: "merged"}
			relMsgDir := writer.GetMessageDir(dummyChat, node.Message, 0)
			absMsgDir := filepath.Join(outputDir, relMsgDir)

			if err := os.MkdirAll(absMsgDir, 0755); err != nil {
				logger.Error("Failed to create directory", "path", absMsgDir, "error", err)
				continue
			}

			if err := writer.WriteMessage(absMsgDir, node.Message); err != nil {
				logger.Error("Failed to write message", "error", err)
			}

			if err := writer.WriteProvenance(absMsgDir, node.Chats); err != nil {
				logger.Error("Failed to write provenance", "error", err)
			}

			result, err := markdown.ExtractCodeBlocks(node.Message.Message.String, trim)
			if err != nil {
				logger.Warning("Failed to parse markdown for message", "id", node.Message.ID, "error", err)
				traverseForGeneration(node.Children, 1)
				continue
			}

			// Write standard code blocks
			for _, block := range result.Blocks {
				if err := writer.WriteBlock(absMsgDir, block, trim); err != nil {
					logger.Error("Failed to write block", "error", err)
				}
			}

			// Process patches and generate "patched" files
			for _, patch := range result.Patches {
				if err := writer.WritePatch(absMsgDir, patch, trim); err != nil {
					logger.Error("Failed to write patch", "error", err)
				}

				if patch.SourceBlockUUID == "" {
					continue
				}

				sourceCode, ok := blockMap[patch.SourceBlockUUID]
				if !ok {
					logger.Warning("Source block not found for patch", "source_uuid", patch.SourceBlockUUID, "msg_id", node.Message.ID)
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

						// Use the first chat associated with this node for context
						primaryChat := db.Chat{}
						if len(node.Chats) > 0 {
							primaryChat = node.Chats[0]
						}

						debugPath, writeErr := WriteDebugArtifacts(node.Message, primaryChat, sourceCode, phase1Diff, phase2Diff, patch.TargetBlockUUID, err)
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
					logger.Error("Failed to write patched file", "error", err)
				}

				if patch.TargetBlockUUID != "" {
					blockMap[patch.TargetBlockUUID] = patchedCode
				}
			}

			// Recurse into children
			traverseForGeneration(node.Children, 1)
		}
	}

	traverseForGeneration(roots, 1)
	return nil
}

// calculateMetrics recursively calculates MaxSubtreeTime for all nodes.
func calculateMetrics(nodes []*MergedNode) {
	for _, node := range nodes {
		if len(node.Children) == 0 {
			node.MaxSubtreeTime = node.Message.UpdatedAt
			continue
		}

		calculateMetrics(node.Children)
		maxTime := node.Message.UpdatedAt
		for _, child := range node.Children {
			if child.MaxSubtreeTime.After(maxTime) {
				maxTime = child.MaxSubtreeTime
			}
		}
		node.MaxSubtreeTime = maxTime
	}
}

// calculateMessageHash generates a deterministic hash for a message.
func calculateMessageHash(msg db.Message) string {
	h := sha256.New()
	h.Write([]byte(msg.Role))
	if msg.Message.Valid {
		h.Write([]byte(msg.Message.String))
	}
	h.Write([]byte(msg.CreatedAt.Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:8]
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
