/*
 * Component: App Log Manager
 * Block-UUID: cb1c2206-48d1-4aed-9be3-b401e521a62b
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements log rotation and management for the native application, ensuring log files do not exceed configured size limits.
 * Language: Go
 * Created-at: 2026-03-20T23:05:44.410Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// RotatingLogWriter implements io.Writer with automatic file rotation
type RotatingLogWriter struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
	size     int64
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

	// Check if rotation is needed before writing
	if w.size+int64(len(p)) > settings.AppLogMaxSize {
		if err := w.rotate(); err != nil {
			// If rotation fails, we still try to write to the current file 
			// to avoid losing logs, but we return the error.
			fmt.Fprintf(os.Stderr, "Log rotation failed: %v\n", err)
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate performs the file shifting logic
func (w *RotatingLogWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}

	// Shift existing backups: .2 -> .3, .1 -> .2, current -> .1
	for i := settings.AppLogMaxBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.filePath, i)
		newPath := fmt.Sprintf("%s.%d", w.filePath, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	// Rename current file to .1
	backupPath := w.filePath + ".1"
	if err := os.Rename(w.filePath, backupPath); err != nil {
		return err
	}

	// Re-open fresh log file
	return w.openFile()
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
