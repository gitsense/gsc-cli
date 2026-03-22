/**
 * Component: Claude Code Data Models
 * Block-UUID: ea393194-0f3a-4ffa-b837-493ab74ac670
 * Parent-UUID: 0d95d41c-b017-4bad-aed6-d27d72f3aa9c
 * Version: 1.4.0
 * Description: Defines the data structures for Claude Code CLI integration, including API responses, usage metrics, and archive settings.
 * Language: Go
 * Created-at: 2026-03-22T21:41:57.559Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
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
	SessionID string `json:"session_id"`
}

// StreamResultEvent represents the final result event containing usage stats and cost.
type StreamResultEvent struct {
	Type       string                 `json:"type"`
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
