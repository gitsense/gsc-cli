/**
 * Component: Scout Session Configuration Helper
 * Block-UUID: 612d2411-6795-4796-9bb0-94d6b67594cc
 * Parent-UUID: cebd85e7-10ea-446a-a5d7-945f349007b8
 * Version: 1.0.4
 * Description: Path resolution and session directory helpers for Scout operations
 * Language: Go
 * Created-at: 2026-04-03T03:05:52.483Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package scout

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// SessionConfig holds path configuration for a scout session
type SessionConfig struct {
	SessionID string
	GSCHome   string
}

// NewSessionConfig creates a new SessionConfig from a session ID
func NewSessionConfig(sessionID string) (*SessionConfig, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	return &SessionConfig{
		SessionID: sessionID,
		GSCHome:   gscHome,
	}, nil
}

// GetSessionDir returns the absolute path to the session directory
func (sc *SessionConfig) GetSessionDir() string {
	return filepath.Join(sc.GSCHome, settings.ScoutSessionsDirRelPath, sc.SessionID)
}

// GetStatusFile returns the absolute path to the status.json file
func (sc *SessionConfig) GetStatusFile() string {
	return filepath.Join(sc.GetSessionDir(), settings.ScoutStatusFileName)
}

// GetIntentFile returns the absolute path to the intent.json file
func (sc *SessionConfig) GetIntentFile(turn int) string {
	return filepath.Join(sc.GetTurnDir(turn), settings.ScoutIntentFileName)
}

// GetReferencesDir returns the absolute path to the references directory
func (sc *SessionConfig) GetReferencesDir() string {
	return filepath.Join(sc.GetSessionDir(), settings.ScoutReferenceDirName)
}

// GetTurnDir returns the absolute path to a specific turn directory
func (sc *SessionConfig) GetTurnDir(turn int) string {
	return filepath.Join(sc.GetSessionDir(), fmt.Sprintf("turn-%d", turn))
}

// GetTurnLogFile returns the absolute path to a turn's raw stream log file
// filename should be in format: raw-stream-<timestamp>.ndjson
func (sc *SessionConfig) GetTurnLogFile(turn int, filename string) string {
	return filepath.Join(sc.GetTurnDir(turn), filename)
}

// GetCodebaseOverviewFile returns the absolute path to codebase-overview.json
func (sc *SessionConfig) GetCodebaseOverviewFile() string {
	return filepath.Join(sc.GetTurnDir(1), "codebase-overview.json")
}

// GetReferenceFile returns the absolute path to a reference file
// refType is the reference category (e.g., "intent", "candidates", "brain")
func (sc *SessionConfig) GetReferenceFile(refType string) string {
	return filepath.Join(sc.GetReferencesDir(), fmt.Sprintf("%s.json", refType))
}

// SessionExists checks if the session directory exists
func (sc *SessionConfig) SessionExists() bool {
	_, err := os.Stat(sc.GetSessionDir())
	return err == nil
}

// InitializeSessionDirs creates all necessary directories for a scout session
func (sc *SessionConfig) InitializeSessionDirs() error {
	dirs := []string{
		sc.GetSessionDir(),
		sc.GetReferencesDir(),
		sc.GetTurnDir(1),
		sc.GetTurnDir(2),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// CleanupSessionDir removes a scout session directory
func (sc *SessionConfig) CleanupSessionDir() error {
	if !sc.SessionExists() {
		return nil // Already doesn't exist
	}
	return os.RemoveAll(sc.GetSessionDir())
}

// BaseScoutDir returns the base scout sessions directory
func BaseScoutDir(gscHome string) string {
	return filepath.Join(gscHome, settings.ScoutSessionsDirRelPath)
}

// ListSessions returns all session IDs in the scout directory
func ListSessions(gscHome string) ([]string, error) {
	baseDir := BaseScoutDir(gscHome)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read scout directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() {
			sessions = append(sessions, entry.Name())
		}
	}

	return sessions, nil
}
