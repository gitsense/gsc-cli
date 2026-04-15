/**
 * Component: Agent Debug Logger
 * Block-UUID: 88277edb-7321-4ef5-9bbb-3deb3113616d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: TODO: Update when refactoring is done.
 * Language: Go
 * Created-at: 2026-04-01T03:15:24.540Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DebugLogger provides thread-safe debug logging for Scout sessions
type DebugLogger struct {
	file   *os.File
	mu     sync.Mutex
	enabled bool
}

// NewDebugLogger creates a new debug logger for a session
func NewDebugLogger(sessionDir string, enabled bool) (*DebugLogger, error) {
	if !enabled {
		return &DebugLogger{enabled: false}, nil
	}

	debugPath := filepath.Join(sessionDir, "debug.log")
	file, err := os.Create(debugPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug log: %w", err)
	}

	logger := &DebugLogger{
		file:    file,
		enabled: true,
	}

	logger.Log("DEBUG", "Debug logging initialized")
	logger.Log("DEBUG", fmt.Sprintf("Session directory: %s", sessionDir))
	logger.Log("DEBUG", fmt.Sprintf("Debug log path: %s", debugPath))

	return logger, nil
}

// Log writes a debug message with timestamp
func (dl *DebugLogger) Log(level, message string) {
	if !dl.enabled {
		return
	}

	dl.mu.Lock()
	defer dl.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)
	dl.file.WriteString(line)
	dl.file.Sync() // Flush immediately for real-time debugging
}

// LogProcessSpawn logs subprocess spawning details
func (dl *DebugLogger) LogProcessSpawn(pid int, command string, workingDir string) {
	if !dl.enabled {
		return
	}
	dl.Log("PROCESS", fmt.Sprintf("Spawned subprocess: PID=%d", pid))
	dl.Log("PROCESS", fmt.Sprintf("Command: %s", command))
	dl.Log("PROCESS", fmt.Sprintf("Working directory: %s", workingDir))
}

// LogStreamEvent logs stream processing events
func (dl *DebugLogger) LogStreamEvent(eventType, details string) {
	if !dl.enabled {
		return
	}
	dl.Log("STREAM", fmt.Sprintf("%s: %s", eventType, details))
}

// LogEventWrite logs event writing status
func (dl *DebugLogger) LogEventWrite(eventType string, success bool, err error) {
	if !dl.enabled {
		return
	}
	if success {
		dl.Log("EVENT", fmt.Sprintf("Wrote event: %s", eventType))
	} else {
		dl.Log("EVENT", fmt.Sprintf("Failed to write event %s: %v", eventType, err))
	}
}

// LogProcessExit logs process exit details
func (dl *DebugLogger) LogProcessExit(pid int, exitCode int, err error) {
	if !dl.enabled {
		return
	}
	if err != nil {
		dl.Log("PROCESS", fmt.Sprintf("Process PID=%d exited with error: %v", pid, err))
	} else {
		dl.Log("PROCESS", fmt.Sprintf("Process PID=%d exited with code: %d", pid, exitCode))
	}
}

// LogError logs an error with context
func (dl *DebugLogger) LogError(context string, err error) {
	if !dl.enabled {
		return
	}
	dl.Log("ERROR", fmt.Sprintf("%s: %v", context, err))
}

// Close closes the debug log file
func (dl *DebugLogger) Close() error {
	if !dl.enabled || dl.file == nil {
		return nil
	}
	dl.Log("DEBUG", "Debug logging closed")
	return dl.file.Close()
}
