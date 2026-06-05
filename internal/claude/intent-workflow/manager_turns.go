/**
 * Component: Intent Workflow Manager Turns
 * Block-UUID: 8463136c-a0d9-43ce-a178-a605e4effa3e
 * Parent-UUID: 8d1f6257-15b0-4f21-94e2-f71eb8b41d9e
 * Version: 1.6.0
 * Description: Orchestrates the lifecycle of agent turns including discovery, change, and correction phases. Handles turn initialization, state transitions, and subprocess spawning. Added pre-flight cleanup logic to remove orphaned .change-meta.json files before starting a new change turn, preventing ghost metadata from confusing the AI. Fixed variable shadowing issue with err declaration in StartChangeTurn. Added AddSkippedDiscoveryTurn() method to support skipping discovery turns when using --skip-discovery flag. Updated StartDiscoveryTurn() to allow starting from "discovery_complete" status to support Discovery -> Discovery flow.
 * Language: Go
 * Created-at: 2026-04-29T02:43:08.639Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
 */


package intent_workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StartDiscoveryTurn initiates the discovery phase and spawns subprocess
func (m *Manager) StartDiscoveryTurn() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting discovery turn")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directories: %d", len(m.session.WorkingDirectories)))

	if m.session.Status != "discovery" && m.session.Status != "discovery_complete" && m.session.Status != "error" {
		return fmt.Errorf("cannot start discovery: session status is %s", m.session.Status)
	}

	// Calculate next turn number dynamically
	nextTurn := m.GetNextTurnNumber()
	m.currentTurn = nextTurn
	m.session.Status = "discovery"

	// Ensure turn directory exists BEFORE any operations that need it
	if err := m.config.EnsureTurnDir(nextTurn); err != nil {
		m.debugLogger.LogError("Failed to create turn directory", err)
		m.markAsError("DIR_CREATE_FAILED", fmt.Sprintf("Failed to create turn directory: %v", err))
		return err
	}

	// Write intent to turn directory (preserves intent for this specific turn)
	intentPath := filepath.Join(m.config.GetTurnDir(nextTurn), "intent.md")
	if err := os.WriteFile(intentPath, []byte(m.session.Intent), 0644); err != nil {
		m.debugLogger.LogError("Failed to write intent file", err)
		m.markAsError("INTENT_WRITE_FAILED", fmt.Sprintf("Failed to write intent file: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent written to: %s", intentPath))

	// Prepare discovery turn (generate input schema)
	if err := m.PrepareContext(); err != nil {
		m.debugLogger.LogError("PrepareContext failed", err)
		return err
	}

	// Check if session already errored out (no context)
	if m.session.Status == "error" {
		m.debugLogger.Log("DEBUG", "Session already in error state, not spawning subprocess")
		return nil // Don't spawn Claude, already handled
	}

	// Close previous eventWriter if it exists to prevent resource leaks
	if m.eventWriter != nil {
		m.eventWriter.Close()
	}

	// Create log file for this turn
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	var err error
	m.eventWriter, err = NewEventWriter(logPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create event writer", err)
		m.markAsError("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Created event writer: %s", logPath))

	// Create new turn state and append to turns slice
	newTurn := TurnState{
		TurnNumber:  nextTurn,
		TurnType:    "discovery",
		Status:      "running",
		StartedAt:   time.Now(),
		LogPath:     logPath,
		ProcessInfo: ProcessInfo{Running: true},
	}
	m.session.Turns = append(m.session.Turns, newTurn)
	
	// Write session state to persist log paths
	if err := m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Log paths stored in session state")

	// Write turn history to disk
	if err := m.writeTurnHistory(m.currentTurn); err != nil {
		m.debugLogger.LogError("Failed to write turn history", err)
	}

	// Spawn subprocess for discovery turn (defined in spawn.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn, "discovery", []string{}); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsError("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", "Subprocess spawned successfully")

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// AddSkippedDiscoveryTurn creates a virtual skipped discovery turn without spawning a subprocess.
// This is used when the user specifies --skip-discovery flag to proceed directly to change.
func (m *Manager) AddSkippedDiscoveryTurn() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Adding skipped discovery turn")

	// Calculate next turn number
	nextTurn := m.GetNextTurnNumber()
	m.currentTurn = nextTurn

	// Record timestamps
	now := time.Now()

	// Create skipped discovery turn state
	turn := TurnState{
		TurnNumber:  nextTurn,
		TurnType:    "discovery",
		Status:      "skipped",
		StartedAt:   now,
		CompletedAt: &now,
		LogPath:     "", // No log file for skipped turns
		ProcessInfo: ProcessInfo{
			PID:     0,
			Command: "",
			Running: false,
		},
		Usage: nil, // No token usage for skipped turns
		Result: &TurnResult{
			Discovery: &DiscoveryResult{
				Candidates:        []Candidate{},
				TotalFound:        0,
				MissingFiles:      []MissingFile{},
				KeywordAssessment: nil,
				DiscoveryLog: &DiscoveryLog{
					IntentKeywords:       []string{},
					PivotChecks:          []string{},
					Methodology:          "Discovery skipped by user request",
					TotalCandidatesFound: 0,
					TopCandidatesReturned: 0,
					ValidationMethod:     "N/A",
				},
				Coverage: "Discovery skipped",
			},
		},
	}

	// Append turn to session
	m.session.Turns = append(m.session.Turns, turn)

	// Update session status to discovery_complete
	m.session.Status = "discovery_complete"

	// Write session state (do NOT call writeTurnHistory - no turn directory exists)
	if err := m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}

	m.debugLogger.Log("DEBUG", fmt.Sprintf("Skipped discovery turn %d added successfully", nextTurn))
	return nil
}

