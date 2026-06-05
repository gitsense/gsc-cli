/**
 * Component: Intent Workflow Manager Lifecycle
 * Block-UUID: f97e4b96-6b54-45f4-93d3-c68420ed8888
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Manages session and turn lifecycle, including state transitions (stopped, error, complete), process termination, and finalization of turn results.
 * Language: Go
 * Created-at: 2026-04-28T13:35:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package intent_workflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// CheckProcessStatus checks if the subprocess is still running
func (m *Manager) CheckProcessStatus() (bool, error) {
	if m.processInfo == nil {
		return false, fmt.Errorf("no process info available")
	}

	process, err := os.FindProcess(m.processInfo.PID)
	if err != nil {
		m.debugLogger.LogError("Process not found", err)
		m.processInfo.Running = false
		return false, nil
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		m.debugLogger.LogError("Process signal failed", err)
		m.processInfo.Running = false
		return false, nil
	}

	m.processInfo.Running = true
	return true, nil
}

// StopSession stops the current intent workflow session and cleanup.
// Implements graceful shutdown with SIGTERM → wait 10s → SIGKILL pattern.
func (m *Manager) StopSession() error {
	m.debugLogger.Log("DEBUG", "StopSession called")

	// Phase 1: Pre-Shutdown Validation
	if m.processInfo == nil || !m.processInfo.Running {
		// Already stopped, nothing to do
		m.debugLogger.Log("DEBUG", "Process not running, nothing to stop")
		m.closeDebugLogger()
		return nil
	}

	// Validate session state
	if m.session.Status == "stopped" || m.session.Status == "error" {
		m.debugLogger.Log("DEBUG", "Session already stopped or in error state")
		return nil // Already stopped
	}

	// Get process handle
	process, err := os.FindProcess(m.processInfo.PID)
	if err != nil {
		// Process doesn't exist, mark as error
		m.debugLogger.LogError("Process not found", err)
		m.markAsError("PROCESS_NOT_FOUND", "Process no longer exists")
		m.closeDebugLogger()
		return nil
	}

	// Phase 2: Graceful Shutdown (SIGTERM)
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Sending SIGTERM to PID %d", m.processInfo.PID))
	err = process.Signal(syscall.SIGTERM)

	// CRITICAL FIX: Create .stopped file to communicate with process reaper
	// This fixes race condition where m.session.Stopped is not shared between processes
	stoppedFilePath := filepath.Join(m.config.GetTurnDir(m.currentTurn), ".stopped")
	if err := os.WriteFile(stoppedFilePath, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0644); err != nil {
		m.debugLogger.LogError("Failed to write .stopped file", err)
		// Don't fail the stop operation if we can't write the file
	}
	m.session.Stopped = true // Also set in-memory flag for backward compatibility

	if err != nil {
		// Can't send signal, try force kill
		m.debugLogger.LogError("Failed to send SIGTERM", err)
		m.closeDebugLogger()
		return m.forceKillProcess(process)
	}

	// Write initial stop status
	stopStartTime := time.Now()
	stopStatus := map[string]interface{}{
		"status":      "stopping",
		"started_at":  stopStartTime.UTC().Format(time.RFC3339Nano),
		"watcher_pid": m.GetWatcherPID(),
		"agent_pid":   m.processInfo.PID,
		"message":     "Waiting for graceful shutdown...",
	}
	if err := m.writeStopStatus(stopStatus); err != nil {
		m.debugLogger.LogError("Failed to write stop status", err)
		// Don't fail the stop operation if we can't write the status file
	}

	// Wait for graceful exit (10 second timeout)
	gracefulExit := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		gracefulExit <- err
	}()

	select {
	case <-gracefulExit:
		// Process exited gracefully
		stopCompletedAt := time.Now()
		m.debugLogger.Log("DEBUG", "Process exited gracefully within timeout")

		// Update stop status
		stopStatus["status"] = "stopped"
		stopStatus["message"] = "Process exited gracefully"
		stopStatus["completed_at"] = stopCompletedAt.UTC().Format(time.RFC3339Nano)
		stopStatus["duration_ms"] = stopCompletedAt.Sub(stopStartTime).Milliseconds()
		if err := m.writeStopStatus(stopStatus); err != nil {
			m.debugLogger.LogError("Failed to update stop status", err)
		}

		m.markAsStopped("USER_STOPPED", "Intent workflow session stopped by user")
		m.closeDebugLogger()
		return nil

	case <-time.After(10 * time.Second):
		// Phase 3: Force Kill (timeout exceeded)
		timeoutAt := time.Now()
		m.debugLogger.Log("DEBUG", "Graceful shutdown timeout (10s), forcing kill")

		// Update stop status to indicate timeout
		stopStatus["status"] = "force_killing"
		stopStatus["message"] = "Graceful shutdown timeout, forcing kill"
		stopStatus["timeout_at"] = timeoutAt.UTC().Format(time.RFC3339Nano)
		stopStatus["duration_ms"] = timeoutAt.Sub(stopStartTime).Milliseconds()
		if err := m.writeStopStatus(stopStatus); err != nil {
			m.debugLogger.LogError("Failed to update stop status", err)
		}

		result := m.forceKillProcess(process)

		// Update stop status after force kill
		stopCompletedAt := time.Now()
		stopStatus["status"] = "force_killed"
		stopStatus["message"] = "Process force killed"
		stopStatus["completed_at"] = stopCompletedAt.UTC().Format(time.RFC3339Nano)
		stopStatus["duration_ms"] = stopCompletedAt.Sub(stopStartTime).Milliseconds()
		if err := m.writeStopStatus(stopStatus); err != nil {
			m.debugLogger.LogError("Failed to update stop status", err)
		}

		return result
	}
}

// forceKillProcess sends SIGKILL to a process
func (m *Manager) forceKillProcess(process *os.Process) error {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Sending SIGKILL to PID %d", process.Pid))
	err := process.Signal(syscall.SIGKILL)
	if err != nil {
		m.debugLogger.LogError("Failed to send SIGKILL", err)
		m.markAsError("KILL_FAILED", "Failed to send SIGKILL")
		m.closeDebugLogger()
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
		m.debugLogger.Log("DEBUG", "Process killed successfully")
		m.markAsError("FORCE_STOPPED", "Force stopped after timeout")
		m.closeDebugLogger()
		return nil

	case <-time.After(1 * time.Second):
		// Process still running after SIGKILL
		m.debugLogger.Log("ERROR", "Process became zombie after SIGKILL")
		m.markAsError("ZOMBIE_PROCESS", "Process still running after SIGKILL")
		m.closeDebugLogger()
		return fmt.Errorf("process became zombie after SIGKILL")
	}
}

// finalizeTurn updates session and turn state when a subprocess exits.
// This is called by the process reaper in spawn.go.
func (m *Manager) finalizeTurn(exitCode int, wasStopped bool) {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Finalizing turn: exitCode=%d, wasStopped=%v", exitCode, wasStopped))

	// Update turn state when process exits naturally
	completedAt := time.Now()
	if m.session != nil {
		// Find the current turn and update its state
		for i := range m.session.Turns {
			if m.session.Turns[i].TurnNumber == m.currentTurn {
				// Determine turn status based on exit code and stopped flag
				if wasStopped {
					// Session was stopped by user
					m.session.Turns[i].Status = "stopped"
				} else if exitCode != 0 {
					// Process exited with error (not stopped)
					m.session.Turns[i].Status = "error"
				} else {
					// Process completed successfully
					m.session.Turns[i].Status = "complete"
				}

				m.session.Turns[i].CompletedAt = &completedAt
				m.session.Turns[i].ProcessInfo.Running = false
				// Note: PID is already set in spawn.go

				// Set error if process exited with non-zero code
				if exitCode != 0 {
					errorMsg := fmt.Sprintf("Process exited with code %d", exitCode)
					m.session.Turns[i].Error = &errorMsg
				}
				break
			}
		}

		// Update overall session status
		if wasStopped {
			m.session.Status = "stopped"
		} else if exitCode != 0 {
			m.session.Status = "error"
		} else {
			// Set completion status based on turn type
			if m.session.Turns[m.currentTurn-1].TurnType == "change" {
				m.session.Status = "change_post_processing"
			} else {
				m.session.Status = "discovery_complete"
			}
		}
		m.session.CompletedAt = &completedAt
	}
	if m.processInfo != nil {
		m.processInfo.Running = false
	}
	m.writeSessionState()

	// Close debug logger when process exits naturally
	m.closeDebugLogger()

	// Clean up the .stopped file after both turn and session status are updated
	if wasStopped {
		stoppedFilePath := filepath.Join(m.config.GetTurnDir(m.currentTurn), ".stopped")
		os.Remove(stoppedFilePath)
	}
}

// markAsStopped updates session state and writes error event.
// This method should ONLY be called for USER_STOPPED.
// For all other error codes, use markAsError() instead.
func (m *Manager) markAsStopped(errorCode, message string) {
	// Only use this for USER_STOPPED
	if errorCode != "USER_STOPPED" {
		m.markAsError(errorCode, message)
		return
	}

	if m.session == nil {
		return
	}

	m.debugLogger.Log("ERROR", fmt.Sprintf("Marking session as stopped: %s - %s", errorCode, message))

	// Update session status
	m.session.Status = "stopped"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]

	// Update turn state to stopped (only if not already complete)
	if len(m.session.Turns) > 0 {
		lastTurn := &m.session.Turns[len(m.session.Turns)-1]

		// Don't overwrite if already complete (race condition handling)
		if lastTurn.Status != "complete" {
			lastTurn.Status = "stopped"
			lastTurn.CompletedAt = m.session.CompletedAt
			if lastTurn.ProcessInfo.Running {
				lastTurn.ProcessInfo.Running = false
			}

			// Write stop event to log file
			if lastTurn.LogPath != "" {
				m.writeStopEventToLog(lastTurn.LogPath, errorCode, message)
			}
		}
	}

	// Write error event (for eventWriter if it exists - e.g., during normal error)
	if m.eventWriter != nil {
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", m.currentTurn),
			ErrorCode: errorCode,
			Message:   message,
		})
		// Also write a status event to ensure Phase is set in StatusData
		phase := "discovery"
		if len(m.session.Turns) > 0 {
			lastTurn := m.session.Turns[len(m.session.Turns)-1]
			if lastTurn.TurnType == "validation" {
				phase = "validation"
			} else if lastTurn.TurnType == "change" {
				phase = "change"
			}
		}
		m.eventWriter.WriteStatusEvent(StatusEvent{Phase: phase, Message: message})
		m.eventWriter.Close()
		m.eventWriter = nil
	}

	// Update process info
	if m.processInfo != nil {
		m.processInfo.Running = false
	}

	// Persist state
	if err := m.writeSessionState(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to persist session state: %v\n", err)
	}
}

// markAsError updates session state and writes error event for system failures.
// This should be used for parse errors, validation errors, process crashes, etc.
// Use markAsStopped() only for USER_STOPPED.
func (m *Manager) markAsError(errorCode, message string) {
	if m.session == nil {
		return
	}

	m.debugLogger.Log("ERROR", fmt.Sprintf("Marking session as error: %s - %s", errorCode, message))

	// Update session status to ERROR (not stopped)
	m.session.Status = "error"
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]

	// Update turn state to error
	if len(m.session.Turns) > 0 {
		lastTurn := &m.session.Turns[len(m.session.Turns)-1]
		
		// Don't overwrite if already complete (race condition handling)
		if lastTurn.Status != "complete" {
			lastTurn.Status = "error"
			lastTurn.CompletedAt = m.session.CompletedAt
			if lastTurn.ProcessInfo.Running {
				lastTurn.ProcessInfo.Running = false
			}
			
			// Write error event to log file
			if lastTurn.LogPath != "" {
				m.writeErrorEventToLog(lastTurn.LogPath, errorCode, message)
			}
		}
	}

	// Write error event (for eventWriter if it exists - e.g., during normal error)
	if m.eventWriter != nil {
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", m.currentTurn),
			ErrorCode: errorCode,
			Message:   message,
		})
		// Also write a status event to ensure Phase is set in StatusData
		phase := "discovery"
		if len(m.session.Turns) > 0 {
			lastTurn := m.session.Turns[len(m.session.Turns)-1]
			if lastTurn.TurnType == "validation" {
				phase = "validation"
			} else if lastTurn.TurnType == "change" {
				phase = "change"
			}
		}
		m.eventWriter.WriteStatusEvent(StatusEvent{Phase: phase, Message: message})
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

// MarkTurnAsError marks a specific turn as error with structured error details
// This is used when validation fails with file-level errors
func (m *Manager) MarkTurnAsError(turnNumber int, errorDetails *ErrorDetails) {
	if m.session == nil {
		return
	}

	m.debugLogger.Log("ERROR", fmt.Sprintf("Marking turn %d as error: %s - %s", turnNumber, errorDetails.ErrorCode, errorDetails.Message))

	// Update session status to ERROR
	m.session.Status = "error"
	m.session.ErrorDetails = errorDetails
	m.session.CompletedAt = &[]time.Time{time.Now()}[0]

	// Update turn state to error
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == turnNumber {
			turnState := &m.session.Turns[i]
			turnState.Status = "error"
			turnState.CompletedAt = m.session.CompletedAt
			turnState.ErrorDetails = errorDetails
			if turnState.ProcessInfo.Running {
				turnState.ProcessInfo.Running = false
			}
			
			// Write error event to log file
			if turnState.LogPath != "" {
				m.writeErrorEventToLog(turnState.LogPath, errorDetails.ErrorCode, errorDetails.Message)
			}
			break
		}
	}

	// Persist state
	if err := m.writeSessionState(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to persist session state: %v\n", err)
	}
}

// writeStopStatus writes the stop status to a JSON file in the turn directory
func (m *Manager) writeStopStatus(status map[string]interface{}) error {
	turnDir := m.config.GetTurnDir(m.currentTurn)
	stopStatusPath := filepath.Join(turnDir, "stop-status.json")

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stop status: %w", err)
	}

	if err := os.WriteFile(stopStatusPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write stop status file: %w", err)
	}

	m.debugLogger.Log("DEBUG", fmt.Sprintf("Stop status written to: %s", stopStatusPath))
	return nil
}

// writeStopEventToLog appends a stop event and stream-end marker to the log file
func (m *Manager) writeStopEventToLog(logPath, errorCode, message string) {
	// Open log file in append mode
	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		m.debugLogger.LogError("Failed to open log file for stop event", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write stop event
	stopEvent := map[string]interface{}{
		"type":      "stopped",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"data": map[string]interface{}{
			"error_code": errorCode,
			"message":    message,
		},
	}
	stopEventBytes, _ := json.Marshal(stopEvent)
	writer.WriteString(string(stopEventBytes) + "\n")

	// Write stream end marker with stopped flag
	endMarker := map[string]interface{}{
		"type":       "gsc-agent-stream-end",
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"source":     "gsc-agent",
		"session_id": m.session.SessionID,
		"turn":       m.currentTurn,
		"stopped":    true,
	}
	endMarkerBytes, _ := json.Marshal(endMarker)
	writer.WriteString(string(endMarkerBytes) + "\n")
}

// writeErrorEventToLog appends an error event to the log file
func (m *Manager) writeErrorEventToLog(logPath, errorCode, message string) {
	// Open log file in append mode
	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		m.debugLogger.LogError("Failed to open log file for error event", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write error event
	errorEvent := map[string]interface{}{
		"type":      "error",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"data": map[string]interface{}{
			"phase":       fmt.Sprintf("turn-%d", m.currentTurn),
			"error_code":  errorCode,
			"message":     message,
		},
	}
	errorEventBytes, _ := json.Marshal(errorEvent)
	writer.WriteString(string(errorEventBytes) + "\n")
}

// SetWatcherPID sets the PID of the background watcher process
func (m *Manager) SetWatcherPID(pid int) {
	if m.session == nil {
		return
	}
	m.session.WatcherPID = &pid
}

// GetWatcherPID returns the PID of the background watcher process
func (m *Manager) GetWatcherPID() int {
	if m.session == nil {
		return 0
	}
	if m.session.WatcherPID == nil {
		return 0
	}
	return *m.session.WatcherPID
}
