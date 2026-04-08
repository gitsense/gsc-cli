/**
 * Component: Scout Session Manager
 * Block-UUID: ea59c662-ac0d-444a-91fe-8734f3afb6b6
 * Parent-UUID: dc5b321f-4537-4ff5-8082-1c0db52b2400
 * Version: 1.16.0
 * Description: Orchestrates Scout discovery and verification phases. Refactored to focus on session lifecycle and orchestration; subprocess management moved to subprocess.go, stream processing moved to stream.go. Fixed to set phase in writeNoBrainsError based on current turn. Updated LoadSession to populate WorkingDirectories and ReferenceFilesContext from StatusData. Removed GetFinalizedTurnResults() function as results are now stored in session.json. Updated GenerateStatusData() to read candidates from session state. Added lastAssistantMessage field to track assistant messages for post-processing. Updated comments to reflect turn-type based approach instead of turn numbers. Fixed critical issues with hardcoded turn numbers - now uses dynamic turn calculation to support multiple discovery turns.
 * Language: Go
 * Created-at: 2026-04-08T16:55:49.327Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4), GLM-4.7 (v1.2.5), GLM-4.7 (v1.2.6), GLM-4.7 (v1.2.7), GLM-4.7 (v1.2.8), GLM-4.7 (v1.2.9), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.3.3), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1), claude-haiku-4-5-20251001 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.5.2), GLM-4.7 (v1.5.3), GLM-4.7 (v1.5.4), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FinalizedTurnResults represents the lightweight results for a completed turn
// For verification turns, Candidates contains only verified/relevant candidates (score > 0.0)
// OriginalCandidates contains all discovery results for comparison
type FinalizedTurnResults struct {
	SessionID          string      `json:"session_id"`
	Turn               int         `json:"turn"`
	Status             string      `json:"status"`
	Candidates         []Candidate `json:"candidates"`
	OriginalCandidates []Candidate `json:"original_candidates,omitempty"`
	TotalFound         int         `json:"total_found"`
	TotalDiscovered    int         `json:"total_discovered,omitempty"`
}

// ErrTurnNotComplete is returned when a turn has not yet completed
var ErrTurnNotComplete = fmt.Errorf("turn has not yet completed")

// Manager orchestrates a scout session
type Manager struct {
	config      *SessionConfig
	debugLogger *DebugLogger
	session     *Session
	processor   *ProcessorHelper
	eventWriter *EventWriter
	currentTurn int
	processInfo *ProcessInfo
	wg          sync.WaitGroup
	loggerMu    sync.Mutex
	loggerClosed bool
	lastAssistantMessage string // Stores the last assistant message for post-processing
}

// NewManager creates a new scout manager
func NewManager(sessionID string) (*Manager, error) {
	config, err := NewSessionConfig(sessionID)
	if err != nil {
		return nil, err
	}

	// Create debug logger (disabled by default)
	debugLogger, err := NewDebugLogger(config.GetSessionDir(), false)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:      config,
		processor:   NewProcessorHelper(config),
		debugLogger: debugLogger,
		currentTurn: 1,
	}, nil
}

// NewManagerWithDebug creates a new scout manager with debug logging enabled
func NewManagerWithDebug(sessionID string, debugEnabled bool) (*Manager, error) {
	config, err := NewSessionConfig(sessionID)
	if err != nil {
		return nil, err
	}

	// Ensure session directory exists before creating debug log
	sessionDir := config.GetSessionDir()
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create debug logger with specified enabled state
	debugLogger, err := NewDebugLogger(sessionDir, debugEnabled)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:      config,
		processor:   NewProcessorHelper(config),
		debugLogger: debugLogger,
		currentTurn: 1,
	}, nil
}

// GetConfig returns the session configuration
func (m *Manager) GetConfig() *SessionConfig {
	return m.config
}

// InitializeSession sets up the session directory and writes initial state
func (m *Manager) InitializeSession(intent string, workdirs []WorkingDirectory, refFilesContext []ReferenceFileContext, autoReview bool, model string) error {
	m.debugLogger.Log("DEBUG", "Initializing session")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent: %s", m.truncateForLog(intent, 100)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directories: %d", len(workdirs)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Reference files: %d", len(refFilesContext)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Auto review: %v", autoReview))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Model: %s", model))

	// Validate inputs
	if err := ValidateIntent(intent); err != nil {
		m.debugLogger.LogError("Intent validation failed", err)
		return err
	}

	// Initialize directories first (so we have a place to write session.json)
	if err := m.config.InitializeSessionDirs(); err != nil {
		m.debugLogger.LogError("Failed to initialize session directories", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Session directories initialized")

	// Create session struct with common state information
	m.session = &Session{
		SessionDir: 		   m.config.GetSessionDir(),
		SessionID:             m.config.SessionID,
		Intent:                intent,
		Model:                 model,
		WorkingDirectories:    workdirs,
		ReferenceFilesContext: refFilesContext,
		AutoReview:            autoReview,
		Status:                "discovery",  // Default status
		StartedAt:             time.Now(),
		Turns:                 []TurnState{},
	}
	m.debugLogger.Log("DEBUG", "Session struct created")

	// Validate setup
	errs, _ := ValidateSetup(workdirs, refFilesContext)
	if len(errs) > 0 {
		// Build detailed error message
		var errorDetails []string
		for _, e := range errs {
			errorDetails = append(errorDetails, fmt.Sprintf("  - %s: %s", e.Type, e.Message))
		}
		errMsg := fmt.Sprintf("validation failed with %d error(s):\n%s", len(errs), strings.Join(errorDetails, "\n"))
		m.debugLogger.LogError("Setup validation failed", fmt.Errorf(errMsg))

		// Update only error-specific fields
		m.session.Status = "error"
		m.session.Error = &errMsg
		completedAt := time.Now()
		m.session.CompletedAt = &completedAt
		m.debugLogger.Log("DEBUG", "Session updated with error status")

		// Write session state with error
		if err := m.WriteSessionState(); err != nil {
			m.debugLogger.LogError("Failed to write session state", err)
			return fmt.Errorf("%s (also failed to write session state: %w)", errMsg, err)
		}
		m.debugLogger.Log("DEBUG", "Session state written with error")

		// Return the validation error
		return fmt.Errorf(errMsg)
	}
	m.debugLogger.Log("DEBUG", "Validation passed")

	// Write intent to turn-1/intent.md
	intentPath := m.config.GetIntentFile(1)
	if err := os.WriteFile(intentPath, []byte(intent), 0644); err != nil {
		m.debugLogger.LogError("Failed to write intent file", err)
		return fmt.Errorf("failed to write intent file: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent written to: %s", intentPath))

	// Write session state for successful initialization
	return m.WriteSessionState()
}

// PrepareTurn1 generates codebase overview and handles no-brains case
func (m *Manager) PrepareTurn1() error {
	m.debugLogger.Log("DEBUG", "Preparing discovery turn")

	// 1. Build codebase overview
	overview, err := BuildCodebaseOverview(m.session.SessionID, m.session.WorkingDirectories)
	if err != nil {
		m.debugLogger.LogError("Failed to build codebase overview", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Codebase overview built successfully")

	// 2. Write codebase overview to file
	overviewPath := m.config.GetCodebaseOverviewFile()
	overviewJSON, err := json.MarshalIndent(overview, "", "  ")
	if err != nil {
		m.debugLogger.LogError("Failed to marshal codebase overview", err)
		return fmt.Errorf("failed to marshal codebase overview: %w", err)
	}
	if err := os.WriteFile(overviewPath, overviewJSON, 0644); err != nil {
		m.debugLogger.LogError("Failed to write codebase overview", err)
		return fmt.Errorf("failed to write codebase overview: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Codebase overview written to: %s", overviewPath))

	// 3. Check if all brains unavailable
	if checkAllBrainsUnavailable(overview.WorkingDirectories) {
		m.debugLogger.Log("DEBUG", "All brains unavailable, writing error event")
		// Write error event to log
		if err := m.writeNoBrainsError(); err != nil {
			return fmt.Errorf("failed to write no-brains error: %w", err)
		}

		// Update session status
		m.session.Status = "error"
		errMsg := "NO_BRAINS_AVAILABLE: No brains available in any working directory"
		m.session.Error = &errMsg
		completedAt := time.Now()
		m.session.CompletedAt = &completedAt

		// Write status
		return m.writeSessionState()
	}

	return nil
}

// writeNoBrainsError writes error event to log when no brains available
func (m *Manager) writeNoBrainsError() error {
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	writer, err := NewEventWriter(logPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create event writer for no-brains error", err)
		return err
	}
	defer writer.Close()

	// Set phase based on current turn type
	phase := "discovery"
	if len(m.session.Turns) > 0 {
		lastTurn := m.session.Turns[len(m.session.Turns)-1]
		if lastTurn.TurnType == "verification" {
			phase = "verification"
		}
	}

	return writer.WriteErrorEvent(ErrorEvent{
		Phase:     phase,
		ErrorCode: "NO_BRAINS_AVAILABLE",
		Message:   "No brains available in any working directory",
	})
}

// getNextTurnNumber calculates the next turn number dynamically
func (m *Manager) getNextTurnNumber() int {
	if len(m.session.Turns) == 0 {
		return 1
	}
	return m.session.Turns[len(m.session.Turns)-1].TurnNumber + 1
}

// StartDiscoveryTurn initiates the discovery phase and spawns subprocess
func (m *Manager) StartDiscoveryTurn() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting discovery turn")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directories: %d", len(m.session.WorkingDirectories)))

	if m.session.Status != "discovery" && m.session.Status != "error" {
		return fmt.Errorf("cannot start discovery: session status is %s", m.session.Status)
	}

	// Prepare discovery turn (generate input schema)
	if err := m.PrepareTurn1(); err != nil {
		m.debugLogger.LogError("PrepareTurn1 failed", err)
		return err
	}

	// Check if session already errored out (no brains)
	if m.session.Status == "error" {
		m.debugLogger.Log("DEBUG", "Session already in error state, not spawning subprocess")
		return nil // Don't spawn Claude, already handled
	}

	// Calculate next turn number dynamically
	nextTurn := m.getNextTurnNumber()
	m.currentTurn = nextTurn
	m.session.Status = "discovery"

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
		m.markAsStopped("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
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

	// Spawn subprocess for discovery turn (defined in subprocess.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn, "discovery"); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsStopped("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", "Subprocess spawned successfully")

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// StartVerificationTurn initiates the verification phase
func (m *Manager) StartVerificationTurn(selectedCandidates *SelectedCandidates) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting verification turn")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))

	if m.session.Status != "discovery_complete" {
		return fmt.Errorf("cannot start verification: discovery not complete")
	}

	// Calculate next turn number dynamically
	nextTurn := m.getNextTurnNumber()
	m.currentTurn = nextTurn
	m.session.Status = "verification"

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
		m.markAsStopped("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Created event writer: %s", logPath))

	// Mark last turn as complete
	if len(m.session.Turns) > 0 {
		lastTurnIndex := len(m.session.Turns) - 1
		m.session.Turns[lastTurnIndex].Status = "complete"
		m.session.Turns[lastTurnIndex].CompletedAt = &[]time.Time{time.Now()}[0]
		m.session.Turns[lastTurnIndex].ProcessInfo.Running = false
	}
	
	// Create new turn state and append to turns slice
	newTurn := TurnState{
		TurnNumber:  nextTurn,
		TurnType:    "verification",
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

	// Save selected candidates for Claude to reference
	if selectedCandidates != nil {
		candData, _ := json.MarshalIndent(selectedCandidates, "", "  ")
		candPath := filepath.Join(m.config.GetTurnDir(m.currentTurn), "selected-candidates.json")
		if err := os.WriteFile(candPath, candData, 0644); err != nil {
			m.debugLogger.LogError("Failed to save selected candidates", err)
			m.markAsStopped("CANDIDATE_SAVE_FAILED", fmt.Sprintf("Failed to save selected candidates: %v", err))
			return err
		}
	}

	// Spawn subprocess for verification turn (defined in subprocess.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn, "verification"); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsStopped("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", "Subprocess spawned successfully")

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// GetSessionStatus reconstructs the current session status
func (m *Manager) GetSessionStatus() (*StatusData, error) {
	// Read session.json (single source of truth)
	session, err := m.processor.ReadSession(m.config.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	// Generate StatusData from Session for display
	return m.processor.GenerateStatusData(session, m.currentTurn)
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

// MarkDiscoveryComplete transitions to discovery_complete state
func (m *Manager) MarkDiscoveryComplete() error {
	if m.session.Status != "discovery" {
		return fmt.Errorf("cannot mark complete: not in discovery state, current status: %s", m.session.Status)
	}

	m.session.Status = "discovery_complete"
	return m.writeSessionState()
}

// MarkVerificationComplete transitions to verification_complete state
func (m *Manager) MarkVerificationComplete() error {
	if m.session.Status != "verification" {
		return fmt.Errorf("cannot mark complete: not in verification state, current status: %s", m.session.Status)
	}

	m.session.Status = "verification_complete"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]
	return m.writeSessionState()
}

// CloseEventWriter flushes and closes the current event writer
func (m *Manager) CloseEventWriter() error {
	if m.eventWriter != nil {
		m.debugLogger.Log("DEBUG", "Closing event writer")
		return m.eventWriter.Close()
	}
	return nil
}

// WriteSessionState writes the session state to disk (public method)
func (m *Manager) WriteSessionState() error {
	return m.writeSessionState()
}

// writeSessionState writes the session state to disk
func (m *Manager) writeSessionState() error {
	sessionPath := m.config.GetSessionFile()

	// Marshal the complete session state directly
	data, err := json.MarshalIndent(m.session, "", "  ")
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

	return mgr, nil
}

// truncateForLog truncates a string for logging purposes
func (m *Manager) truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// closeDebugLogger safely closes the debug logger (idempotent)
func (m *Manager) closeDebugLogger() {
	m.loggerMu.Lock()
	defer m.loggerMu.Unlock()

	if m.loggerClosed {
		m.debugLogger.Log("DEBUG", "Debug logger already closed, skipping")
		return
	}

	m.debugLogger.Log("DEBUG", "Closing debug logger")
	m.loggerClosed = true

	// Wait for all goroutines to finish before closing
	// This ensures all pending log messages are written
	go func() {
		m.wg.Wait()
		m.debugLogger.Log("DEBUG", "All goroutines finished, closing logger file")
		m.debugLogger.Close()
	}()
}

// writeTurnHistory writes the turn-history.json file
func (m *Manager) writeTurnHistory(turnNumber int) error {
	if m.session == nil {
		return nil
	}
	
	// Find the turn
	var turn *TurnState
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == turnNumber {
			turn = &m.session.Turns[i]
			break
		}
	}
	
	if turn == nil {
		return fmt.Errorf("turn %d not found", turnNumber)
	}
	
	// Build turn history entry
	historyEntry := map[string]interface{}{
		"turn_number": turn.TurnNumber,
		"turn_type":   turn.TurnType,
		"intent":      m.session.Intent,
		"started_at":  turn.StartedAt.Format(time.RFC3339),
	}
	
	if turn.CompletedAt != nil {
		historyEntry["completed_at"] = turn.CompletedAt.Format(time.RFC3339)
	}
	
	if turn.Error != nil {
		historyEntry["error"] = *turn.Error
	}
	
	// Add results if available
	if turn.Results != nil {
		historyEntry["candidates"] = turn.Results.Candidates
		
		if turn.Results.DiscoveryLog != nil {
			historyEntry["discovery_log"] = turn.Results.DiscoveryLog
		}
		
		if turn.Results.VerificationSummary != nil {
			historyEntry["verification_summary"] = turn.Results.VerificationSummary
		}
	}
	
	// Read existing history
	var history []map[string]interface{}
	historyPath := filepath.Join(m.config.GetSessionDir(), "turn-history.json")
	
	if data, err := os.ReadFile(historyPath); err == nil {
		if err := json.Unmarshal(data, &history); err != nil {
			return fmt.Errorf("failed to parse turn-history.json: %w", err)
		}
	}
	
	// Add or update this turn
	found := false
	for i, entry := range history {
		if entry["turn_number"] == turnNumber {
			history[i] = historyEntry
			found = true
			break
		}
	}
	
	if !found {
		history = append(history, historyEntry)
	}
	
	// Write back
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal turn-history.json: %w", err)
	}
	
	return os.WriteFile(historyPath, data, 0644)
}
