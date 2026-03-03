/*
 * Component: Dump Writer Interface
 * Block-UUID: 2906ebf3-de4e-4b6e-a262-768f6b187535
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the Strategy interface for different dump types (Tree, Mapped, Text). This allows the orchestrator to remain agnostic of the specific file organization and content transformation logic.
 * Language: Go
 * Created-at: 2026-03-03T02:05:45.801Z
 * Authors: Gemini 3 Flash (v1.0.0)
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
	WriteBlock(msgDir string, block markdown.CodeBlock) error

	// WritePatch handles the persistence of a patch block.
	WritePatch(msgDir string, patch markdown.PatchBlock) error
}
