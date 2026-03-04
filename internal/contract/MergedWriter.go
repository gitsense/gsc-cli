/**
 * Component: Merged Dump Writer
 * Block-UUID: 91b51237-db1d-463c-a9e1-6ab5094a95e0
 * Parent-UUID: 3709dcd6-eba6-4b6a-bd57-c7604cb3bf77
 * Version: 1.0.8
 * Description: Updated WriteMessage to generate a message.json sidecar file containing database identifiers (id, chat_id, uuid, role, parent_id, created_at) to improve traceability and eliminate the need to parse directory names.
 * Language: Go
 * Created-at: 2026-03-04T02:19:44.766Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), Gemini 3 Flash (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), Gemini 3 Flash (v1.0.7), GLM-4.7 (v1.0.8)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// MergedWriter implements the DumpWriter interface for the merged/squashed strategy.
type MergedWriter struct {
	rank  int
	count int
}

// SetMetrics allows the orchestrator (dumper.go) to inject the calculated rank and count
// before calling GetMessageDir. This avoids changing the DumpWriter interface signature.
func (w *MergedWriter) SetMetrics(rank int, count int) {
	w.rank = rank
	w.count = count
}

// Prepare wipes the output directory to ensure a clean state for the new dump.
func (w *MergedWriter) Prepare(outputDir string) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}
	return os.MkdirAll(outputDir, 0755)
}

// GetMessageDir generates the directory path for a specific message.
// Format: <rank>_<count>_<role>_<hash>/
// The hash is calculated deterministically from the message content and timestamp.
func (w *MergedWriter) GetMessageDir(chat db.Chat, msg db.Message, position int) string {
	role := abbreviateRole(msg.Role)
	hash := calculateMessageHash(msg)
	
	// Debug logging to diagnose zero-time issues
	logger.Debug("MergedWriter: GetMessageDir", "msg_id", msg.ID, "updated_at", msg.UpdatedAt, "created_at", msg.CreatedAt)

	// Use UpdatedAt, fallback to CreatedAt if zero
	timestamp := msg.UpdatedAt
	if timestamp.IsZero() {
		logger.Debug("MergedWriter: UpdatedAt is zero, falling back to CreatedAt", "msg_id", msg.ID)
		timestamp = msg.CreatedAt
	}
	
	if timestamp.IsZero() {
		logger.Warning("MergedWriter: Both UpdatedAt and CreatedAt are zero", "msg_id", msg.ID)
	}
	
	// Format: YYYY-MM-DD/HH/rank_count_role_hash
	dateStr := timestamp.Format("2006-01-02")
	hourStr := timestamp.Format("15")
	path := fmt.Sprintf("%s/%s/%03d_%03d_%s_%s", dateStr, hourStr, w.rank, w.count, role, hash)
	
	logger.Debug("MergedWriter: Generated path", "msg_id", msg.ID, "path", path)
	
	return path
}

// WriteMessage persists the raw markdown content of the message.
// It now also creates a message.json sidecar for database traceability.
func (w *MergedWriter) WriteMessage(msgDir string, msg db.Message) error {
	// 1. Write message.md
	path := filepath.Join(msgDir, "message.md")
	if err := os.WriteFile(path, []byte(msg.Message.String), 0644); err != nil {
		return err
	}

	// 2. Write message.json
	metadata := map[string]interface{}{
		"id":         msg.ID,
		"chat_id":    msg.ChatID,
		"type":       msg.Type,
		"role":       msg.Role,
		"parent_id":  msg.ParentID,
		"created_at": msg.CreatedAt,
	}

	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal message metadata: %w", err)
	}

	jsonPath := filepath.Join(msgDir, "message.json")
	return os.WriteFile(jsonPath, jsonData, 0644)
}

// WriteProvenance persists the chat metadata (provenance) for a message.
// It creates a chats.md file listing all chats that contain this message.
func (w *MergedWriter) WriteProvenance(msgDir string, chats []db.Chat) error {
	if len(chats) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Message Provenance\n\n")
	sb.WriteString(fmt.Sprintf("This message exists in **%d** chat%s.\n\n", len(chats), pluralize(len(chats))))
	
	sb.WriteString("| Chat ID | Chat UUID | Chat Name |\n")
	sb.WriteString("| :--- | :--- | :--- |\n")

	for _, c := range chats {
		sb.WriteString(fmt.Sprintf("| %d | `%s` | %s |\n", c.ID, c.UUID, c.Name))
	}

	path := filepath.Join(msgDir, "chats.md")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// WriteBlock persists a code block.
func (w *MergedWriter) WriteBlock(msgDir string, block markdown.CodeBlock, trim bool) error {
	filename := fmt.Sprintf("%03d_block", block.Index+1)
	if block.BlockUUID != "" {
		filename += "_" + block.BlockUUID[:8]
	}
	filename += getExtension(block.Language)

	content := block.RawHeader + "\n\n\n" + block.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// WritePatch persists a patch block.
func (w *MergedWriter) WritePatch(msgDir string, patch markdown.PatchBlock, trim bool) error {
	filename := fmt.Sprintf("%03d_patch", patch.Index+1)
	if patch.TargetBlockUUID != "" {
		filename += "_" + patch.TargetBlockUUID[:8]
	}
	filename += ".diff"

	content := patch.RawHeader + "\n\n\n" + patch.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// WritePatchedFile persists the result of a successful patch application.
func (w *MergedWriter) WritePatchedFile(msgDir string, patch markdown.PatchBlock, header string, content string) error {
	filename := fmt.Sprintf("%03d_patched", patch.Index+1)
	if patch.TargetBlockUUID != "" {
		filename += "_" + patch.TargetBlockUUID[:8]
	}
	filename += getExtension(patch.Language)

	fullContent := header + "\n\n\n" + content
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(fullContent), 0644)
}

// WriteSourceFile is a no-op for the MergedWriter as it does not support the mapped shadow workspace.
func (w *MergedWriter) WriteSourceFile(msgDir string, relPath string, content string) error {
	return nil
}

// WriteProvenanceJSON is a no-op for the MergedWriter as it does not support the mapped shadow workspace.
func (w *MergedWriter) WriteProvenanceJSON(msgDir string, data Provenance) error {
	return nil
}

// pluralize returns "s" if n is not 1, otherwise an empty string.
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
