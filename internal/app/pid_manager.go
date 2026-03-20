/*
 * Component: App PID Manager
 * Block-UUID: 58389287-28cc-4c58-a38f-98676b14367a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Manages the creation, validation, and deletion of the application's PID file to prevent multiple instances and track the running process.
 * Language: Go
 * Created-at: 2026-03-20T23:05:21.143Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// WritePID writes the current process ID to the PID file in the data directory.
func WritePID(dataDir string, pid int) error {
	pidFile := filepath.Join(dataDir, settings.AppPIDFileName)
	
	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory for PID file: %w", err)
	}

	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// RemovePID deletes the PID file from the data directory.
func RemovePID(dataDir string) error {
	pidFile := filepath.Join(dataDir, settings.AppPIDFileName)
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// IsProcessRunning checks if a process is already running based on the PID file.
// Returns (isRunning, pid, error).
func IsProcessRunning(dataDir string) (bool, int, error) {
	pidFile := filepath.Join(dataDir, settings.AppPIDFileName)
	
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	// Check if the process actually exists in the OS
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0, nil
	}

	// On Unix, FindProcess always succeeds. We must send signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, pid, nil
	}

	return false, 0, nil
}
