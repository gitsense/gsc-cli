/**
 * Component: Scout Session Manager
 * Block-UUID: 241bdaca-3510-4303-9b9b-5dcf4c1258f0
 * Parent-UUID: 8211b556-ba81-49bc-866a-bba885c4ab60
 * Version: 1.2.3
 * Description: Orchestrates Scout discovery and verification phases, manages subprocess execution
 * Language: Go
 * Created-at: 2026-04-01T01:15:39.805Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.2), GLM-4.7 (v1.2.3)
 */


package scout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
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
	session     *Session
	processor   *ProcessorHelper
	eventWriter *EventWriter
	currentTurn int
	processInfo *ProcessInfo
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

// GetConfig returns the session configuration
func (m *Manager) GetConfig() *SessionConfig {
	return m.config
}

// InitializeSession sets up the session directory and writes initial state
func (m *Manager) InitializeSession(intent string, workdirs []WorkingDirectory, refFilesContext []ReferenceFileContext, autoReview bool, model string) error {
	// Validate inputs
	if err := ValidateIntent(intent); err != nil {
		return err
	}

	if errs, _ := ValidateSetup(workdirs, refFilesContext); len(errs) > 0 {
		// Build detailed error message
		var errorDetails []string
		for _, e := range errs {
			errorDetails = append(errorDetails, fmt.Sprintf("  - %s: %s", e.Type, e.Message))
		}
		errMsg := fmt.Sprintf("validation failed with %d error(s):\n%s", len(errs), strings.Join(errorDetails, "\n"))
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
		Model:              model,
		WorkingDirectories: workdirs,
		ReferenceFilesContext: refFilesContext,
		AutoReview:         autoReview,
		Status:             "discovery",
		StartedAt:          time.Now(),
	}

	// Reference files from NDJSON already have content embedded, no need to copy

	return m.writeSessionState()
}

// PrepareTurn1 generates input schema and handles no-brains case
func (m *Manager) PrepareTurn1() error {
	// 1. Build input schema
	schema, err := BuildInputSchema(m.session.SessionID, m.session.WorkingDirectories)
	if err != nil {
		return err
	}

	// 2. Write input schema to file
	schemaPath := m.config.GetInputSchemaFile()
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal input schema: %w", err)
	}
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write input schema: %w", err)
	}

	// 3. Check if all brains unavailable
	if checkAllBrainsUnavailable(schema.WorkingDirectories) {
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
		return err
	}
	defer writer.Close()

	return writer.WriteErrorEvent(ErrorEvent{
		Phase:     "turn-1",
		ErrorCode: "NO_BRAINS_AVAILABLE",
		Message:   "No brains available in any working directory",
	})
}

