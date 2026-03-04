/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: 545075c7-3a57-478f-990f-af1f838ff3f1
 * Parent-UUID: 4da67bdb-9b19-4c61-b7dc-dff212374b32
 * Version: 2.6.1
 * Description: Removed unused variable 'targetMessage' in executeMappedDump to resolve build error.
 * Language: Go
 * Created-at: 2026-03-04T01:34:28.487Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v2.4.1), Gemini 3 Flash (v2.4.2), Gemini 3 Flash (v2.5.0), GLM-4.7 (v2.6.0), GLM-4.7 (v2.6.1)
 */


package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"database/sql"
	"os"
	"path/filepath"
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
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool, trim bool, dumpType string, sortMode string, debugPatch bool, messageID int64) (*MappedDumpResult, error) {
	// 1. Initialize Output
	if err := writer.Prepare(outputDir); err != nil {
		return nil, fmt.Errorf("failed to prepare dump directory: %w", err)
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
		return executeMappedDump(chats, sqliteDB, writer, outputDir, includeSystem, trim, debugPatch, messageID)
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
func executeMappedDump(chats []db.Chat, sqliteDB *sql.DB, writer DumpWriter, outputDir string, includeSystem bool, trim bool, debugPatch bool, messageID int64) (*MappedDumpResult, error) {
	// 1. Fetch Messages
	var messages []db.Message
	var dumpHash string

	if messageID > 0 {
		// Single Message Mode
		msg, err := db.GetMessage(sqliteDB, messageID)
		if err != nil {
			return nil, fmt.Errorf("message ID %d not found: %w", messageID, err)
		}
		messages = []db.Message{*msg}
		dumpHash = calculateMessageHash(*msg)
	} else {
		// Full Contract Mode (Fetch all messages from all chats)
		for _, chat := range chats {
			chatMsgs, err := db.GetMessagesRecursive(sqliteDB, chat.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch messages for chat %d: %w", chat.ID, err)
			}
			messages = append(messages, chatMsgs...)
		}
		// For full contract, use contract UUID as hash (simplified)
		dumpHash = "full-contract"
	}

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
	}

	// Convert map to slice
	var uuidList []string
	for uuid := range parentUUIDs {
		uuidList = append(uuidList, uuid)
	}

	// Resolve paths using ripgrep
	// We need the workdir. We can get it from the first chat's context or assume CWD.
	// For now, assume CWD is the workdir (standard for CLI usage).
	workdir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	engine := &search.RipgrepEngine{}
	pathMap, err := engine.ResolvePathsByUUIDs(context.Background(), workdir, uuidList)
	if err != nil {
		logger.Warning("Batch discovery failed", "error", err)
		// Continue with empty map (everything will be unmapped)
		pathMap = make(map[string]string)
	}

	// Inject PathMap into MappedWriter
	if mw, ok := writer.(*MappedWriter); ok {
		mw.SetPathMap(pathMap)
	}

	// 3. Generation Pass
	result := &MappedDumpResult{
		Success: true,
		Hash:    dumpHash,
		RootDir: outputDir,
		Files:   []MappedFileEntry{},
	}

	// blockMap stores Block-UUID -> ExecutableCode content for patching
	blockMap := make(map[string]string)

	for _, msg := range messages {
		if !msg.Message.Valid {
			continue
		}

		if !includeSystem && msg.Role == "system" {
			continue
		}

		// Parse message
		parseResult, err := markdown.ExtractCodeBlocks(msg.Message.String, trim)
		if err != nil {
			logger.Warning("Failed to parse markdown", "msg_id", msg.ID, "error", err)
			continue
		}

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
			} else {
				if block.ParentUUID == "" || block.ParentUUID == "N/A" {
					reason = "no_parent_uuid"
				} else {
					reason = "parent_not_found"
				}
			}

			// Add to result list
			entry := MappedFileEntry{
				Path:      relPath, // Will be component name if unmapped
				Status:    status,
				BlockUUID: block.BlockUUID,
				Reason:    reason,
			}
			if !isMapped && block.Component != "" {
				entry.Path = block.Component
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
				if err := writer.WriteSourceFile(outputDir, relPath, string(sourceContent)); err != nil {
					return nil, err
				}
			}

			// Write Proposed File
			if err := writer.WriteBlock(outputDir, block, trim); err != nil {
				return nil, err
			}

			// Write Provenance
			prov := Provenance{
				FilePath:     relPath,
				BlockUUID:    block.BlockUUID,
				ParentUUID:   block.ParentUUID,
				Version:      block.Version,
				ChatID:       msg.ChatID,
				MessageID:    msg.ID,
				ContractUUID: "", // TODO: Pass contract UUID if available
				Model:        msg.RealModel.String,
				Timestamp:    msg.CreatedAt.Format(time.RFC3339),
				Action:       "full_code",
				Authors:      block.Authors,
			}
			if err := writer.WriteProvenanceJSON(outputDir, prov); err != nil {
				logger.Warning("Failed to write provenance", "block_uuid", block.BlockUUID, "error", err)
			}
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
			} else {
				reason = "parent_not_found"
			}

			// Add to result list
			entry := MappedFileEntry{
				Path:      relPath,
				Status:    status,
				BlockUUID: patch.TargetBlockUUID,
				Reason:    reason,
			}
			if !isMapped && patch.Component != "" {
				entry.Path = patch.Component
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
				if err := writer.WriteSourceFile(outputDir, relPath, sourceCode); err != nil {
					return nil, err
				}
			} else {
				// If unmapped, we can't patch it against a source file easily.
				// We just store the patch in unmapped/patches.
				if err := writer.WritePatch(outputDir, patch, trim); err != nil {
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
				if err := writer.WritePatch(outputDir, patch, trim); err != nil {
					return nil, err
				}
				continue
			}

			// Write Patched File (Proposed)
			header := generateCodeHeaderFromPatch(patch)
			if err := writer.WritePatchedFile(outputDir, patch, header, patchedCode); err != nil {
				return nil, err
			}

			// Write Provenance
			prov := Provenance{
				FilePath:     relPath,
				BlockUUID:    patch.TargetBlockUUID,
				ParentUUID:   patch.SourceBlockUUID,
				Version:      patch.TargetVersion,
				ChatID:       msg.ChatID,
				MessageID:    msg.ID,
				ContractUUID: "",
				Model:        msg.RealModel.String,
				Timestamp:    msg.CreatedAt.Format(time.RFC3339),
				Action:       "patch_applied",
				Authors:      patch.Authors,
			}
			if err := writer.WriteProvenanceJSON(outputDir, prov); err != nil {
				logger.Warning("Failed to write provenance", "block_uuid", patch.TargetBlockUUID, "error", err)
			}
		}
	}

	// Calculate Stats
	for _, f := range result.Files {
		if f.Status == MappedStatusMapped {
			result.Stats.Mappable++
		} else {
			result.Stats.Unmappable++
		}
	}

	return result, nil
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
