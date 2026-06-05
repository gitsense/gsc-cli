/**
 * Component: App PID Manager
 * Block-UUID: 1d2dc93f-e0d6-4265-ad6c-17ba8b640757
 * Parent-UUID: dec1aa6f-c98f-432b-84dd-647d56c8c549
 * Version: 2.0.0
 * Description: Manages the creation, validation, and deletion of the application's PID file to prevent multiple instances and track the running process. Updated to track both supervisor and child PIDs, enabling proper stop functionality by targeting the supervisor process.
 * Language: Go
 * Created-at: 2026-03-20T23:55:04.663Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v2.0.0)
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

// WritePID writes the supervisor PID and child PID to the PID file in the data directory.
// The supervisor PID is written first, followed by the child PID, separated by a space.
func WritePID(dataDir string, childPid int) error {
	pidFile := filepath.Join(dataDir, settings.AppPIDFileName)
	
	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory for PID file: %w", err)
	}

	// Write supervisor PID and child PID
	supervisorPid := os.Getpid()
	content := fmt.Sprintf("%d %d\n", supervisorPid, childPid)
	return os.WriteFile(pidFile, []byte(content), 0644)
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
// Returns (isRunning, supervisorPid, childPid, error).
// The supervisor PID is checked to determine if the process is running.
func IsProcessRunning(dataDir string) (bool, int, int, error) {
	pidFile := filepath.Join(dataDir, settings.AppPIDFileName)
	
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, 0, nil
		}
		return false, 0, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse supervisor PID and child PID
	parts := strings.Fields(string(data))
	if len(parts) < 2 {
		return false, 0, 0, fmt.Errorf("invalid PID file format: expected 'supervisorPid childPid', got: %s", string(data))
	}

	supervisorPid, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, 0, 0, fmt.Errorf("invalid supervisor PID in file: %w", err)
	}

	childPid, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, 0, 0, fmt.Errorf("invalid child PID in file: %w", err)
	}

	// Check if the supervisor process actually exists in the OS
	process, err := os.FindProcess(supervisorPid)
	if err != nil {
		return false, 0, 0, nil
	}

	// On Unix, FindProcess always succeeds. We must send signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, supervisorPid, childPid, nil
	}

	return false, 0, 0, nil
}

// StopProcess sends a SIGTERM signal to the supervisor process identified in the PID file.
// The supervisor will handle stopping the child process and cleaning up.
func StopProcess(dataDir string) error {
	running, supervisorPid, _, err := IsProcessRunning(dataDir)
	if err != nil {
		return err
	}

	if !running {
		return fmt.Errorf("no process found running for data directory: %s", dataDir)
	}

	process, err := os.FindProcess(supervisorPid)
	if err != nil {
		return fmt.Errorf("failed to find supervisor process %d: %w", supervisorPid, err)
	}

	return process.Signal(syscall.SIGTERM)
}
