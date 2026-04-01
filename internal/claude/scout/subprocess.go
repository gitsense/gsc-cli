/**
 * Component: Scout Subprocess Manager
 * Block-UUID: d8d9056a-66ab-49e9-9377-c1e616ba809a
 * Parent-UUID: c0b1f385-4707-42c8-8de4-41a700c20bde
 * Version: 1.0.0
 * Description: Manages subprocess spawning, process lifecycle, signal handling, and resource cleanup for Scout Claude sessions
 * Language: Go
 * Created-at: 2026-04-01T14:45:00.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// spawnClaudeSubprocess spawns the claude subprocess for a turn
func (m *Manager) spawnClaudeSubprocess(turn int) error {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Spawning subprocess for turn %d", turn))

	// Write Scout permissions to restrict Bash to gsc commands only
	if err := WriteScoutPermissions(m.config.GetTurnDir(turn)); err != nil {
		m.debugLogger.LogError("Failed to write permissions", err)
		return fmt.Errorf("failed to write permissions: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Permissions written successfully")

	// Get the Claude prompt template using absolute path
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		m.debugLogger.LogError("Failed to get GSC_HOME", err)
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("GSC_HOME: %s", gscHome))

	// Write reference files NDJSON to turn directory
	if err := m.writeReferenceFilesNDJSON(); err != nil {
		m.debugLogger.LogError("Failed to write reference files", err)
		return fmt.Errorf("failed to write reference files: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Reference files written successfully")

	var templateName string
	if turn == 1 {
		templateName = "turn-1-discovery.md"
	} else {
		templateName = "turn-2-verification.md"
	}

	promptPath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", templateName)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		m.debugLogger.LogError("Failed to read prompt template", err)
		m.markAsStopped("TEMPLATE_FAILED", fmt.Sprintf("Failed to read prompt template: %v", err))
		return fmt.Errorf("failed to read prompt template: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Read prompt template: %s", promptPath))

	// Build the command flags for the bash script
	// Format reference files metadata and replace placeholder
	refFilesMarkdown := m.formatReferenceFilesMetadata()
	promptStr := strings.ReplaceAll(string(promptData), "{{REFERENCE_FILES}}", refFilesMarkdown)

	// Replace other placeholders
	promptStr = strings.ReplaceAll(promptStr, "{{INTENT}}", m.session.Intent)

	// Format working directories and replace placeholder
	workdirsMarkdown := m.formatWorkingDirectories()
	promptStr = strings.ReplaceAll(promptStr, "{{WORKING_DIRECTORIES}}", workdirsMarkdown)

	promptData = []byte(promptStr)

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Windows: Write prompt to file and use @file syntax
		m.debugLogger.Log("DEBUG", "Using Windows file-based approach for prompt")
		promptFile := filepath.Join(m.config.GetTurnDir(turn), "prompt.txt")
		if err := os.WriteFile(promptFile, []byte(promptStr), 0644); err != nil {
			m.debugLogger.LogError("Failed to write prompt file", err)
			m.markAsStopped("PROMPT_FAILED", fmt.Sprintf("Failed to write prompt file: %v", err))
			return fmt.Errorf("failed to write prompt file: %w", err)
		}
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Prompt written to: %s", promptFile))

		// Build add-dir flags
		var addDirFlags []string
		for _, wd := range m.session.WorkingDirectories {
			addDirFlags = append(addDirFlags, "--add-dir", wd.Path)
		}

		// Build claude command
		args := []string{
			"--allowedTools", "Read,Bash",
			"--verbose",
			"--include-partial-messages",
			"--output-format", "stream-json",
		}
		args = append(args, addDirFlags...)
		if m.session.Model != "" {
			args = append(args, "--model", m.session.Model)
		}
		args = append(args, "--print", "-p", "@"+promptFile)

		cmd = exec.Command("claude", args...)
		cmd.Dir = m.config.GetTurnDir(turn)
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Claude command: %s", cmd.String()))
	} else {
		// Unix: Use bash script with heredoc
		m.debugLogger.Log("DEBUG", "Using Unix bash script with heredoc")

		// Build add-dir flags
		var addDirFlags []string
		for _, wd := range m.session.WorkingDirectories {
			addDirFlags = append(addDirFlags, fmt.Sprintf("--add-dir %s", wd.Path))
		}
		addDirFlagsStr := strings.Join(addDirFlags, " ")

		// Build model flag if specified
		modelFlag := ""
		if m.session.Model != "" {
			modelFlag = fmt.Sprintf("--model %s", m.session.Model)
		}

		// Create bash script content using heredoc
		scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

echo "=== Starting Claude Scout subprocess ==="
echo "Working directory: $(pwd)"
echo "Turn: %d"
echo "Session ID: %s"
echo "=== Executing Claude command ==="

claude --allowedTools Read,Bash \
--verbose \
--include-partial-messages \
--output-format stream-json \
%s \
%s \
-p <<'ENDOFPROMPT'
%s
ENDOFPROMPT

echo "=== Claude subprocess completed ==="
exit_code=$?
echo "Exit code: $exit_code"
exit $exit_code
`, turn, m.session.SessionID, addDirFlagsStr, modelFlag, promptStr)

		// Write bash script to turn directory
		scriptPath := filepath.Join(m.config.GetTurnDir(turn), "run-claude.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			m.debugLogger.LogError("Failed to write bash script", err)
			m.markAsStopped("SCRIPT_FAILED", fmt.Sprintf("Failed to write bash script: %v", err))
			return fmt.Errorf("failed to write bash script: %w", err)
		}
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Bash script written to: %s", scriptPath))

		// Execute the bash script
		cmd = exec.Command("/bin/bash", scriptPath)
		cmd.Dir = m.config.GetTurnDir(turn)
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Working directory: %s", cmd.Dir))
	}

	// Create stderr pipe for error capture
	// Create stdout pipe for stream processing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.debugLogger.LogError("Failed to create stdout pipe", err)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Stdout pipe created successfully")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.debugLogger.LogError("Failed to create stderr pipe", err)
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Stderr pipe created successfully")

	// Start the process
	if err := cmd.Start(); err != nil {
		m.debugLogger.LogError("Failed to start subprocess", err)
		// Close stdout pipe on error
		m.markAsStopped("START_FAILED", fmt.Sprintf("Failed to start subprocess: %v", err))
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start subprocess: %w", err)
	}

	m.processInfo = &ProcessInfo{
		PID:     cmd.Process.Pid,
		Command: cmd.String(),
		Running: true,
	}
	m.debugLogger.LogProcessSpawn(cmd.Process.Pid, cmd.String(), cmd.Dir)

	// Start background goroutine to process stream
	m.debugLogger.Log("DEBUG", "Starting stream processing goroutine")
	m.wg.Add(1)
	go m.processStream(stdout, turn)

	// Start background goroutine to reap zombie process
	m.debugLogger.Log("DEBUG", "Starting process reaper goroutine")
	m.wg.Add(1)
	go func() {
		m.debugLogger.Log("DEBUG", "Process reaper: waiting for process to exit")
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				m.debugLogger.Log("DEBUG", fmt.Sprintf("Process reaper: process exited with code %d", exitCode))
			}
		}
		m.debugLogger.LogProcessExit(cmd.Process.Pid, exitCode, err)
		// Close debug logger when process exits naturally
		m.closeDebugLogger()
		m.wg.Done()
	}()

	// Start background goroutine to capture stderr
	m.wg.Add(1)
	go m.captureStderr(stderr)
	return nil
}

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

// StopSession stops the current scout session and cleanup
// Implements graceful shutdown with SIGTERM → wait 5s → SIGKILL pattern
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
		// Process doesn't exist, mark as stopped
		m.debugLogger.LogError("Process not found", err)
		m.markAsStopped("PROCESS_NOT_FOUND", "Process no longer exists")
		m.closeDebugLogger()
		return nil
	}

	// Phase 2: Graceful Shutdown (SIGTERM)
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Sending SIGTERM to PID %d", m.processInfo.PID))
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		// Can't send signal, try force kill
		m.debugLogger.LogError("Failed to send SIGTERM", err)
		m.closeDebugLogger()
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
		m.debugLogger.Log("DEBUG", "Process exited gracefully")
		m.markAsStopped("USER_STOPPED", "Scout session stopped by user")
		m.closeDebugLogger()
		return nil

	case <-time.After(5 * time.Second):
		// Phase 3: Force Kill (timeout exceeded)
		m.debugLogger.Log("DEBUG", "Graceful shutdown timeout, forcing kill")
		return m.forceKillProcess(process)
	}
}

// forceKillProcess sends SIGKILL to a process
func (m *Manager) forceKillProcess(process *os.Process) error {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Sending SIGKILL to PID %d", process.Pid))
	// Send SIGKILL
	err := process.Signal(syscall.SIGKILL)
	if err != nil {
		m.debugLogger.LogError("Failed to send SIGKILL", err)
		m.markAsStopped("KILL_FAILED", "Failed to send SIGKILL")
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
		m.markAsStopped("FORCE_STOPPED", "Force stopped after timeout")
		m.closeDebugLogger()
		return nil

	case <-time.After(1 * time.Second):
		// Process still running after SIGKILL
		m.debugLogger.Log("ERROR", "Process became zombie after SIGKILL")
		m.markAsStopped("ZOMBIE_PROCESS", "Process still running after SIGKILL")
		m.closeDebugLogger()
		return fmt.Errorf("process became zombie after SIGKILL")
	}
}

// markAsStopped updates session state and writes error event
func (m *Manager) markAsStopped(errorCode, message string) {
	if m.session == nil {
		return
	}

	m.debugLogger.Log("ERROR", fmt.Sprintf("Marking session as stopped: %s - %s", errorCode, message))

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
		// Also write a status event to ensure Phase is set in StatusData
		phase := "discovery"
		if m.currentTurn == 2 {
			phase = "verification"
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

// writeReferenceFilesNDJSON writes the reference files to an NDJSON file in the turn directory
func (m *Manager) writeReferenceFilesNDJSON() error {
	if len(m.session.ReferenceFilesContext) == 0 {
		return nil // No reference files to write
	}

	turnDir := m.config.GetTurnDir(m.currentTurn)
	refDir := filepath.Join(turnDir, "turn-data")

	if err := os.MkdirAll(refDir, 0755); err != nil {
		m.debugLogger.LogError("Failed to create turn-data directory", err)
		return fmt.Errorf("failed to create turn-data directory: %w", err)
	}

	refPath := filepath.Join(refDir, "references.ndjson")
	file, err := os.Create(refPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create references.ndjson", err)
		return fmt.Errorf("failed to create references.ndjson: %w", err)
	}
	defer file.Close()

	for _, ref := range m.session.ReferenceFilesContext {
		data, err := json.Marshal(ref)
		if err != nil {
			m.debugLogger.LogError("Failed to marshal reference file", err)
			return fmt.Errorf("failed to marshal reference file: %w", err)
		}
		if _, err := file.WriteString(string(data) + "\n"); err != nil {
			m.debugLogger.LogError("Failed to write reference file", err)
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

// formatWorkingDirectories formats working directories for display in the prompt
func (m *Manager) formatWorkingDirectories() string {
	if len(m.session.WorkingDirectories) == 0 {
		return "No working directories provided."
	}

	var sb strings.Builder
	sb.WriteString("The following working directories will be searched:\n")
	for i, wd := range m.session.WorkingDirectories {
		sb.WriteString(fmt.Sprintf("- workdir-%03d: %s (path: %s)\n",
			i+1, wd.Name, wd.Path))
	}
	return sb.String()
}
