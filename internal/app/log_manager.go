/**
 * Component: App Log Manager
 * Block-UUID: 904486ef-8b98-4637-9237-ad28d875dd35
 * Parent-UUID: cb1c2206-48d1-4aed-9be3-b401e521a62b
 * Version: 2.0.0
 * Description: Implements log rotation and management for the native application, ensuring log files do not exceed configured size limits. Updated to support event-based rotation (e.g., on crashes) in addition to size-based rotation.
 * Language: Go
 * Created-at: 2026-03-20T23:05:44.410Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v2.0.0)
 */


package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// RotatingLogWriter implements io.Writer with automatic file rotation
type RotatingLogWriter struct {
	mu               sync.Mutex
	filePath         string
	file             *os.File
	size             int64
	rotateOnNextWrite bool
	eventSuffix      string
}

// NewRotatingLogWriter initializes a new log writer with rotation support
func NewRotatingLogWriter(dataDir string) (*RotatingLogWriter, error) {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	filePath := filepath.Join(logDir, "app.log")
	w := &RotatingLogWriter{filePath: filePath}
	
	if err := w.openFile(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *RotatingLogWriter) openFile() error {
	f, err := os.OpenFile(w.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	w.file = f
	w.size = info.Size()
	return nil
}

// Write implements the io.Writer interface
func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if event-based rotation is pending
	if w.rotateOnNextWrite {
		if err := w.rotateWithSuffix(w.eventSuffix); err != nil {
			fmt.Fprintf(os.Stderr, "Log rotation failed: %v\n", err)
		}
		w.rotateOnNextWrite = false
		w.eventSuffix = ""
	}

	// Check if rotation is needed before writing
	if w.size+int64(len(p)) > settings.AppLogMaxSize {
		timestamp := time.Now().Format("20060102-150405")
		if err := w.rotateWithSuffix(timestamp); err != nil {
			// If rotation fails, we still try to write to the current file 
			// to avoid losing logs, but we return the error.
			fmt.Fprintf(os.Stderr, "Log rotation failed: %v\n", err)
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotateWithSuffix performs the file rotation with a custom suffix
func (w *RotatingLogWriter) rotateWithSuffix(suffix string) error {
	if err := w.file.Close(); err != nil {
		return err
	}

	// Create timestamped backup with custom suffix
	backupPath := fmt.Sprintf("%s.%s", w.filePath, suffix)
	if err := os.Rename(w.filePath, backupPath); err != nil {
		return err
	}

	// Re-open fresh log file
	return w.openFile()
}

// ScheduleRotation schedules a rotation on the next write with a custom suffix
func (w *RotatingLogWriter) ScheduleRotation(event string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rotateOnNextWrite = true
	w.eventSuffix = event
}

// Close closes the underlying log file
func (w *RotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// GetOutputWriters returns a combined writer for both file and console
func GetOutputWriters(dataDir string, foreground bool) (io.Writer, error) {
	fileWriter, err := NewRotatingLogWriter(dataDir)
	if err != nil {
		return nil, err
	}

	if foreground {
		// In foreground mode (Docker), we pipe to both the file and the console
		return io.MultiWriter(os.Stdout, fileWriter), nil
	}

	// In daemon mode, we only pipe to the file
	return fileWriter, nil
}
