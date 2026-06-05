/**
 * Component: Chat Debug Logger
 * Block-UUID: f82f6fb4-c5e5-46d6-bf45-a402a73aaaf6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Debug logging for Chat sessions to diagnose stream processing issues
 * Language: Go
 * Created-at: 2026-04-14T13:19:38.970Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChatDebugLogger provides thread-safe debug logging for Chat sessions
type ChatDebugLogger struct {
	file   *os.File
	mu     sync.Mutex
	enabled bool
}

// NewChatDebugLogger creates a new debug logger for a chat session
func NewChatDebugLogger(logDir string, enabled bool) (*ChatDebugLogger, error) {
	if !enabled {
		return &ChatDebugLogger{enabled: false}, nil
	}

	debugPath := filepath.Join(logDir, "debug.log")
	file, err := os.Create(debugPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug log: %w", err)
	}

	logger := &ChatDebugLogger{
		file:    file,
		enabled: true,
	}

	logger.Log("DEBUG", "Debug logging initialized")
	logger.Log("DEBUG", fmt.Sprintf("Log directory: %s", logDir))
	logger.Log("DEBUG", fmt.Sprintf("Debug log path: %s", debugPath))

	return logger, nil
}

// Log writes a debug message with timestamp
func (cdl *ChatDebugLogger) Log(level, message string) {
	if !cdl.enabled {
		return
	}

	cdl.mu.Lock()
	defer cdl.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)
	cdl.file.WriteString(line)
	cdl.file.Sync()
}

// LogStreamEvent logs stream processing events
func (cdl *ChatDebugLogger) LogStreamEvent(eventType, details string) {
	if !cdl.enabled {
		return
	}
	cdl.Log("STREAM", fmt.Sprintf("%s: %s", eventType, details))
}

// LogError logs an error with context
func (cdl *ChatDebugLogger) LogError(context string, err error) {
	if !cdl.enabled {
		return
	}
	cdl.Log("ERROR", fmt.Sprintf("%s: %v", context, err))
}

// LogMetrics logs metrics information
func (cdl *ChatDebugLogger) LogMetrics(message string) {
	if !cdl.enabled {
		return
	}
	cdl.Log("METRICS", message)
}

// Close closes the debug log file
func (cdl *ChatDebugLogger) Close() error {
	if !cdl.enabled || cdl.file == nil {
		return nil
	}
	cdl.Log("DEBUG", "Debug logging closed")
	return cdl.file.Close()
}
