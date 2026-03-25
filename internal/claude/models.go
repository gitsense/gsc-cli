/**
 * Component: Claude Code Data Models
 * Block-UUID: f5cd823c-5202-4355-95f1-c77941f176aa
 * Parent-UUID: 928885b3-b03c-47b1-9bb7-75724f85882f
 * Version: 1.10.0
 * Description: Updated to support cache-optimized context file construction with bucket-based organization. Added ContextFile struct reference and ensured compatibility with context parser and bucketer.
 * Language: Go
 * Created-at: 2026-03-25T03:47:11.844Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), ..., GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), claude-haiku-4-5-20251001 (v1.10.0)
 */


package claude

import (
	"github.com/gitsense/gsc-cli/internal/context"
)

// ClaudeResponse represents the JSON response from the Claude Code CLI.
type ClaudeResponse struct {
	Result     string `json:"result"`
	SessionID  string `json:"session_id"`
	Usage      Usage  `json:"usage"`
	Cost       float64 `json:"cost"`
}

// Usage represents token usage metrics.
type Usage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens    int `json:"cache_read_input_tokens"`
}

// Settings defines the configuration for the Tiered Rolling Archive.
type Settings struct {
	ChunkSize int    `json:"chunk_size"`
	MaxFiles  int    `json:"max_files"`
	Model     string `json:"model"` // New field
}

// MessageFile represents a single message in the JSON files.
type MessageFile struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ActiveWindow represents the structure of messages-active.json.
type ActiveWindow struct {
	ArchiveMap ArchiveMap   `json:"archive_map"`
	Messages   []MessageFile `json:"messages"`
}

// ArchiveMap provides a summary of available archives.
type ArchiveMap struct {
	Files []ArchiveFile `json:"files"`
}

// ArchiveFile represents metadata for a single archive file.
type ArchiveFile struct {
	Name     string `json:"name"`
	Hash     string `json:"hash"`
	Messages int    `json:"messages"`
}

// StreamEvent represents the base structure for stream-json events.
type StreamEvent struct {
	Type string `json:"type"`
}

// AssistantMessageEvent represents the full assistant message event containing text content
type AssistantMessageEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// ContentBlockDeltaEvent represents a streaming content block delta event with thinking or text
type ContentBlockDeltaEvent struct {
	Type  string `json:"type"`
	Event struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		Delta struct {
			Type     string `json:"type"`
			Thinking string `json:"thinking"`
			Text     string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

// PlaceholderMap holds the template variables for text replacement
type PlaceholderMap struct {
	ModelName string
	UTCTime   string
}

// TextDeltaEvent represents a chunk of text content.
type TextDeltaEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

// StreamUsageEvent represents the final usage metrics.
type StreamUsageEvent struct {
	Type  string `json:"type"`
	Usage Usage  `json:"usage"`
	Cost  float64 `json:"cost"`
}

// StreamErrorEvent represents an error from the CLI.
type StreamErrorEvent struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// SystemInitEvent represents the first event from Claude containing session info
type SystemInitEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	Model     string `json:"model"`
	SessionID         string `json:"session_id"`         // CRITICAL: Fixed missing JSON tag
	CWD               string `json:"cwd"`                // Working directory for debugging
	UUID              string `json:"uuid"`               // Unique event ID for tracing
	ClaudeCodeVersion string `json:"claude_code_version"` // Claude Code CLI version
}

// StreamResultEvent represents the final result event containing usage stats and cost.
type StreamResultEvent struct {
	Type       string                 `json:"type"`
	Result     string                 `json:"result"`
	Subtype    string                 `json:"subtype"`
	DurationMs int                    `json:"duration_ms"`
	StopReason string                 `json:"stop_reason"`
	Usage      Usage                  `json:"usage"`
	ModelUsage map[string]ModelStats  `json:"modelUsage"`
	TotalCost  float64                `json:"total_cost_usd"`
}

// ModelStats represents per-model usage details.
type ModelStats struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
	CostUSD                  float64 `json:"costUSD"`
	ContextWindow            int     `json:"contextWindow"`
	MaxOutputTokens          int     `json:"maxOutputTokens"`
}

// MapFile represents the structure of messages.map, the single entry point for context reconstruction.
type MapFile struct {
	Version        string      `json:"version"`        // Map file version
	ReadSequence   []string    `json:"read_sequence"`   // Ordered list of files to read (stable-to-volatile)
	ContextFiles   []FileMeta  `json:"context_files"`   // Metadata for context bucket files
	CliOutputFiles []FileMeta  `json:"cli_output_files"` // Metadata for CLI output files
	Messages       MessagesMeta `json:"messages"`       // Metadata for active/archive message files
}

// FileMeta represents metadata for a single file in the messages directory.
type FileMeta struct {
	ID        string       `json:"id"`        // Unique identifier (e.g., "context-range-2600-2699")
	File      string       `json:"file"`      // Filename (e.g., "context-range-2600-2699.md")
	MinID     int64        `json:"min_id"`    // Minimum message ID in bucket (for context files)
	MaxID     int64        `json:"max_id"`    // Maximum message ID in bucket (for context files)
	DBID      int64        `json:"db_id"`     // Database message ID (for CLI output files)
	Size      int          `json:"size"`      // Total size in bytes
	Stability string       `json:"stability"` // "high", "medium", "low"
	FileCount int          `json:"file_count"` // Number of files in bucket
	Files     []FileEntry  `json:"files"`     // List of files in this bucket (for context files)
	Lifecycle string       `json:"lifecycle"` // "long-lived" or "volatile" (for CLI output files)
	Type      string       `json:"type"`
}

// FileEntry represents a single file entry within a context bucket.
type FileEntry struct {
	ChatID int64  `json:"chat_id"` // Database message ID
	Name   string `json:"name"`    // File path (e.g., "src/index.js")
	Size   int    `json:"size"`    // File size in bytes
}

// MessagesMeta represents metadata for active and archive message files.
type MessagesMeta struct {
	Active   string   `json:"active"`   // Filename of active window
	Archives []string `json:"archives"` // List of archive filenames (oldest to newest)
}

// Bucket represents a temporary structure for grouping files during bucketing.
type Bucket struct {
	MinID     int64       // Minimum message ID in bucket
	MaxID     int64       // Maximum message ID in bucket
	Files     []FileEntry // Files in this bucket
	TotalSize int         // Total size in bytes
}

// ContextFile is an alias for context.ContextFile for convenience in the claude package.
// This allows the claude package to reference the context file structure without
// circular dependencies.
type ContextFile = context.ContextFile

// Constants for bucketing strategy
const (
	MaxFileSizeKB      = 10  // Maximum size per file in KB
	MaxBucketSizeKB    = 40  // Maximum bucket size in KB
	LeewayKB           = 2   // Allowed growth per file in KB
	MaxBucketSizeBytes = MaxBucketSizeKB * 1024
	MaxFileSizeBytes   = MaxFileSizeKB * 1024
	LeewayBytes       = LeewayKB * 1024
)