// StartTurn1Discovery initiates the discovery phase and spawns subprocess
func (m *Manager) StartTurn1Discovery() error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	if m.session.Status != "discovery" && m.session.Status != "error" {
		return fmt.Errorf("cannot start discovery: session status is %s", m.session.Status)
	}

	// Prepare Turn 1 (generate input schema)
	if err := m.PrepareTurn1(); err != nil {
		return err
	}

	// Check if session already errored out (no brains)
	if m.session.Status == "error" {
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
		m.markAsStopped("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}

	// Write init event
	initEvent := InitEvent{
		SessionID:          m.session.SessionID,
		Intent:             m.session.Intent,
		WorkingDirectories: m.session.WorkingDirectories,
		ReferenceFilesContext: m.session.ReferenceFilesContext,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
			Model:      m.session.Model,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		return err
	}

	// Spawn subprocess for Turn 1
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
		m.markAsStopped("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
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
		m.markAsStopped("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}

	// Write init event with selected candidates
	initEvent := InitEvent{
		SessionID:          m.session.SessionID,
		Intent:             m.session.Intent,
		WorkingDirectories: m.session.WorkingDirectories,
		ReferenceFilesContext: m.session.ReferenceFilesContext,
		Options: InitOptions{
			AutoReview: m.session.AutoReview,
			Turn:       m.currentTurn,
			Model:      m.session.Model,
		},
	}
	if err := m.eventWriter.WriteInitEvent(initEvent); err != nil {
		return err
	}

	// Save selected candidates for Claude to reference
	if selectedCandidates != nil {
		candData, _ := json.MarshalIndent(selectedCandidates, "", "  ")
		candPath := filepath.Join(m.config.GetTurnDir(m.currentTurn), "selected-candidates.json")
		if err := os.WriteFile(candPath, candData, 0644); err != nil {
			m.markAsStopped("CANDIDATE_SAVE_FAILED", fmt.Sprintf("Failed to save selected candidates: %v", err))
			return err
		}
	}

	// Spawn subprocess for Turn 2
	if err := m.spawnClaudeSubprocess(m.currentTurn); err != nil {
		m.markAsStopped("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
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

	// Write reference files NDJSON to turn directory
	if err := m.writeReferenceFilesNDJSON(); err != nil {
		return fmt.Errorf("failed to write reference files: %w", err)
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
		m.markAsStopped("TEMPLATE_FAILED", fmt.Sprintf("Failed to read prompt template: %v", err))
		return fmt.Errorf("failed to read prompt template: %w", err)
	}

	// Format reference files metadata and replace placeholder
	refFilesMarkdown := m.formatReferenceFilesMetadata()
	promptStr := strings.ReplaceAll(string(promptData), "{{REFERENCE_FILES}}", refFilesMarkdown)
	
	// Replace other placeholders
	promptStr = strings.ReplaceAll(promptStr, "{{INTENT}}", m.session.Intent)
	// Add working directories formatting here if needed
	
	promptData = []byte(promptStr)

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

	if m.session.Model != "" {
		flags = append(flags, "--model", m.session.Model)
	}

	cmd := exec.Command("claude", flags...)
	cmd.Dir = m.config.GetTurnDir(turn)

	// Create stdout pipe for stream processing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		// Close stdout pipe on error
		m.markAsStopped("START_FAILED", fmt.Sprintf("Failed to start subprocess: %v", err))
		stdout.Close()
		return fmt.Errorf("failed to start subprocess: %w", err)
	}

	m.processInfo = &ProcessInfo{
		PID:     cmd.Process.Pid,
		Command: cmd.String(),
		Running: true,
	}

	// Start background goroutine to process stream
	go m.processStream(stdout, turn)

	// Start background goroutine to reap zombie process
	go func() {
		cmd.Wait()
	}()

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
		return nil, fmt.Errorf("no results found for turn %d", turn)
	}

	// Read events from the log file
	reader, err := NewEventReader(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read turn results: %w", err)
	}
	defer reader.Close()

	events, err := reader.ReadAllEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	if len(events) == 0 {
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
			return nil, fmt.Errorf("failed to read Turn 1 log file: %w", err)
		}

		reader, err := NewEventReader(originalLogFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open Turn 1 log file: %w", err)
		}
		defer reader.Close()

		originalEvents, err := reader.ReadAllEvents()
		if err != nil {
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
	if err := process.Signal(syscall.Signal(0)); err != nil {
		m.processInfo.Running = false
		return false, nil
	}

	m.processInfo.Running = true
	return true, nil
}

// StopSession stops the current scout session and cleanup
// Implements graceful shutdown with SIGTERM → wait 5s → SIGKILL pattern
func (m *Manager) StopSession() error {
	// Phase 1: Pre-Shutdown Validation
	if m.processInfo == nil || !m.processInfo.Running {
		// Already stopped, nothing to do
		return nil
	}

	// Validate session state
	if m.session.Status == "stopped" || m.session.Status == "error" {
		return nil // Already stopped
	}

	// Get process handle
	process, err := os.FindProcess(m.processInfo.PID)
	if err != nil {
		// Process doesn't exist, mark as stopped
		m.markAsStopped("PROCESS_NOT_FOUND", "Process no longer exists")
		return nil
	}

	// Phase 2: Graceful Shutdown (SIGTERM)
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		// Can't send signal, try force kill
		return m.forceKillProcess(process)
	}

	// Wait for graceful exit (5 second timeout)
	gracefulExit := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		gracefulExit <- err
	}()

	select {
	case <-gracefulExit:
		// Process exited gracefully
		m.markAsStopped("USER_STOPPED", "Scout session stopped by user")
		return nil

	case <-time.After(5 * time.Second):
		// Phase 3: Force Kill (timeout exceeded)
		return m.forceKillProcess(process)
	}
}

// forceKillProcess sends SIGKILL to a process
func (m *Manager) forceKillProcess(process *os.Process) error {
	// Send SIGKILL
	err := process.Signal(syscall.SIGKILL)
	if err != nil {
		m.markAsStopped("KILL_FAILED", "Failed to send SIGKILL")
		return err
	}

	// Wait for process to die (1 second timeout)
	killExit := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		killExit <- err
	}()

	select {
	case <-killExit:
		// Process killed
		m.markAsStopped("FORCE_STOPPED", "Force stopped after timeout")
		return nil

	case <-time.After(1 * time.Second):
		// Process still running after SIGKILL
		m.markAsStopped("ZOMBIE_PROCESS", "Process still running after SIGKILL")
		return fmt.Errorf("process became zombie after SIGKILL")
	}
}

// markAsStopped updates session state and writes error event
func (m *Manager) markAsStopped(errorCode, message string) {
	if m.session == nil {
		return
	}

	// Update session status
	m.session.Status = "stopped"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]

	// Write error event
	if m.eventWriter != nil {
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", m.currentTurn),
			ErrorCode: errorCode,
			Message:   message,
		})
		m.eventWriter.Close()
		m.eventWriter = nil
	}

	// Update process info
	if m.processInfo != nil {
		m.processInfo.Running = false
	}

	// Persist state
	if err := m.writeSessionState(); err != nil {
		// Log error but don't fail - state update is best-effort
		fmt.Fprintf(os.Stderr, "failed to persist session state: %v\n", err)
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
		return m.eventWriter.Close()
	}
	return nil
}