// StartChangeTurn initiates the change phase for in-place code editing
func (m *Manager) StartChangeTurn(intent string) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting change turn")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Change intent: %s", m.truncateForLog(intent, 100)))

	if m.session.Status != "discovery_complete" {
		return fmt.Errorf("cannot start change: discovery not complete (current status: %s)", m.session.Status)
	}

	// Pre-flight cleanup: Remove orphaned .change-meta.json files
	m.debugLogger.Log("DEBUG", "Performing pre-flight cleanup")
	orphanedFiles, cleanupErr := m.cleanupOrphanedMetadata()
	if cleanupErr != nil {
		m.debugLogger.LogError("Pre-flight cleanup failed", cleanupErr)
		// Don't fail the turn for cleanup errors, just log and continue
	} else if len(orphanedFiles) > 0 {
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Removed %d orphaned metadata file(s)", len(orphanedFiles)))
		for _, f := range orphanedFiles {
			m.debugLogger.Log("DEBUG", fmt.Sprintf("  - %s", f))
		}
	}

	// Calculate next turn number dynamically
	nextTurn := m.GetNextTurnNumber()
	m.currentTurn = nextTurn
	m.session.Status = "change"

	// Ensure turn directory exists
	if err := m.config.EnsureTurnDir(nextTurn); err != nil {
		m.debugLogger.LogError("Failed to create turn directory", err)
		m.markAsError("DIR_CREATE_FAILED", fmt.Sprintf("Failed to create turn directory: %v", err))
		return err
	}

	// Write intent to turn directory
	intentPath := filepath.Join(m.config.GetTurnDir(nextTurn), "intent.md")
	if err := os.WriteFile(intentPath, []byte(intent), 0644); err != nil {
		m.debugLogger.LogError("Failed to write intent file", err)
		m.markAsError("INTENT_WRITE_FAILED", fmt.Sprintf("Failed to write intent file: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent written to: %s", intentPath))

	// Close previous eventWriter if it exists to prevent resource leaks
	if m.eventWriter != nil {
		m.eventWriter.Close()
	}

	// Create log file for this turn
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	var err error
	m.eventWriter, err = NewEventWriter(logPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create event writer", err)
		m.markAsError("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Created event writer: %s", logPath))

	// Create new turn state and append to turns slice
	newTurn := TurnState{
		TurnNumber:  nextTurn,
		TurnType:    "change",
		Status:      "running",
		StartedAt:   time.Now(),
		LogPath:     logPath,
		ProcessInfo: ProcessInfo{Running: true},
	}
	m.session.Turns = append(m.session.Turns, newTurn)
	
	// Write session state to persist log paths
	if err := m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Log paths stored in session state")

	// Write turn history to disk
	if err := m.writeTurnHistory(m.currentTurn); err != nil {
		m.debugLogger.LogError("Failed to write turn history", err)
	}

	// Spawn subprocess for change turn (defined in spawn.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn, "change", orphanedFiles); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsError("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", "Subprocess spawned successfully")

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// cleanupOrphanedMetadata removes any orphaned .change-meta.json files from the working directories.
// Returns a list of files that were removed (for AI context injection).
func (m *Manager) cleanupOrphanedMetadata() ([]string, error) {
	var orphanedFiles []string
	
	// Find all .change-meta.json files
	metaFiles, err := FindChangeMetaFiles(m.session.WorkingDirectories)
	if err != nil {
		m.debugLogger.LogError("Failed to find orphaned metadata files", err)
		return nil, err
	}
	
	// Remove each orphaned file
	for _, metaFile := range metaFiles {
		m.debugLogger.Log("CLEANUP", fmt.Sprintf("Removing orphaned metadata: %s", metaFile))
		if err := os.Remove(metaFile); err != nil {
			m.debugLogger.LogError("Failed to remove orphaned metadata", err)
			// Continue with other files even if one fails
		} else {
			orphanedFiles = append(orphanedFiles, metaFile)
		}
	}
	
	return orphanedFiles, nil
}

// StartCorrectionTurn orchestrates a format-correction subprocess for the
// specified turn. It delegates spawning to spawn.go and provides the
// same lifecycle wrapper as StartDiscoveryTurn and StartChangeTurn.
//
// turnNumber is the turn whose output needs correction.
// modelID is the full Claude model ID (e.g. "claude-haiku-4-5-20251001").
func (m *Manager) StartCorrectionTurn(turnNumber int, modelID string) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", fmt.Sprintf("Starting correction turn for turn %d with model %s", turnNumber, modelID))

	turnState := m.getTurnState(turnNumber)
	if turnState == nil {
		return fmt.Errorf("turn %d not found in session", turnNumber)
	}

	// Update correction metadata before spawning so status is visible immediately.
	turnState.CorrectionAttempts++
	turnState.CorrectionModel = modelID
	turnState.CorrectionStatus = "running"
	if err := m.writeSessionState(); err != nil {
		return fmt.Errorf("failed to persist correction start: %w", err)
	}

	// Spawn the correction subprocess (defined in spawn.go).
	if err := m.spawnCorrectionSubprocess(turnNumber, modelID); err != nil {
		m.debugLogger.LogError("Failed to spawn correction subprocess", err)
		turnState.CorrectionStatus = CorrectionStatusFailed
		_ = m.writeSessionState()
		return fmt.Errorf("correction subprocess failed: %w", err)
	}

	m.debugLogger.Log("DEBUG", "Correction subprocess completed")
	return nil
}

