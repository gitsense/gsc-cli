/**
 * Component: App Health Status Manager
 * Block-UUID: d1315c21-e2a5-415c-a82d-7c021813fab3
 * Parent-UUID: a3bed39c-7cda-467e-88d7-2fd59fa713ac
 * Version: 1.1.0
 * Description: Manages the persistence and retrieval of the health status file (health.json), which tracks the application's lifecycle including crashes, restarts, uptime, and port configuration.
 * Language: Go
 * Created-at: 2026-05-12T14:38:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HealthStatus represents the current health and lifecycle state of the application
type HealthStatus struct {
	SupervisorPID int        `json:"supervisor_pid"`
	ChildPID      int        `json:"child_pid"`
	StartedAt     time.Time  `json:"started_at"`
	LastCrashAt   *time.Time `json:"last_crash_at,omitempty"`
	CrashCount    int        `json:"crash_count"`
	RestartCount  int        `json:"restart_count"`
	UptimeSeconds int        `json:"uptime_seconds"`
	Status        string     `json:"status"` // "running", "stopped", "crashed"
	Port          string     `json:"port"`   // The port the application is listening on
}

const HealthFileName = "health.json"

// GetHealthFilePath returns the absolute path to the health status file
func GetHealthFilePath(dataDir string) string {
	return filepath.Join(dataDir, HealthFileName)
}

// LoadHealthStatus reads the health status from the data directory
// Returns nil, nil if the file does not exist
func LoadHealthStatus(dataDir string) (*HealthStatus, error) {
	path := GetHealthFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read health status: %w", err)
	}

	var status HealthStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to parse health status: %w", err)
	}

	return &status, nil
}

// SaveHealthStatus writes the health status to the data directory
func SaveHealthStatus(dataDir string, status *HealthStatus) error {
	path := GetHealthFilePath(dataDir)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// InitializeHealthStatus creates a new health status with initial values
func InitializeHealthStatus(dataDir string, supervisorPID int, port string) *HealthStatus {
	return &HealthStatus{
		SupervisorPID: supervisorPID,
		StartedAt:     time.Now().UTC(),
		CrashCount:    0,
		RestartCount:  0,
		Status:        "running",
		Port:          port,
	}
}

// UpdateUptime updates the uptime_seconds field based on the start time
func UpdateUptime(status *HealthStatus) {
	status.UptimeSeconds = int(time.Since(status.StartedAt).Seconds())
}
