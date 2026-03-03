/*
 * Component: Dump Writer Interface
 * Block-UUID: 9c341414-0f51-4bc3-a99a-1016e7d80e9a
 * Parent-UUID: 2906ebf3-de4e-4b6e-a262-768f6b187535
 * Version: 1.1.0
 * Description: Updated WriteBlock and WritePatch signatures to accept a trim boolean. This allows implementations to decide whether to apply smart trimming or preserve raw content.
 * Language: Go
 * Created-at: 2026-03-03T04:38:31.000Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
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
}