// MarkTurnComplete transitions a turn to its complete state based on turn type
func (m *Manager) MarkTurnComplete(turnType string) error {
	switch turnType {
	case "discovery":
		if m.session.Status != "discovery" {
			return fmt.Errorf("cannot mark complete: not in discovery state, current status: %s", m.session.Status)
		}
		m.session.Status = "discovery_complete"
	case "change":
		if m.session.Status != "change" {
			return fmt.Errorf("cannot mark complete: not in change state, current status: %s", m.session.Status)
		}
		m.session.Status = "change_complete"
		m.session.CompletedAt = &[]time.Time{time.Now()}[0]
	default:
		return fmt.Errorf("unknown turn type: %s", turnType)
	}
	return m.writeSessionState()
}

// getTurnState returns a pointer to the TurnState for the given turn number,
// or nil if the turn does not exist in the session.
func (m *Manager) getTurnState(turnNumber int) *TurnState {
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == turnNumber {
			return &m.session.Turns[i]
		}
	}
	return nil
}

// GetNextTurnNumber calculates the next turn number dynamically
func (m *Manager) GetNextTurnNumber() int {
	if len(m.session.Turns) == 0 {
		return 1
	}
	return m.session.Turns[len(m.session.Turns)-1].TurnNumber + 1
}

// GetLastCompletedTurn returns the highest turn number that has completed successfully
// Returns 0 if no turn has completed (new session)
func (m *Manager) GetLastCompletedTurn() (int, error) {
	if m.session == nil {
		return 0, nil
	}

	// Find the last turn with status "complete"
	for i := len(m.session.Turns) - 1; i >= 0; i-- {
		if m.session.Turns[i].Status == "complete" {
			return m.session.Turns[i].TurnNumber, nil
		}
	}

	return 0, nil
}
