/*
 * Component: Dump Writer Interface
 * Block-UUID: 5edf749d-9b86-4bf1-80fa-a9b4c9b40aab
 * Parent-UUID: f1a0998b-3727-455e-9b62-3688434912f2
 * Version: 1.4.0
 * Description: Added WriteProvenance to the DumpWriter interface. This method allows the orchestrator to persist the list of chats associated with a specific message, which is essential for the 'merged' dump type.
 * Language: Go
 * Created-at: 2026-03-03T07:37:57.416Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0)
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

	// WriteProvenance persists the chat metadata (provenance) for a message.
	// This is used by the 'merged' type to create the <n>_chats.md file.
	WriteProvenance(msgDir string, chats []db.Chat) error

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
