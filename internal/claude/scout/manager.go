/**
 * Component: Scout Session Manager
 * Block-UUID: 1405c45c-4328-46a9-b885-6e7db4749bc6
 * Parent-UUID: bbaa58c4-10a2-4fde-a8c1-a89de7fff930
 * Version: 1.5.4
 * Description: Orchestrates Scout discovery and verification phases. Refactored to focus on session lifecycle and orchestration; subprocess management moved to subprocess.go, stream processing moved to stream.go. Fixed to set phase in writeNoBrainsError based on current turn. Updated LoadSession to populate WorkingDirectories and ReferenceFilesContext from StatusData.
 * Language: Go
 * Created-at: 2026-04-05T14:49:42.685Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4), GLM-4.7 (v1.2.5), GLM-4.7 (v1.2.6), GLM-4.7 (v1.2.7), GLM-4.7 (v1.2.8), GLM-4.7 (v1.2.9), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.3.3), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1), claude-haiku-4-5-20251001 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.5.2), GLM-4.7 (v1.5.3), GLM-4.7 (v1.5.4)
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
// For Turn 2, Candidates contains only verified/relevant candidates (score > 0.0)
// OriginalCandidates contains all Turn 1 discovery results for comparison
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

	if errs, _ := ValidateSetup(workdirs, refFilesContext); len(errs) > 0 {
		// Build detailed error message
		var errorDetails []string
		for _, e := range errs {
			errorDetails = append(errorDetails, fmt.Sprintf("  - %s: %s", e.Type, e.Message))
		}
		errMsg := fmt.Sprintf("validation failed with %d error(s):\n%s", len(errs), strings.Join(errorDetails, "\n"))
		m.debugLogger.LogError("Setup validation failed", fmt.Errorf(errMsg))
		return fmt.Errorf(errMsg)
	}
	m.debugLogger.Log("DEBUG", "Validation passed")

	// Initialize directories
	if err := m.config.InitializeSessionDirs(); err != nil {
		m.debugLogger.LogError("Failed to initialize session directories", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Session directories initialized")

	// Write intent to turn-1/intent.md
	intentPath := m.config.GetIntentFile(1)
	if err := os.WriteFile(intentPath, []byte(intent), 0644); err != nil {
		m.debugLogger.LogError("Failed to write intent file", err)
		return fmt.Errorf("failed to write intent file: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent written to: %s", intentPath))

	// Create session struct
	m.session = &Session{
		SessionID:             m.config.SessionID,
		Intent:                intent,
		Model:                 model,
		WorkingDirectories:    workdirs,
		ReferenceFilesContext: refFilesContext,
		AutoReview:            autoReview,
		Status:                "discovery",
		StartedAt:             time.Now(),
	}
	m.debugLogger.Log("DEBUG", "Session struct created")

	// Reference files from NDJSON already have content embedded, no need to copy

	return m.writeSessionState()
}

// PrepareTurn1 generates codebase overview and handles no-brains case
func (m *Manager) PrepareTurn1() error {
	m.debugLogger.Log("DEBUG", "Preparing Turn 1")

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
	logPath := m.config.GetTurnLogFile(1, logFilename)

	writer, err := NewEventWriter(logPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create event writer for no-brains error", err)
		return err
	}
	defer writer.Close()

	// Set phase based on current turn
	phase := "discovery"
	if m.currentTurn == 2 {
		phase = "verification"
	}

	return writer.WriteErrorEvent(ErrorEvent{
		Phase:     phase,
		ErrorCode: "NO_BRAINS_AVAILABLE",
		Message:   "No brains available in any working directory",
	})
}

// StartTurn1Discovery initiates the discovery phase and spawns subprocess
func (m *Manager) StartTurn1Discovery() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting Turn 1 discovery")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directories: %d", len(m.session.WorkingDirectories)))

	if m.session.Status != "discovery" && m.session.Status != "error" {
		return fmt.Errorf("cannot start discovery: session status is %s", m.session.Status)
	}

	// Prepare Turn 1 (generate input schema)
	if err := m.PrepareTurn1(); err != nil {
		m.debugLogger.LogError("PrepareTurn1 failed", err)
		return err
	}

	// Check if session already errored out (no brains)
	if m.session.Status == "error" {
		m.debugLogger.Log("DEBUG", "Session already in error state, not spawning subprocess")
		return nil // Don't spawn Claude, already handled
	}

	m.currentTurn = 1
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

	// Store log paths in session
	m.session.Turn1LogPath = logPath
	m.session.CurrentLogPath = logPath
	
	// Write session state to persist log paths
	if err := m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Log paths stored in session state")

	// Write init event
	initEvent := InitEvent{
		SessionID:             m.session.SessionID,
		Intent:                m.session.Intent,
		WorkingDirectories:    m.session.WorkingDirectories,
		ReferenceFilesContext: m.session.ReferenceFilesContext,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
			Model:      m.session.Model,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		m.debugLogger.LogEventWrite("init", false, err)
		return err
	}
	m.debugLogger.LogEventWrite("init", true, nil)

	// Spawn subprocess for Turn 1 (defined in subprocess.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsStopped("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}
	m.debugLogger.Log("DEBUG", "Subprocess spawned successfully")

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// StartTurn2Verification initiates the verification phase
func (m *Manager) StartTurn2Verification(selectedCandidates *SelectedCandidates) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", "Starting Turn 2 verification")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Session status: %s", m.session.Status))

	if m.session.Status != "discovery_complete" {
		return fmt.Errorf("cannot start verification: discovery not complete")
	}

	m.currentTurn = 2
	m.session.Status = "verification"

	// Close previous eventWriter if it exists to prevent resource leaks
	if m.eventWriter != nil {
		m.eventWriter.Close()
	}

	// Create log file for Turn 2
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

	// Store log paths in session
	m.session.Turn2LogPath = logPath
	m.session.CurrentLogPath = logPath
	
	// Write session state to persist log paths
	if err := m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Log paths stored in session state")

	// Write init event with selected candidates
	initEvent := InitEvent{
		SessionID:             m.session.SessionID,
		Intent:                m.session.Intent,
		WorkingDirectories:    m.session.WorkingDirectories,
		ReferenceFilesContext: m.session.ReferenceFilesContext,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
			Model:      m.session.Model,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		m.debugLogger.LogEventWrite("init", false, err)
		return err
	}
	m.debugLogger.LogEventWrite("init", true, nil)

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

	// Spawn subprocess for Turn 2 (defined in subprocess.go)
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
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
	if m.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}

	// Read status from latest turn's log file
	status, err := m.processor.ReadSessionStatusFromEvents(m.currentTurn)
	if err != nil {
		m.debugLogger.LogError("Failed to read status from events", err)
		// If no events yet, construct minimal status with safe ProcessInfo
		processInfo := ProcessInfo{}
		if m.processInfo != nil {
			processInfo = *m.processInfo
		}

		status = &StatusData{
			SessionID:          m.session.SessionID,
			Status:             m.session.Status,
			Phase:              "discovery",
			StartedAt:          m.session.StartedAt,
			WorkingDirectories: m.session.WorkingDirectories,
			ReferenceFilesContext: m.session.ReferenceFilesContext,
			Candidates:         []Candidate{},
			ProcessInfo:        processInfo,
		}
	}

	return status, nil
}

// GetFinalizedTurnResults retrieves the finalized results for a specific turn
// For Turn 2, merges verification updates with original candidates and filters out irrelevant ones
// Returns ErrTurnNotComplete if the turn has not yet finished
func (m *Manager) GetFinalizedTurnResults(turn int) (*FinalizedTurnResults, error) {
	if turn != 1 && turn != 2 {
		return nil, fmt.Errorf("turn must be 1 or 2")
	}

	// Check if the session exists
	if !m.config.SessionExists() {
		return nil, fmt.Errorf("session does not exist: %s", m.config.SessionID)
	}

	// Get the latest log file for the requested turn
	logFile, err := m.processor.GetLatestTurnLogFile(turn)
	if err != nil {
		m.debugLogger.LogError("Failed to get latest turn log file", err)
		return nil, fmt.Errorf("no results found for turn %d", turn)
	}

	// Read events from the log file
	reader, err := NewEventReader(logFile)
	if err != nil {
		m.debugLogger.LogError("Failed to create event reader", err)
		return nil, fmt.Errorf("failed to read turn results: %w", err)
	}
	defer reader.Close()

	events, err := reader.ReadAllEvents()
	if err != nil {
		m.debugLogger.LogError("Failed to read all events", err)
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	if len(events) == 0 {
		m.debugLogger.Log("DEBUG", fmt.Sprintf("No events found for turn %d", turn))
		return nil, fmt.Errorf("no events found for turn %d", turn)
	}

	// Find the candidates, verified, and done events
	var candidates []Candidate
	var verificationUpdates []VerificationUpdate
	var totalFound int
	var statusValue string
	var foundDone bool

	for _, event := range events {
		switch event.Type {
		case "candidates":
			// Parse candidates event (Turn 1)
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue // Skip malformed events
			}
			var candEvent CandidatesEvent
			if err := json.Unmarshal(data, &candEvent); err == nil {
				candidates = candEvent.Candidates
				totalFound = candEvent.TotalFound
			}

		case "verified":
			// Parse verified event (Turn 2)
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue // Skip malformed events
			}
			var verEvent VerifiedEvent
			if err := json.Unmarshal(data, &verEvent); err == nil {
				verificationUpdates = verEvent.UpdatedCandidates
				totalFound = verEvent.TotalVerified
			}

		case "done":
			// Found the completion marker
			foundDone = true
		}
	}

	// Verify that the turn actually completed
	if !foundDone {
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Turn %d not complete (no done event)", turn))
		return nil, ErrTurnNotComplete
	}

	// Determine the status based on turn
	if turn == 1 {
		statusValue = "discovery_complete"
	} else {
		statusValue = "verification_complete"
	}

	results := &FinalizedTurnResults{
		SessionID:  m.config.SessionID,
		Turn:       turn,
		Status:     statusValue,
		Candidates: candidates,
		TotalFound: totalFound,
	}

	// For Turn 2, merge verification updates with original candidates
	if turn == 2 {
		// Read Turn 1 candidates
		originalLogFile, err := m.processor.GetLatestTurnLogFile(1)
		if err != nil {
			m.debugLogger.LogError("Failed to read Turn 1 log file", err)
			return nil, fmt.Errorf("failed to read Turn 1 log file: %w", err)
		}

		reader, err := NewEventReader(originalLogFile)
		if err != nil {
			m.debugLogger.LogError("Failed to open Turn 1 log file", err)
			return nil, fmt.Errorf("failed to open Turn 1 log file: %w", err)
		}
		defer reader.Close()

		originalEvents, err := reader.ReadAllEvents()
		if err != nil {
			m.debugLogger.LogError("Failed to read Turn 1 events", err)
			return nil, fmt.Errorf("failed to read Turn 1 events: %w", err)
		}

		var originalCandidates []Candidate
		for _, event := range originalEvents {
			if event.Type == "candidates" {
				data, err := json.Marshal(event.Data)
				if err != nil {
					continue // Skip malformed events
				}
				var candEvent CandidatesEvent
				if err := json.Unmarshal(data, &candEvent); err == nil {
					originalCandidates = candEvent.Candidates
					break
				}
			}
		}

		// Merge verification updates with original candidates
		verifiedCandidates := make([]Candidate, 0, len(originalCandidates))
		for _, orig := range originalCandidates {
			updated := orig
			// Find matching verification update
			for _, update := range verificationUpdates {
				if update.FilePath == orig.FilePath && update.WorkdirID == orig.WorkdirID {
					updated.Score = update.VerifiedScore
					updated.Reasoning = update.Reason
					break
				}
			}
			// Only include candidates with verified score > 0.0
			if updated.Score > 0.0 {
				verifiedCandidates = append(verifiedCandidates, updated)
			}
		}

		results.Candidates = verifiedCandidates
		results.OriginalCandidates = originalCandidates
		results.TotalFound = len(verifiedCandidates)
		results.TotalDiscovered = len(originalCandidates)
	}

	return results, nil
}

// GetLastCompletedTurn returns the highest turn number that has completed successfully
// Returns 0 if no turn has completed (new session)
func (m *Manager) GetLastCompletedTurn() (int, error) {
	if m.session == nil {
		// Load status to check completion
		status, err := m.GetSessionStatus()
		if err != nil {
			return 0, err
		}
		if status == nil {
			return 0, nil
		}

		// Check what turn is currently referenced
		if status.Phase == "discovery" && (status.Status == "discovery_complete" || status.Status == "discovery_in_progress") {
			return 1, nil
		}
		if status.Phase == "verification" {
			return 2, nil
		}
		return 0, nil
	}

	// Based on session status, determine completed turn
	switch m.session.Status {
	case "discovery_complete", "verification", "verification_complete":
		return 1, nil
	case "stopped", "error":
		// Check log file to see which turn actually completed
		lastLogFile, lastTurn, err := m.processor.GetLatestLogFile()
		if err == nil && lastLogFile != "" {
			reader, err := NewEventReader(lastLogFile)
			if err != nil {
				// Log error but continue - if we can't read the log file,
				// we can't determine if the turn completed
				return 0, nil
			}
			if reader != nil {
				defer reader.Close()
				events, _ := reader.ReadAllEvents()
				for _, event := range events {
					if event.Type == "done" {
						return lastTurn, nil
					}
				}
			}
		}
		return 0, nil
	default:
		return 0, nil
	}
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
	statusPath := m.config.GetStatusFile()

	// Get full status data including candidates, process info, etc.
	status, err := m.GetSessionStatus()
	if err != nil {
		m.debugLogger.LogError("Failed to get session status for write", err)
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		m.debugLogger.LogError("Failed to marshal session status", err)
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		m.debugLogger.LogError("Failed to write status file", err)
		return fmt.Errorf("failed to write status file: %w", err)
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

	statusPath := config.GetStatusFile()
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read status file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
	}

	// Reconstruct Session from StatusData
	// Note: status.json contains StatusData, not Session
	var statusData StatusData
	if err := json.Unmarshal(data, &statusData); err != nil {
		return nil, fmt.Errorf("failed to parse status data: %w", err)
	}

	// Reconstruct Session from StatusData
	session = Session{
		SessionID:             statusData.SessionID,
		Status:                statusData.Status,
		StartedAt:             statusData.StartedAt,
		CompletedAt:           statusData.CompletedAt,
		Error:                 statusData.Error,
		WorkingDirectories:    statusData.WorkingDirectories,
		ReferenceFilesContext: statusData.ReferenceFilesContext,
		// Intent, AutoReview, Model are not in StatusData
		// These are only available during initial session creation
	}

	// Infer currentTurn from session status
	currentTurn := 1
	if session.Status == "verification" || session.Status == "verification_complete" {
		currentTurn = 2
	}

	mgr := &Manager{
		config:      config,
		session:     &session,
		processor:   NewProcessorHelper(config),
		debugLogger: debugLogger,
		currentTurn: currentTurn,
	}

	// Initialize ProcessInfo from loaded status
	mgr.processInfo = &statusData.ProcessInfo

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
