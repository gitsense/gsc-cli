/**
 * Component: Mapped Dump Writer
 * Block-UUID: 3af4de63-3a87-4792-8420-31279f031e43
 * Parent-UUID: 59a3aeaa-661d-4eb9-982a-9a6f9d5e1ecc
 * Version: 1.1.0
 * Description: Updated WriteMessage to generate a message.json sidecar file containing database identifiers (id, chat_id, uuid, role, parent_id, created_at) to improve traceability and eliminate the need to parse directory names.
 * Language: Go
 * Created-at: 2026-03-04T04:33:14.119Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// MappedWriter implements the DumpWriter interface for the project-mapped strategy.
type MappedWriter struct {
	// PathMap maps Block-UUIDs to their resolved relative paths in the project.
	// This is populated by the orchestrator (dumper.go) during the discovery pass.
	PathMap map[string]string
}

// SetPathMap allows the orchestrator to inject the UUID-to-Path mappings.
func (w *MappedWriter) SetPathMap(pathMap map[string]string) {
	w.PathMap = pathMap
}

// Prepare wipes the output directory to ensure a clean state for the new dump.
func (w *MappedWriter) Prepare(outputDir string) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}
	return os.MkdirAll(outputDir, 0755)
}

// GetMessageDir returns the root of the dump directory.
// For the mapped type, we don't use a hierarchical message structure; 
// everything is organized by file path relative to the dump root.
func (w *MappedWriter) GetMessageDir(chat db.Chat, msg db.Message, position int) string {
	return "."
}

// WriteMessage persists the raw markdown content of the message.
// It now also creates a message.json sidecar for database traceability.
func (w *MappedWriter) WriteMessage(msgDir string, msg db.Message) error {
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

// WriteProvenance is a no-op for MappedWriter.
func (w *MappedWriter) WriteProvenance(msgDir string, chats []db.Chat) error {
	return nil
}

// WriteBlock handles the persistence of a code block.
// If the block is mapped, it writes to mapped/<path>/proposed.<ext>.
// If unmapped, it writes to unmapped/snippets/ or unmapped/components/.
func (w *MappedWriter) WriteBlock(msgDir string, block markdown.CodeBlock, trim bool) error {
	relPath, isMapped := w.PathMap[block.BlockUUID]
	
	var targetDir string
	var filename string

	if isMapped {
		targetDir = filepath.Join(msgDir, "mapped", relPath)
		filename = "proposed" + getExtension(block.Language)
	} else {
		if block.Component != "" {
			targetDir = filepath.Join(msgDir, "unmapped", "components", block.Component)
			filename = "proposed" + getExtension(block.Language)
		} else {
			targetDir = filepath.Join(msgDir, "unmapped", "snippets")
			filename = fmt.Sprintf("block_%03d%s", block.Index+1, getExtension(block.Language))
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	content := block.RawHeader + "\n\n\n" + block.ExecutableCode
	return os.WriteFile(filepath.Join(targetDir, filename), []byte(content), 0644)
}

// WritePatch handles the persistence of a patch block.
// For the mapped dump, we only store the patch if it's unmapped or as a reference.
func (w *MappedWriter) WritePatch(msgDir string, patch markdown.PatchBlock, trim bool) error {
	// We don't map patches directly to the project tree; we map the resulting patched file.
	// However, we store the patch in the unmapped/patches directory for reference.
	targetDir := filepath.Join(msgDir, "unmapped", "patches")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("patch_%03d", patch.Index+1)
	if patch.TargetBlockUUID != "" {
		filename += "_" + patch.TargetBlockUUID[:8]
	}
	filename += ".diff"

	content := patch.RawHeader + "\n\n\n" + patch.ExecutableCode
	return os.WriteFile(filepath.Join(targetDir, filename), []byte(content), 0644)
}

// WritePatchedFile persists the result of a successful patch application.
// This becomes the 'proposed.<ext>' file in the mapped directory.
func (w *MappedWriter) WritePatchedFile(msgDir string, patch markdown.PatchBlock, header string, content string) error {
	relPath, isMapped := w.PathMap[patch.TargetBlockUUID]
	if !isMapped {
		// If the target isn't mapped, we treat it as an unmapped component
		targetDir := filepath.Join(msgDir, "unmapped", "components", patch.Component)
		if patch.Component == "" {
			targetDir = filepath.Join(msgDir, "unmapped", "orphans")
		}
		os.MkdirAll(targetDir, 0755)
		
		filename := fmt.Sprintf("proposed_%03d%s", patch.Index+1, getExtension(patch.Language))
		fullContent := header + "\n\n\n" + content
		return os.WriteFile(filepath.Join(targetDir, filename), []byte(fullContent), 0644)
	}

	targetDir := filepath.Join(msgDir, "mapped", relPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	filename := "proposed" + getExtension(patch.Language)
	fullContent := header + "\n\n\n" + content
	return os.WriteFile(filepath.Join(targetDir, filename), []byte(fullContent), 0644)
}

// WriteSourceFile copies the original source file content to the dump directory.
func (w *MappedWriter) WriteSourceFile(msgDir string, relPath string, content string) error {
	targetDir := filepath.Join(msgDir, "mapped", relPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	filename := "source" + filepath.Ext(relPath)
	return os.WriteFile(filepath.Join(targetDir, filename), []byte(content), 0644)
}

// WriteProvenanceJSON persists the structured provenance data for a specific file.
func (w *MappedWriter) WriteProvenanceJSON(msgDir string, data Provenance) error {
	relPath, isMapped := w.PathMap[data.BlockUUID]
	
	var targetDir string
	if isMapped {
		targetDir = filepath.Join(msgDir, "mapped", relPath)
	} else {
		// For unmapped files, we place provenance in the same directory as the proposed file
		if data.FilePath != "" {
			targetDir = filepath.Join(msgDir, "unmapped", "components", data.FilePath)
		} else {
			// Snippets don't get individual provenance.json files in this implementation
			return nil
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(targetDir, "provenance.json"), jsonData, 0644)
}
