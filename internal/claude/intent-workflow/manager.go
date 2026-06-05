/**
 * Component: Intent Workflow Session Manager
 * Block-UUID: a3345746-4e4c-4127-87b3-80d856a0db8c
 * Parent-UUID: f6077751-e2cf-4e32-9fe8-aff9ef1859be
 * Version: 1.49.0
 * Description: Core session manager struct and initialization logic. Refactored to delegate persistence, lifecycle, turn orchestration, resume logic, and prompt generation to specialized files. Removed fatal error path for missing brains - now supports hybrid discovery strategy with experts/generic modes. Added disableExperts parameter to InitializeSession to support --disable-experts flag for forcing generic discovery mode.
 * Language: Go
 * Created-at: 2026-04-28T13:47:04.136Z
 * Authors: ..., GLM-4.7 (v1.44.0), GLM-4.7 (v1.45.0), GLM-4.7 (v1.46.0), GLM-4.7 (v1.47.0), GLM-4.7 (v1.47.1), GLM-4.7 (v1.48.0), GLM-4.7 (v1.49.0)
 */


package intent_workflow

import (
	"encoding/json"
	"strings"
	"fmt"
	"os"
	"sync"
	"time"
)

// FinalizedTurnResults represents the lightweight results for a completed turn
// For discovery turns, Candidates contains only validated candidates (score > 0.7)
type FinalizedTurnResults struct {
	SessionID          string      `json:"session_id"`
	Turn               int         `json:"turn"`
	Status             string      `json:"status"`
	Candidates         []Candidate `json:"candidates"`
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
func (m *Manager) InitializeSession(intent string, workdirs []WorkingDirectory, refFilesContext []ReferenceFileContext, autoReview bool, model string, disableExperts bool) error {
	m.debugLogger.Log("DEBUG", "Initializing session")
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Intent: %s", m.truncateForLog(intent, 100)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directories: %d", len(workdirs)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Reference files: %d", len(refFilesContext)))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Auto review: %v", autoReview))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Model: %s", model))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Disable experts: %v", disableExperts))

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
		DisableExperts:        disableExperts,
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
		if err := m.writeSessionState(); err != nil {
			m.debugLogger.LogError("Failed to write session state", err)
			return fmt.Errorf("%s (also failed to write session state: %w)", errMsg, err)
		}
		m.debugLogger.Log("DEBUG", "Session state written with error")

		// Return the validation error
		return fmt.Errorf(errMsg)
	}
	m.debugLogger.Log("DEBUG", "Validation passed")

	// Write session state for successful initialization
	return m.writeSessionState()
}

// PrepareContext generates context and handles no-context case
func (m *Manager) PrepareContext() error {
	m.debugLogger.Log("DEBUG", "Preparing discovery turn")

	// 1. Build context
	overview, err := BuildCodebaseOverview(m.session.SessionID, m.session.WorkingDirectories, m.session.DisableExperts)
	if err != nil {
		m.debugLogger.LogError("Failed to build context", err)
		return err
	}
	m.debugLogger.Log("DEBUG", "Context built successfully")

	// 2. Write context to file
	overviewPath := m.config.GetCodebaseOverviewFile()
	overviewJSON, err := json.MarshalIndent(overview, "", "  ")
	if err != nil {
		m.debugLogger.LogError("Failed to marshal context", err)
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	if err := os.WriteFile(overviewPath, overviewJSON, 0644); err != nil {
		m.debugLogger.LogError("Failed to write context", err)
		return fmt.Errorf("failed to write context: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Context written to: %s", overviewPath))

	// 3. Continue regardless of experts availability
	// The discovery turn will use the appropriate mode (experts or generic)
	// based on the context file contents
	m.debugLogger.Log("DEBUG", "Context preparation complete")

	return nil
}

// GetSession returns the current session state
func (m *Manager) GetSession() *Session {
	return m.session
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

// CloseEventWriter flushes and closes the current event writer
func (m *Manager) CloseEventWriter() error {
	if m.eventWriter != nil {
		m.debugLogger.Log("DEBUG", "Closing event writer")
		return m.eventWriter.Close()
	}
	return nil
}

// updateSessionWithCorrectedResults replaces the malformed turn results with
// the corrected results produced by a correction turn. It rebuilds quick
// candidates, stores the full corrected TurnResults, records correction
// metadata, aggregates cost, and persists the updated session state.
func (m *Manager) updateSessionWithCorrectedResults(turnNumber int, correctedResults *TurnResult, correctionCost float64) error {
	turnState := m.getTurnState(turnNumber)
	if turnState == nil {
		return fmt.Errorf("turn %d not found in session", turnNumber)
	}

	// Store the corrected results
	turnState.Result = correctedResults

	// Record correction outcome.
	turnState.CorrectionStatus = CorrectionStatusSuccess
	turnState.CorrectionCost = &correctionCost
	if turnState.Cost != nil {
		total := *turnState.Cost + correctionCost
		turnState.TotalCost = &total
	}

	m.session.Status = "discovery_complete"
	return m.writeSessionState()
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
