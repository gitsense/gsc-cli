/**
 * Component: Intent Workflow Manager Persistence
 * Block-UUID: 5589944f-af47-4c24-822d-8d8f0228714e
 * Parent-UUID: 52820a86-0af6-400f-a118-2eaa569313b7
 * Version: 1.0.1
 * Description: Handles session state persistence, including loading sessions from disk, writing session state, and maintaining turn history files.
 * Language: Go
 * Created-at: 2026-04-28T13:47:41.531Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadSession loads an existing session from disk
func LoadSession(sessionID string) (*Manager, error) {
	config, err := NewSessionConfig(sessionID)
	if err != nil {
		return nil, err
	}

	if !config.SessionExists() {
		return nil, fmt.Errorf("session does not exist: %s", sessionID)
	}

	// Create debug logger (disabled by default)
	debugLogger, err := NewDebugLogger(config.GetSessionDir(), false) // Keep disabled for loaded sessions
	if err != nil {
		return nil, err
	}

	sessionPath := config.GetSessionFile()
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read status file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
	}

	// Infer currentTurn from turn count (supports unlimited turns)
	currentTurn := len(session.Turns)
	if currentTurn == 0 {
		currentTurn = 1
	}

	mgr := &Manager{
		config:      config,
		session:     &session,
		processor:   NewProcessorHelper(config),
		debugLogger: debugLogger,
		currentTurn: currentTurn,
	}

	// CRITICAL FIX: Restore m.processInfo from the last running turn
	// This allows StopSession() to send SIGTERM to the actual process
	for i := len(session.Turns) - 1; i >= 0; i-- {
		t := session.Turns[i]
		if t.ProcessInfo.Running && t.ProcessInfo.PID > 0 {
			mgr.processInfo = &ProcessInfo{
				PID:     t.ProcessInfo.PID,
				Command: t.ProcessInfo.Command,
				Running: true,
			}
			break
		}
	}

	return mgr, nil
}

// WriteSessionState writes the session state to disk (public method)
func (m *Manager) WriteSessionState() error {
	return m.writeSessionState()
}

// writeSessionState writes the session state to disk
func (m *Manager) writeSessionState() error {
	sessionPath := m.config.GetSessionFile()

	// Create a deep copy of the session to avoid mutating the in-memory state
	// We use JSON marshal/unmarshal as a simple deep copy mechanism
	sessionCopy := &Session{}
	sessionData, err := json.Marshal(m.session)
	if err != nil {
		m.debugLogger.LogError("Failed to marshal session for copy", err)
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if err := json.Unmarshal(sessionData, sessionCopy); err != nil {
		m.debugLogger.LogError("Failed to unmarshal session copy", err)
		return fmt.Errorf("failed to unmarshal session copy: %w", err)
	}

	// Marshal the modified copy to JSON
	data, err := json.MarshalIndent(sessionCopy, "", "  ")
	if err != nil {
		m.debugLogger.LogError("Failed to marshal session status", err)
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		m.debugLogger.LogError("Failed to write status file", err)
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// writeTurnHistory writes the turn-history.json file
func (m *Manager) writeTurnHistory(turnNumber int) error {
	if m.session == nil {
		return nil
	}
	
	// Build complete turn history from session.json (source of truth)
	var history []map[string]interface{}
	for _, turn := range m.session.Turns {
		historyEntry := map[string]interface{}{
			"session_intent": m.session.Intent,
			"turn":           turn,
		}
		history = append(history, historyEntry)
	}
	
	// Write complete history to the turn directory
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal turn-history.json: %w", err)
	}
	return os.WriteFile(filepath.Join(m.config.GetTurnDir(turnNumber), "turn-history.json"), data, 0644)
}
