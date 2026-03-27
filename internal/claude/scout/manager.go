/**
 * Component: Scout Session Manager
 * Block-UUID: a3839915-fa21-4f3f-a7b4-4451ce013278
 * Parent-UUID: e815e9d4-df92-4d1f-a99e-e50fa0ff206f
 * Version: 1.0.3
 * Description: Orchestrates Scout discovery and verification phases, manages subprocess execution
 * Language: Go
 * Created-at: 2026-03-27T16:15:50.025Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gs-devtools/gsc/pkg/settings"
)

// Manager orchestrates a scout session
type Manager struct {
	config            *SessionConfig
	session           *Session
	processor         *ProcessorHelper
	eventWriter       *EventWriter
	currentTurn       int
	processInfo       *ProcessInfo
}

// NewManager creates a new scout manager
func NewManager(sessionID string) (*Manager, error) {
	config, err := NewSessionConfig(sessionID)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:    config,
		processor: NewProcessorHelper(config),
		currentTurn: 1,
	}, nil
}

// InitializeSession sets up the session directory and writes initial state
func (m *Manager) InitializeSession(intent string, workdirs []WorkingDirectory, refFiles []ReferenceFile, autoReview bool) error {
	// Validate inputs
	if err := ValidateIntent(intent); err != nil {
		return err
	}

	if errs, _ := ValidateSetup(workdirs, refFiles); len(errs) > 0 {
		errMsg := fmt.Sprintf("validation failed: %d errors", len(errs))
		return fmt.Errorf(errMsg)
	}

	// Initialize directories
	if err := m.config.InitializeSessionDirs(); err != nil {
		return err
	}

	// Write intent to turn-1/intent.md
	intentPath := m.config.GetIntentFile(1)
	if err := os.WriteFile(intentPath, []byte(intent), 0644); err != nil {
		return fmt.Errorf("failed to write intent file: %w", err)
	}

	// Create session struct
	m.session = &Session{
		SessionID:          m.config.SessionID,
		Intent:             intent,
		WorkingDirectories: workdirs,
		ReferenceFiles:     refFiles,
		AutoReview:         autoReview,
		Status:             "discovery",
		StartedAt:          time.Now(),
	}

	// Copy reference files
	for i, rf := range refFiles {
		refType := fmt.Sprintf("reference-%d", i)
		if err := m.processor.CopyReferenceFile(rf.OriginalPath, refType); err != nil {
			return fmt.Errorf("failed to copy reference file: %w", err)
		}
	}

	return m.writeSessionState()
}

// StartTurn1Discovery initiates the discovery phase and spawns subprocess
func (m *Manager) StartTurn1Discovery() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.currentTurn = 1
	m.session.Status = "discovery"

	// Create log file for this turn
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	var err error
	m.eventWriter, err = NewEventWriter(logPath)
	if err != nil {
		return err
	}

	// Write init event
	initEvent := InitEvent{
		SessionID:          m.session.SessionID,
		Intent:             m.session.Intent,
		WorkingDirectories: m.session.WorkingDirectories,
		ReferenceFiles:     m.session.ReferenceFiles,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		return err
	}

	// Spawn subprocess for Turn 1
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
		return err
	}

	return m.writeSessionState()
}

// StartTurn2Verification initiates the verification phase
func (m *Manager) StartTurn2Verification(selectedCandidates *SelectedCandidates) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	if m.session.Status != "discovery_complete" {
		return fmt.Errorf("cannot start verification: discovery not complete")
	}

	m.currentTurn = 2
	m.session.Status = "verification"

	// Create log file for Turn 2
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	var err error
	m.eventWriter, err = NewEventWriter(logPath)
	if err != nil {
		return err
	}

	// Write init event with selected candidates
	initEvent := InitEvent{
		SessionID:          m.session.SessionID,
		Intent:             m.session.Intent,
		WorkingDirectories: m.session.WorkingDirectories,
		ReferenceFiles:     m.session.ReferenceFiles,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		return err
	}

	// Save selected candidates for Claude to reference
	if selectedCandidates != nil {
		candData, _ := json.MarshalIndent(selectedCandidates, "", "  ")
		candPath := filepath.Join(m.config.GetTurnDir(m.currentTurn), "selected-candidates.json")
		os.WriteFile(candPath, candData, 0644)
	}

	// Spawn subprocess for Turn 2
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
		return err
	}

	return m.writeSessionState()
}

// spawnClaudeSubprocess spawns the claude subprocess for a turn
func (m *Manager) spawnClaudeSubprocess(turn int) error {
	// Write Scout permissions to restrict Bash to gsc commands only
	if err := WriteScoutPermissions(m.config.GetTurnDir(turn)); err != nil {
		return fmt.Errorf("failed to write permissions: %w", err)
	}

	// Get the Claude prompt template using absolute path
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	var templateName string
	if turn == 1 {
		templateName = "turn-1-discovery.md"
	} else {
		templateName = "turn-2-verification.md"
	}

	promptPath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", templateName)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("failed to read prompt template: %w", err)
	}

	// Build the command to invoke claude CLI directly
	flags := []string{
		"-p", fmt.Sprintf("%q", string(promptData)),
		"--allowedTools", "Read,Bash",
		"--verbose",
		"--include-partial-messages",
		"--output-format", "stream-json",
	}

	// Add working directories as --add-dir flags
	for _, wd := range m.session.WorkingDirectories {
		flags = append(flags, "--add-dir", wd.Path)
	}

	cmd := exec.Command("claude", flags...)
	cmd.Dir = m.config.GetTurnDir(turn)

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start subprocess: %w", err)
	}

	m.processInfo = &ProcessInfo{
		PID:     cmd.Process.Pid,
		Command: cmd.String(),
		Running: true,
	}

	// Don't wait for process - fire and forget
	// Caller can monitor progress via log file
	return nil
}

// GetSessionStatus reconstructs the current session status
func (m *Manager) GetSessionStatus() (*StatusData, error) {
	if m.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}

	// Read status from latest turn's log file
	status, err := m.processor.ReadSessionStatusFromEvents(m.currentTurn)
	if err != nil {
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
			Candidates:         []Candidate{},
			ProcessInfo:        processInfo,
		}
	}

	return status, nil
}

// CheckProcessStatus checks if the subprocess is still running
func (m *Manager) CheckProcessStatus() (bool, error) {
	if m.processInfo == nil {
		return false, fmt.Errorf("no process info available")
	}

	process, err := os.FindProcess(m.processInfo.PID)
	if err != nil {
		m.processInfo.Running = false
		return false, nil
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(os.Signal(nil)); err != nil {
		m.processInfo.Running = false
		return false, nil
	}

	m.processInfo.Running = true
	return true, nil
}

// StopSession stops the current scout session and cleanup
func (m *Manager) StopSession() error {
	if m.processInfo != nil && m.processInfo.Running {
		process, err := os.FindProcess(m.processInfo.PID)
		if err == nil {
			process.Kill()
		}
		m.processInfo.Running = false
	}

	m.session.Status = "stopped"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]

	if m.eventWriter != nil {
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", m.currentTurn),
			ErrorCode: "USER_STOPPED",
			Message:   "Scout session stopped by user",
		})
		m.eventWriter.Close()
		m.eventWriter = nil
	}

	return m.writeSessionState()
}

// MarkDiscoveryComplete transitions to discovery_complete state
func (m *Manager) MarkDiscoveryComplete() error {
	if m.session.Status != "discovery" {
		return fmt.Errorf("cannot mark complete: not in discovery state")
	}

	m.session.Status = "discovery_complete"
	return m.writeSessionState()
}

// MarkVerificationComplete transitions to verification_complete state
func (m *Manager) MarkVerificationComplete() error {
	if m.session.Status != "verification" {
		return fmt.Errorf("cannot mark complete: not in verification state")
	}

	m.session.Status = "verification_complete"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]
	return m.writeSessionState()
}

// CloseEventWriter flushes and closes the current event writer
func (m *Manager) CloseEventWriter() error {
	if m.eventWriter != nil {
		return m.eventWriter.Close()
	}
	return nil
}

// writeSessionState writes the session state to disk
func (m *Manager) writeSessionState() error {
	statusPath := m.config.GetStatusFile()

	data, err := json.MarshalIndent(m.session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(statusPath, data, 0644); err != nil {
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

	statusPath := config.GetStatusFile()
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session state: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
	}

	mgr := &Manager{
		config:      config,
		session:     &session,
		processor:   NewProcessorHelper(config),
		currentTurn: 1,
	}

	return mgr, nil
}
