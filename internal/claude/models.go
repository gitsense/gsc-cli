/**
 * Component: Claude Code Data Models
 * Block-UUID: f2d4cd00-7e8c-444c-a190-4ebc9e83df1d
 * Parent-UUID: e6cef371-716b-49aa-b311-8d4859d1e73b
 * Version: 1.1.0
 * Description: Defines the data structures for Claude Code CLI integration, including API responses, usage metrics, and archive settings.
 * Language: Go
 * Created-at: 2026-03-22T15:34:50.192Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package claude

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
	ChunkSize int
	MaxFiles  int
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
