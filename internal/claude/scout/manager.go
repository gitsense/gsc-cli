/**
 * Component: Scout Session Manager
 * Block-UUID: b30fe060-677a-41a3-a3d4-51dd1dee30f4
 * Parent-UUID: c71bb1d9-9d11-4789-86fa-c82ed2c4d736
 * Version: 1.0.7
 * Description: Orchestrates Scout discovery and verification phases, manages subprocess execution
 * Language: Go
 * Created-at: 2026-03-27T17:27:17.467Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), GLM-4.7 (v1.0.7)
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
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
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

	// Create stdout pipe for stream processing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		// Close stdout pipe on error
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
		return nil, fmt.Errorf("failed to read session state: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
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
	startTime := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse as generic map
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			// Not valid JSON, skip
			continue
		}

		// Check event type
		eventType, _ := result["type"].(string)

		switch eventType {
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

		case "result":
			// Parse result event
			if resultContent, ok := result["result"].(string); ok {
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

// maxTokenSize defines the maximum size for a single JSONL event (10MB)
const maxTokenSize = 10 * 1024 * 1024
