/*
 * Component: Tree Dump Writer
 * Block-UUID: 6d74a81f-87cb-4226-983d-0e3a49daf85f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the DumpWriter interface for the 'tree' strategy. This creates a conversational filesystem where messages are organized by chat, position, and timestamp, preserving full code context including traceability headers.
 * Language: Go
 * Created-at: 2026-03-03T02:10:15.220Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// TreeWriter implements the DumpWriter interface for the conversational tree strategy.
type TreeWriter struct{}

// Prepare wipes the output directory to ensure a clean state for the new dump.
func (w *TreeWriter) Prepare(outputDir string) error {
	// Remove existing directory if it exists
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}
	// Create fresh directory
	return os.MkdirAll(outputDir, 0755)
}

// GetMessageDir generates the directory path for a specific message.
// Format: chat_<id>_<sanitized_name>/<pos>_<timestamp>_<role>/
func (w *TreeWriter) GetMessageDir(chat db.Chat, msg db.Message, position int) string {
	safeName := sanitizeName(chat.Name)
	timestamp := msg.CreatedAt.Format("2006-01-02T15-04-05")
	return fmt.Sprintf("chat_%d_%s/%02d_%s_%s", chat.ID, safeName, position, timestamp, msg.Role)
}

// WriteMessage persists the raw markdown content of the message.
func (w *TreeWriter) WriteMessage(msgDir string, msg db.Message) error {
	path := filepath.Join(msgDir, "message.md")
	return os.WriteFile(path, []byte(msg.Message.String), 0644)
}

// WriteBlock persists a code block, including its traceability header.
func (w *TreeWriter) WriteBlock(msgDir string, block markdown.CodeBlock) error {
	// Determine filename: block_<idx>_<uuid>.ext or block_<idx>.ext
	filename := fmt.Sprintf("block_%d", block.Index)
	if block.BlockUUID != "" {
		filename += "_" + block.BlockUUID
	}
	filename += getExtension(block.Language)

	// Combine header and code for full context
	content := block.RawHeader + "\n\n\n" + block.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// WritePatch persists a patch block, including its metadata header.
func (w *TreeWriter) WritePatch(msgDir string, patch markdown.PatchBlock) error {
	// Determine filename: patch_<idx>_<uuid>.diff or patch_<idx>.diff
	filename := fmt.Sprintf("patch_%d", patch.Index)
	if patch.TargetBlockUUID != "" {
		filename += "_" + patch.TargetBlockUUID
	}
	filename += ".diff"

	// Combine header and diff content
	content := patch.RawHeader + "\n\n\n" + patch.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// sanitizeName replaces unsafe characters in chat names with underscores.
func sanitizeName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return reg.ReplaceAllString(name, "_")
}

// getExtension maps a language string to a file extension.
func getExtension(lang string) string {
	switch strings.ToLower(lang) {
	case "go":
		return ".go"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "python", "py":
		return ".py"
	case "rust", "rs":
		return ".rs"
	case "markdown", "md":
		return ".md"
	case "diff":
		return ".diff"
	case "sql":
		return ".sql"
	case "bash", "sh":
		return ".sh"
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yml"
	default:
		return ".txt"
	}
}
