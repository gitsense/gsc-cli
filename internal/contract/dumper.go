/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: ea3f0cb9-4d2f-47c8-bb43-2c43d07438b4
 * Parent-UUID: 99085959-89e1-4484-a810-1fbbd916be1f
 * Version: 2.3.0
 * Description: Refactored ExecuteDump to support the 'merged' dump type. Added Pass 0 to build a global MergedNode tree, calculate metrics (ChatCount, MaxSubtreeTimestamp), and handle sorting strategies (recency, popularity, chronological). The orchestrator now traverses the merged tree instead of individual chats for the 'merged' type.
 * Language: Go
 * Created-at: 2026-03-03T19:22:00.669Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0)
 */


package contract

import (
	"errors"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
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
// It supports both 'tree' and 'merged' strategies.
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool, trim bool, dumpType string, sortMode string, debugPatch bool) error {
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
	// STRATEGY SELECTION
	// ==========================================
	if dumpType == "merged" {
		return executeMergedDump(chats, sqliteDB, writer, outputDir, includeSystem, trim, sortMode, blockMap, debugPatch)
	}

	// ==========================================
	// LEGACY 'TREE' STRATEGY
	// ==========================================
	// PASS 1: Indexing
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

						debugPath, writeErr := WriteDebugArtifacts(sourceCode, phase1Diff, phase2Diff, patch.TargetBlockUUID, err)
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

		for i, node := range nodes {
			if !node.Message.Message.Valid {
				continue
			}

			if !includeSystem && node.Message.Role == "system" {
				traverseForGeneration(node.Children, 1) // Continue recursion but don't write
				continue
			}

			// Inject metrics into MergedWriter if applicable
			if mw, ok := writer.(*MergedWriter); ok {
				mw.SetMetrics(i+1, node.ChatCount)
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

						debugPath, writeErr := WriteDebugArtifacts(sourceCode, phase1Diff, phase2Diff, patch.TargetBlockUUID, err)
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
