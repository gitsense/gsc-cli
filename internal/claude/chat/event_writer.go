/**
 * Component: Chat Event Writer
 * Block-UUID: 211d6bd5-0840-4355-ae1f-e6c03064beb3
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Structured event logging for Chat sessions using JSONL format
 * Language: Go
 * Created-at: 2026-04-14T13:19:38.970Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ChatStreamEvent represents a structured event in the stream log
type ChatStreamEvent struct {
	Timestamp string      `json:"timestamp"`
	Type      string      `json:"type"` // "init", "text_delta", "assistant", "usage", "result", "error"
	Data      interface{} `json:"data"`
}

// ChatInitEvent represents initialization data
type ChatInitEvent struct {
	ChatUUID  string `json:"chat_uuid"`
	Model     string `json:"model"`
	SessionID string `json:"session_id"`
	Format    string `json:"format"`
}

// ChatTextDeltaEvent represents text delta data
type ChatTextDeltaEvent struct {
	Delta string `json:"delta"`
	Index int    `json:"index,omitempty"`
}

// ChatUsageEvent represents usage metrics
type ChatUsageEvent struct {
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	Cost                float64 `json:"cost_usd"`
}

// ChatResultEvent represents the final result
type ChatResultEvent struct {
	Result     string `json:"result"`
	StopReason string `json:"stop_reason"`
	DurationMs int    `json:"duration_ms"`
}

// ChatErrorEvent represents an error
type ChatErrorEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ChatEventWriter writes structured JSONL events to a stream file
type ChatEventWriter struct {
	file   *os.File
	writer *bufio.Writer
}

// NewChatEventWriter creates a new event writer for a chat session
func NewChatEventWriter(logFilePath string) (*ChatEventWriter, error) {
	file, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create event log: %w", err)
	}

	return &ChatEventWriter{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// WriteEvent writes a single event to the stream
func (cew *ChatEventWriter) WriteEvent(eventType string, data interface{}) error {
	event := ChatStreamEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      eventType,
		Data:      data,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := cew.writer.Write(eventBytes); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	if err := cew.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return cew.writer.Flush()
}

// WriteInitEvent writes the initialization event
func (cew *ChatEventWriter) WriteInitEvent(init ChatInitEvent) error {
	return cew.WriteEvent("init", init)
}

// WriteTextDeltaEvent writes a text delta event
func (cew *ChatEventWriter) WriteTextDeltaEvent(delta ChatTextDeltaEvent) error {
	return cew.WriteEvent("text_delta", delta)
}

// WriteUsageEvent writes a usage event
func (cew *ChatEventWriter) WriteUsageEvent(usage ChatUsageEvent) error {
	return cew.WriteEvent("usage", usage)
}

// WriteResultEvent writes a result event
func (cew *ChatEventWriter) WriteResultEvent(result ChatResultEvent) error {
	return cew.WriteEvent("result", result)
}

// WriteErrorEvent writes an error event
func (cew *ChatEventWriter) WriteErrorEvent(error ChatErrorEvent) error {
	return cew.WriteEvent("error", error)
}

// Close closes the event writer
func (cew *ChatEventWriter) Close() error {
	var flushErr error
	if cew.writer != nil {
		flushErr = cew.writer.Flush()
	}

	var closeErr error
	if cew.file != nil {
		closeErr = cew.file.Close()
	}

	if flushErr != nil {
		return flushErr
	}
	return closeErr
}
