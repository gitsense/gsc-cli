/**
 * Component: Merged Dump Writer
 * Block-UUID: e0f26185-7850-4e19-9aed-a143824743a1
 * Parent-UUID: 3b8154e0-0fc2-4c7e-8f0f-f2949303192c
 * Version: 1.0.3
 * Description: Implements the DumpWriter interface for the 'merged' strategy. It generates a squashed filesystem tree where duplicate messages are unified. Directory names follow the <rank>_<count>_<role>_<hash> convention, and provenance files (<n>_chats.md) are generated for each node.
 * Language: Go
 * Created-at: 2026-03-04T01:41:01.763Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), Gemini 3 Flash (v1.0.3)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
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
	
	// Format: 001_003_syst_a1b2c3d4
	return fmt.Sprintf("%03d_%03d_%s_%s", w.rank, w.count, role, hash)
}

// WriteMessage persists the raw markdown content of the message.
func (w *MergedWriter) WriteMessage(msgDir string, msg db.Message) error {
	path := filepath.Join(msgDir, "message.md")
	return os.WriteFile(path, []byte(msg.Message.String), 0644)
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

// pluralize returns "s" if n is not 1, otherwise an empty string.
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
