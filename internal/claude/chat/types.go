/**
 * Component: Claude Code Chat Types
 * Block-UUID: be30393e-7d04-4dd7-9f17-a5d655b15f6e
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Chat-specific data structures and constants for stream processing and execution history. Added DebugLogger, EventWriter, and LastAssistantMessage fields to StreamProcessor for improved stream handling.
 * Language: Go
 * Created-at: 2026-04-01T15:26:44.195Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package chat

import (
	"bytes"
	"os"

	"github.com/gitsense/gsc-cli/internal/claude"
)

// StreamResult holds the output from stream processing
type StreamResult struct {
	FullResponse string
	Usage        claude.Usage
	Cost         float64
	SessionID    string
	ExitCode     int
	StderrOutput string
}

// StreamProcessor handles the stream event processing logic
type StreamProcessor struct {
	LogFile        *os.File
	LogDir         string
	Format         string
	EffectiveModel string
	CurrentTime    string
	StderrBuf      *bytes.Buffer
	
	// NEW: Add debug logger and event writer for improved stream handling
	DebugLogger    *ChatDebugLogger
	EventWriter    *ChatEventWriter
	
	// NEW: Track last assistant message for fallback processing
	LastAssistantMessage string
}

// HistoryEntry represents a single execution record in history.jsonl
type HistoryEntry struct {
	Timestamp   string `json:"timestamp"`
	ChatUUID    string `json:"chat_uuid"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir"`
	ExitCode    int    `json:"exit_code"`
	Stderr      string `json:"stderr"`
	DurationMs  int64  `json:"duration_ms"`
}

// Constants for stream processing
const (
	MaxTokenSize    = 10 * 1024 * 1024 // 10MB max buffer
	InitialBufSize  = 64 * 1024        // 64KB initial buffer
	DirPermissions  = 0755
	FilePermissions = 0644
)
