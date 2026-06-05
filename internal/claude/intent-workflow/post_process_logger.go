/*
 * Component: Post Process Logger
 * Block-UUID: b3b6a2ab-d75d-4eb6-845d-2e23892a9a7f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Structured logger for post-processing steps, writing to turn-N/post-process.log.
 * Language: Go
 * Created-at: 2026-04-26T02:55:47.539Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package intent_workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"
)


// PostProcessLogger writes structured logs to turn-N/post-process.log
type PostProcessLogger struct {
	file   *os.File
	writer *bufio.Writer
}

// NewPostProcessLogger creates or opens the post-process log file for a turn
func NewPostProcessLogger(sessionDir string, turn int) (*PostProcessLogger, error) {
	turnDir := filepath.Join(sessionDir, fmt.Sprintf("turn-%d", turn))
	logPath := filepath.Join(turnDir, "post-process.log")

	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create post-process log: %w", err)
	}

	return &PostProcessLogger{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// Log writes a timestamped log entry
func (l *PostProcessLogger) Log(level, message string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	l.writer.WriteString(fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message))
	l.writer.Flush()
}

// Close flushes and closes the log file
func (l *PostProcessLogger) Close() error {
	if l.writer != nil {
		l.writer.Flush()
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
