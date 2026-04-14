/**
 * Component: Scout Subprocess Manager
 * Block-UUID: 70ddc5ab-9b69-4651-bad4-8d844cd1c8d8
 * Parent-UUID: 5b13fbbe-0a6f-4cd8-a6eb-b8e9cf0ab842
 * Version: 2.14.0
 * Description: Manages subprocess spawning, process lifecycle, signal handling, and resource cleanup for Scout Claude sessions. Updated to find gsc location using exec.LookPath and add its directory to PATH in subprocess. Fixed intent file reading to read from turn directory instead of session directory. Added CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS environment variable to increase file reading limit to 15000 tokens (user-overridable).
 * Language: Go
 * Created-at: 2026-04-13T14:45:45.510Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.6.0), GLM-4.7 (v2.7.0), GLM-4.7 (v2.8.0), GLM-4.7 (v2.9.0), GLM-4.7 (v2.10.0), GLM-4.7 (v2.11.0), GLM-4.7 (v2.12.0), GLM-4.7 (v2.13.0), GLM-4.7 (v2.14.0)
 */


package scout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// spawnClaudeSubprocess spawns the claude subprocess for a turn
func (m *Manager) spawnClaudeSubprocess(turn int, turnType string) error {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Spawning subprocess for turn %d", turn))

	// Get the Claude prompt template using absolute path
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		m.debugLogger.LogError("Failed to get GSC_HOME", err)
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("GSC_HOME: %s", gscHome))

	// Find gsc location to add to PATH
	gscPath, err := exec.LookPath("gsc")
	if err != nil {
		m.debugLogger.LogError("Failed to find gsc in PATH", err)
		return fmt.Errorf("gsc not found in PATH: %w", err)
	}
	gscDir := filepath.Dir(gscPath)
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Found gsc at: %s", gscPath))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Adding to PATH: %s", gscDir))

	// Write Scout permissions to restrict Bash to gsc commands only
	if err := WriteScoutPermissions(m.config.GetTurnDir(turn)); err != nil {
		m.debugLogger.LogError("Failed to write permissions", err)
		return fmt.Errorf("failed to write permissions: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Permissions written successfully")

	// Write reference files NDJSON to turn directory
	if err := m.writeReferenceFilesNDJSON(); err != nil {
		m.debugLogger.LogError("Failed to write reference files", err)
		return fmt.Errorf("failed to write reference files: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Reference files written successfully")

	// Copy methodology files to turn directory
	if turnType == "discovery" {
		discoverySrc := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "discovery.md")
		discoveryDest := filepath.Join(m.config.GetTurnDir(turn), "discovery.md")
		if err := copyFile(discoverySrc, discoveryDest); err != nil {
			m.debugLogger.LogError("Failed to copy discovery methodology", err)
			return fmt.Errorf("failed to copy discovery methodology: %w", err)
		}
		m.debugLogger.Log("DEBUG", "Discovery methodology copied successfully")
	} else {
		verificationSrc := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "verification.md")
		verificationDest := filepath.Join(m.config.GetTurnDir(turn), "verification.md")
		if err := copyFile(verificationSrc, verificationDest); err != nil {
			m.debugLogger.LogError("Failed to copy verification methodology", err)
			return fmt.Errorf("failed to copy verification methodology: %w", err)
		}
		m.debugLogger.Log("DEBUG", "Verification methodology copied successfully")
	}

	// Build and write combined system prompt
	systemPrompt, err := buildCombinedSystemPrompt(gscHome, turnType)
	if err != nil {
		m.debugLogger.LogError("Failed to build combined system prompt", err)
		return fmt.Errorf("failed to build combined system prompt: %w", err)
	}
	
	systemPromptFile := filepath.Join(m.config.GetTurnDir(turn), "system-prompt.md")
	if err := os.WriteFile(systemPromptFile, []byte(systemPrompt), 0644); err != nil {
		m.debugLogger.LogError("Failed to write system prompt", err)
		return fmt.Errorf("failed to write system prompt: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("System prompt written to: %s", systemPromptFile))

	// Write task prompt from template
	workdirsMarkdown := m.formatWorkingDirectories()
	refFilesMarkdown := m.formatReferenceFilesMetadata()
	if err := writeTaskPrompt(m, m.config.GetTurnDir(turn), turn, workdirsMarkdown, refFilesMarkdown, turnType); err != nil {
		m.debugLogger.LogError("Failed to write task prompt", err)
		return fmt.Errorf("failed to write task prompt: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Task prompt written successfully")

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Windows: Write prompt to file and use @file syntax
		m.debugLogger.Log("DEBUG", "Using Windows file-based approach for prompt")

		// Build claude command
		args := []string{
			"--allowedTools", "Read,Bash",
			"--verbose",
			"--include-partial-messages",
			"--output-format", "stream-json",
			"--append-system-prompt-file", systemPromptFile,
		}
		
		// Add add-dir flags
		for _, wd := range m.session.WorkingDirectories {
			args = append(args, "--add-dir", wd.Path)
		}
		
		if m.session.Model != "" {
			args = append(args, "--model", m.session.Model)
		}
		
		// Read task.md and pass as string
		taskPath := filepath.Join(m.config.GetTurnDir(turn), "task.md")
		taskContent, err := os.ReadFile(taskPath)
		if err != nil {
			m.debugLogger.LogError("Failed to read task prompt", err)
			m.markAsStopped("TASK_READ_FAILED", fmt.Sprintf("Failed to read task prompt: %v", err))
			return fmt.Errorf("failed to read task prompt: %w", err)
		}
		args = append(args, "-p", fmt.Sprintf("%q", string(taskContent)))

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

		// Read task.md for the script
		taskPath := filepath.Join(m.config.GetTurnDir(turn), "task.md")
		taskContent, err := os.ReadFile(taskPath)
		if err != nil {
			m.debugLogger.LogError("Failed to read task prompt", err)
			m.markAsStopped("TASK_READ_FAILED", fmt.Sprintf("Failed to read task prompt: %v", err))
			return fmt.Errorf("failed to read task prompt: %w", err)
		}

		// Create bash script content using heredoc for -p flag
		// Add gsc directory to PATH
		scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

# Set Claude Code file reading max tokens (default: 15000, user-overridable)
export CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS="${CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS:-%d}"

# Add gsc directory to PATH
export PATH="%s:$PATH"

# Verify gsc is available
if ! command -v gsc &> /dev/null; then
    echo "ERROR: gsc command not found in PATH"
    echo "Current PATH: $PATH"
    echo "Expected gsc at: %s"
    exit 1
fi

echo "=== Starting Claude Scout subprocess ==="
echo "Working directory: $(pwd)"
echo "Turn: %d"
echo "Session ID: %s"
echo "gsc location: $(which gsc)"
echo "=== Executing Claude command ==="

claude --allowedTools Read,Bash \
--verbose \
--include-partial-messages \
--output-format stream-json \
--append-system-prompt-file system-prompt.md \
%s \
%s \
-p <<'EOF'
%s
EOF

echo "=== Claude subprocess completed ==="
exit_code=$?
echo "Exit code: $exit_code"
exit $exit_code
`, defaultClaudeFileReadMaxTokens, gscDir, gscPath, turn, m.session.SessionID, addDirFlagsStr, modelFlag, string(taskContent))

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
		
		// Update turn state when process exits naturally
		completedAt := time.Now()
		if m.session != nil {
			// Find the current turn and update its state
			for i := range m.session.Turns {
				if m.session.Turns[i].TurnNumber == m.currentTurn {
					m.session.Turns[i].Status = "complete"
					m.session.Turns[i].CompletedAt = &completedAt
					m.session.Turns[i].ProcessInfo.Running = false
					m.session.Turns[i].ProcessInfo.PID = cmd.Process.Pid
					
					// Set error if process exited with non-zero code
					if exitCode != 0 {
						errorMsg := fmt.Sprintf("Process exited with code %d", exitCode)
						m.session.Turns[i].Error = &errorMsg
					}
					break
				}
			}
			
			// Update overall session status
			m.session.Status = "stopped"
			m.session.CompletedAt = &completedAt
		}
		if m.processInfo != nil {
			m.processInfo.Running = false
		}
		m.writeSessionState()
		
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
		if len(m.session.Turns) > 0 {
			lastTurn := m.session.Turns[len(m.session.Turns)-1]
			if lastTurn.TurnType == "verification" {
				phase = "verification"
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
	refPath := filepath.Join(turnDir, "references.ndjson")
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
	sb.WriteString("The following reference files have been included:\n")
	for i, ref := range m.session.ReferenceFilesContext {
		sb.WriteString(fmt.Sprintf("- reference-file-%03d: %s (chat-id: %d, repo: %s)\n",
			i+1, ref.RelativePath, ref.ChatID, ref.Repository))
	}
	sb.WriteString("\n**Note:** Complete reference file data is available in `references.ndjson` if you need to examine raw content.\n")
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

// buildCombinedSystemPrompt reads and combines shared + turn-specific prompts with embedded tool capabilities
func buildCombinedSystemPrompt(gscHome string, turnType string) (string, error) {
	// Read shared prompt
	sharedPath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "system_prompt_shared.md")
	sharedContent, err := os.ReadFile(sharedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read shared system prompt: %w", err)
	}
	
	// Read turn-specific prompt
	var turnPromptPath string
	if turnType == "discovery" {
		turnPromptPath = filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "system_prompt_discovery.md")
	} else {
		turnPromptPath = filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "system_prompt_verification.md")
	}
	turnContent, err := os.ReadFile(turnPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read turn-specific system prompt: %w", err)
	}
	
	// Read tool capabilities
	toolCapabilitiesPath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "tool_capabilities.md")
	toolCapabilitiesContent, err := os.ReadFile(toolCapabilitiesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read tool capabilities: %w", err)
	}
	
	// Combine with tool capabilities embedded
	combined := fmt.Sprintf(`# Scout System Prompt

This file combines shared principles with turn-specific instructions.

## Tool Capabilities

%s

---

# Shared Principles

%s

---

# %s Mission

%s
`, string(toolCapabilitiesContent), string(sharedContent), turnType, string(turnContent))
	
	return combined, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// writeTaskPrompt writes the task prompt from template
func writeTaskPrompt(m *Manager, turnDir string, turn int, workdirsMarkdown string, refFilesMarkdown string, turnType string) error {
	// Get GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Read task template
	var templatePath string
	if turnType == "discovery" {
		templatePath = filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "task_discovery.md")
	} else {
		templatePath = filepath.Join(gscHome, settings.ClaudeTemplatesPath, "scout", "task_verification.md")
	}

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read task template: %w", err)
	}

	// Read intent.md file from turn directory
	var intentContent []byte
	
	// For verification turns, use session intent (intent.md is only written for discovery turns)
	if turnType == "verification" {
		intentContent = []byte(m.session.Intent)
	} else {
		// For discovery turns, read from intent.md file
		intentPath := filepath.Join(turnDir, "intent.md")
		intentContent, err = os.ReadFile(intentPath)
		if err != nil {
			return fmt.Errorf("failed to read intent: %w", err)
		}
	}
	
	// Read turn-history.json if it exists
	var turnHistoryJSON string
	var turnHistoryExists bool
	
	// Read selected-candidates.json if it exists (for selective verification)
	var reviewFilesJSON string
	var hasReviewFiles bool
	
	selectedCandPath := filepath.Join(turnDir, "selected-candidates.json")
	if data, err := os.ReadFile(selectedCandPath); err == nil {
		var selectedCands SelectedCandidates
		if err := json.Unmarshal(data, &selectedCands); err == nil {
			var fullPaths []string
			for _, cand := range selectedCands.Selected {
				// Find the workdir to resolve the full path
				for _, wd := range m.session.WorkingDirectories {
					if wd.ID == cand.WorkdirID {
						fullPath := filepath.Join(wd.Path, cand.FilePath)
						fullPaths = append(fullPaths, fullPath)
						break
					}
				}
			}
			if len(fullPaths) > 0 {
				reviewFilesBytes, _ := json.MarshalIndent(fullPaths, "", "  ")
				reviewFilesJSON = string(reviewFilesBytes)
				hasReviewFiles = true
			}
		}
	}
	
	turnHistoryPath := filepath.Join(turnDir, "turn-history.json")
	if data, err := os.ReadFile(turnHistoryPath); err == nil {
		turnHistoryJSON = string(data)
		turnHistoryExists = true
	}
	
	// Create template and execute
	tmpl, err := template.New("task").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse task template: %w", err)
	}
	
	var buf bytes.Buffer
	data := struct {
		Workdirs         string
		RefFiles         string
		Intent           string
		TurnType         string
		TurnHistoryExists bool
		TurnHistoryJSON  string
		ReviewFilesJSON  string
		HasReviewFiles   bool
	}{
		Workdirs:         workdirsMarkdown,
		RefFiles:         refFilesMarkdown,
		Intent:           string(intentContent),
		TurnType:         turnType,
		TurnHistoryExists: turnHistoryExists,
		TurnHistoryJSON:  turnHistoryJSON,
		ReviewFilesJSON:  reviewFilesJSON,
		HasReviewFiles:   hasReviewFiles,
	}
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute task template: %w", err)
	}
	
	// Write to task.md
	taskPath := filepath.Join(turnDir, "task.md")
	return os.WriteFile(taskPath, buf.Bytes(), 0644)
}