// writeSessionState writes the session state to disk
func (m *Manager) writeSessionState() error {
	statusPath := m.config.GetStatusFile()

	// Get full status data including candidates, process info, etc.
	status, err := m.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
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
		SessionID:          statusData.SessionID,
		Status:             statusData.Status,
		StartedAt:          statusData.StartedAt,
		CompletedAt:        statusData.CompletedAt,
		Error:              statusData.Error,
		// WorkingDirectories, ReferenceFiles, Intent, AutoReview are not in StatusData
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
		currentTurn: currentTurn,
	}

	// Initialize ProcessInfo from loaded status
	mgr.processInfo = &statusData.ProcessInfo

	return mgr, nil
}

// processStream reads Claude's stdout and processes events
func (m *Manager) processStream(stdout io.Reader, turn int) {
	defer func() {
		// Close event writer when done
		if m.eventWriter != nil {
			m.eventWriter.Close()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)

	var usage Usage
	var cost float64
	var duration int64
	var claudeSessionID string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Skip empty lines
			continue
		}

		// Parse as generic map
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			// Not valid JSON, skip
			// Log as raw event for debugging
			m.eventWriter.WriteRawEvent(line)
			continue
		}

		// Check event type
		eventType, _ := result["type"].(string)

		switch eventType {
		case "ping":
			// Skip keep-alive events
			continue

		case "usage":
			// Parse usage event
			if usageData, ok := result["usage"].(map[string]interface{}); ok {
				usage = Usage{
					InputTokens:        int(usageData["input_tokens"].(float64)),
					OutputTokens:       int(usageData["output_tokens"].(float64)),
					CacheCreationTokens: int(usageData["cache_creation_input_tokens"].(float64)),
					CacheReadTokens:    int(usageData["cache_read_input_tokens"].(float64)),
				}
				if c, ok := result["cost"].(float64); ok {
					cost = c
				}
			}

		case "error":
			// Handle error events from Claude CLI
			if errorData, ok := result["error"].(map[string]interface{}); ok {
				errorType, _ := errorData["type"].(string)
				errorMsg, _ := errorData["message"].(string)
				m.eventWriter.WriteErrorEvent(ErrorEvent{
					Phase:     fmt.Sprintf("turn-%d", turn),
					ErrorCode: errorType,
					Message:   errorMsg,
					Details:   line,
				})
				m.session.Status = "error"
				m.writeSessionState()
			}
			continue

		case "system":
			// Handle system events (API retries, etc.)
			// Log but don't fail - these are informational
			continue

		case "result":
			// Parse result event
			if resultContent, ok := result["result"].(string); ok {
				// Check for error in result event
				if isError, ok := result["is_error"].(bool); ok && isError {
					m.eventWriter.WriteErrorEvent(ErrorEvent{
						Phase:     fmt.Sprintf("turn-%d", turn),
						ErrorCode: "RESULT_ERROR",
						Message:   fmt.Sprintf("Claude returned error: %v", result),
						Details:   line,
					})
					m.session.Status = "error"
					m.writeSessionState()
					continue
				}

				// Extract usage/cost from outer event
				if usageData, ok := result["usage"].(map[string]interface{}); ok {
					usage = Usage{
						InputTokens:        int(usageData["input_tokens"].(float64)),
						OutputTokens:       int(usageData["output_tokens"].(float64)),
						CacheCreationTokens: int(usageData["cache_creation_input_tokens"].(float64)),
						CacheReadTokens:    int(usageData["cache_read_input_tokens"].(float64)),
					}
				}
				if c, ok := result["total_cost_usd"].(float64); ok {
					cost = c
				}
				if d, ok := result["duration_ms"].(float64); ok {
					duration = int64(d)
				}
				if sid, ok := result["session_id"].(string); ok {
					claudeSessionID = sid
				}

				// Parse Scout's JSON from result field
				// Try Turn 1 format first
				var turn1Result struct {
					Candidates []Candidate `json:"candidates"`
					TotalFound int         `json:"total_found"`
					Coverage   string      `json:"coverage"`
				}
				if err := json.Unmarshal([]byte(resultContent), &turn1Result); err == nil {
					// Write candidates event
					totalFound := turn1Result.TotalFound
					if totalFound == 0 {
						totalFound = 0 // Explicitly 0
					}

					m.eventWriter.WriteCandidatesEvent(CandidatesEvent{
						Phase:      "discovery",
						TotalFound: totalFound,
						Candidates: turn1Result.Candidates,
					})

					// Write done event with usage/cost
					m.eventWriter.WriteDoneEvent(DoneEvent{
						Status:           "success",
						TotalCandidates:  totalFound,
						PhaseCompleted:   "discovery",
						Summary: CompletionSummary{
							FilesFound: totalFound,
						},
						Usage:           &usage,
						Cost:            &cost,
						Duration:        &duration,
						ClaudeSessionID: &claudeSessionID,
					})

					// Update session status
					m.session.Status = "discovery_complete"
					m.writeSessionState()
					break
				}

				// Try Turn 2 format
				var turn2Result struct {
					VerifiedCandidates []VerificationUpdate `json:"verified_candidates"`
					Summary           struct {
						TotalVerified      int `json:"total_verified"`
						CandidatesPromoted int `json:"candidates_promoted"`
						CandidatesDemoted  int `json:"candidates_demoted"`
						CandidatesRemoved  int `json:"candidates_removed"`
						AverageVerifiedScore float64 `json:"average_verified_score"`
						TopCandidatesCount int `json:"top_candidates_count"`
					} `json:"summary"`
				}
				if err := json.Unmarshal([]byte(resultContent), &turn2Result); err == nil {
					// Write verified event
					m.eventWriter.WriteVerifiedEvent(VerifiedEvent{
						Phase:             "verification",
						TotalVerified:     turn2Result.Summary.TotalVerified,
						UpdatedCandidates: turn2Result.VerifiedCandidates,
					})

					// Write done event with usage/cost
					m.eventWriter.WriteDoneEvent(DoneEvent{
						Status:           "success",
						TotalCandidates:  turn2Result.Summary.TotalVerified,
						PhaseCompleted:   "verification",
						Summary: CompletionSummary{
							FilesFound: turn2Result.Summary.TotalVerified,
						},
						Usage:           &usage,
						Cost:            &cost,
						Duration:        &duration,
						ClaudeSessionID: &claudeSessionID,
					})

					// Update session status
					m.session.Status = "verification_complete"
					m.writeSessionState()
					break
				}
			}
		default:
			// Log unknown event types for debugging
			m.eventWriter.WriteRawEvent(line)
		}
	}

	// Handle scanner errors
	if err := scanner.Err(); err != nil {
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", turn),
			ErrorCode: "STREAM_ERROR",
			Message:   fmt.Sprintf("Error reading stream: %v", err),
		})
		m.session.Status = "error"
		m.writeSessionState()
	}
}

