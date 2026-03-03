/**
 * Component: Dump Writer Interface
 * Block-UUID: f1a0998b-3727-455e-9b62-3688434912f2
 * Parent-UUID: f0918a9d-8969-4773-8271-e5b361e953ef
 * Version: 1.3.0
 * Description: Added WritePatchedFile to the DumpWriter interface. This allows the orchestrator to persist the result of a successful patch application during the dump process.
 * Language: Go
 * Created-at: 2026-03-03T07:37:57.416Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), Gemini 3 Flash (v1.3.0)
 */


package contract

import (
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// DumpWriter defines the strategy for organizing and writing chat artifacts to disk.
type DumpWriter interface {
	// Prepare initializes the output directory (e.g., wiping old data).
	Prepare(outputDir string) error

	// GetMessageDir returns the relative path for a specific message's artifacts.
	// This allows different writers to structure the hierarchy differently.
	GetMessageDir(chat db.Chat, msg db.Message, position int) string

	// WriteMessage handles the persistence of the raw message content (message.md).
	WriteMessage(msgDir string, msg db.Message) error

	// WriteBlock handles the persistence of a code block.
	// The trim flag determines if smart trimming should be applied.
	WriteBlock(msgDir string, block markdown.CodeBlock, trim bool) error

	// WritePatch handles the persistence of a patch block.
	// The trim flag determines if smart trimming should be applied.
	WritePatch(msgDir string, patch markdown.PatchBlock, trim bool) error

	// WritePatchedFile persists the result of a successful patch application.
	// The header is prepended to the content to maintain traceability.
	WritePatchedFile(msgDir string, patch markdown.PatchBlock, header string, content string) error
}