// writeReferenceFilesNDJSON writes the reference files to an NDJSON file in the turn directory
func (m *Manager) writeReferenceFilesNDJSON() error {
	if len(m.session.ReferenceFilesContext) == 0 {
		return nil // No reference files to write
	}

	turnDir := m.config.GetTurnDir(m.currentTurn)
	refDir := filepath.Join(turnDir, "turn-data")
	
	if err := os.MkdirAll(refDir, 0755); err != nil {
		return fmt.Errorf("failed to create turn-data directory: %w", err)
	}

	refPath := filepath.Join(refDir, "references.ndjson")
	file, err := os.Create(refPath)
	if err != nil {
		return fmt.Errorf("failed to create references.ndjson: %w", err)
	}
	defer file.Close()

	for _, ref := range m.session.ReferenceFilesContext {
		data, err := json.Marshal(ref)
		if err != nil {
			return fmt.Errorf("failed to marshal reference file: %w", err)
		}
		if _, err := file.WriteString(string(data) + "\n"); err != nil {
			return fmt.Errorf("failed to write reference file: %w", err)
		}
	}

	return nil
}

// formatReferenceFilesMetadata formats reference files for display in the prompt
func (m *Manager) formatReferenceFilesMetadata() string {
	if len(m.session.ReferenceFilesContext) == 0 {
		return "No reference files provided."
	}

	var sb strings.Builder
	sb.WriteString("The following reference files have been imported:\n")
	for i, ref := range m.session.ReferenceFilesContext {
		sb.WriteString(fmt.Sprintf("- reference-file-%03d: %s (chat-id: %d, repo: %s)\n", 
			i+1, ref.RelativePath, ref.ChatID, ref.Repository))
	}
	sb.WriteString("\n**Note:** Complete reference file data is available in `turn-data/references.ndjson` if you need to examine raw content.\n")
	return sb.String()
}
