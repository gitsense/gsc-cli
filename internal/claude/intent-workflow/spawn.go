/**
 * Component: Intent Workflow Spawn
 * Block-UUID: 2655a789-1821-47bd-94f6-927a6929a398
 * Parent-UUID: 7a8f9c2d-4e5f-4a3b-8c7d-9e0f1a2b3c4d
 * Version: 1.9.0
 * Description: Updated template paths from data/templates to cli/templates to separate CLI-specific data from app-specific data.
 * Language: Go
 * Created-at: 2026-04-29T02:37:27.761Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.3.3), GLM-4.7 (v1.3.4), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0)
 */


package intent_workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/settings"
)


// spawnClaudeSubprocess spawns the claude subprocess for a turn
func (m *Manager) spawnClaudeSubprocess(turn int, turnType string, orphanedFiles []string) error {
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Spawning subprocess for turn %d", turn))

	// Get the Claude prompt template using absolute path
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		m.debugLogger.LogError("Failed to get GSC_HOME", err)
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}
	m.debugLogger.Log("DEBUG", fmt.Sprintf("GSC_HOME: %s", gscHome))

	// Define agent templates path
	agentTemplatesPath := filepath.Join(gscHome, "cli", "templates", "claude", "intent-workflow")

	// Find gsc location to add to PATH
	gscPath, err := exec.LookPath("gsc")
	if err != nil {
		m.debugLogger.LogError("Failed to find gsc in PATH", err)
		return fmt.Errorf("gsc not found in PATH: %w", err)
	}
	gscDir := filepath.Dir(gscPath)
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Found gsc at: %s", gscPath))
	m.debugLogger.Log("DEBUG", fmt.Sprintf("Adding to PATH: %s", gscDir))

	// Write agent permissions to restrict Bash to gsc commands only
	if err := m.WriteAgentPermissions(m.config.GetTurnDir(turn)); err != nil {
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

	// Copy methodology files to turn directory (only for discovery)
	if turnType == "discovery" {
		discoverySrc := filepath.Join(agentTemplatesPath, "discovery", "discovery.md")
		discoveryDest := filepath.Join(m.config.GetTurnDir(turn), "discovery.md")
		if err := m.copyFile(discoverySrc, discoveryDest); err != nil {
			m.debugLogger.LogError("Failed to copy discovery methodology", err)
			return fmt.Errorf("failed to copy discovery methodology: %w", err)
		}
		m.debugLogger.Log("DEBUG", "Discovery methodology copied successfully")
	}

	// Copy response format file to turn directory (for discovery and change turns)
	if turnType == "discovery" || turnType == "change" {
		formatSrc := getFormatFile(gscHome, turnType)
		if formatSrc == "" {
			m.debugLogger.Log("WARN", fmt.Sprintf("No format file defined for turn type %q", turnType))
		} else {
			formatDest := filepath.Join(m.config.GetTurnDir(turn), "response-format.md")
			if err := m.copyFile(formatSrc, formatDest); err != nil {
				m.debugLogger.LogError("Failed to copy response format", err)
				return fmt.Errorf("failed to copy response format: %w", err)
			}
			m.debugLogger.Log("DEBUG", "Response format copied successfully")
		}
	}

	// Copy change meta format file to turn directory (only for change turns)
	if turnType == "change" {
		changeMetaSrc := filepath.Join(agentTemplatesPath, "change", "change_meta_format.md")
		changeMetaDest := filepath.Join(m.config.GetTurnDir(turn), "change_meta_format.md")
		if err := m.copyFile(changeMetaSrc, changeMetaDest); err != nil {
			m.debugLogger.LogError("Failed to copy change meta format", err)
			return fmt.Errorf("failed to copy change meta format: %w", err)
		}
		m.debugLogger.Log("DEBUG", "Change meta format copied successfully")
	}

	// Copy resume task template to turn directory (only for resume turns)
	// Uses parseTurnType to support generic resume types (e.g., resume-change, resume-verify)
	baseType, isResume := parseTurnType(turnType)
	if isResume {
		resumeTaskSrc := filepath.Join(agentTemplatesPath, baseType, "resume_task.md")
		resumeTaskDest := filepath.Join(m.config.GetTurnDir(turn), "resume_task.md")

		// Check if the file already exists (it may have been rendered by StartResumeChangeTurn)
		if _, err := os.Stat(resumeTaskDest); os.IsNotExist(err) {
			// Only copy if it doesn't exist
			if err := m.copyFile(resumeTaskSrc, resumeTaskDest); err != nil {
				m.debugLogger.LogError("Failed to copy resume task", err)
				return fmt.Errorf("failed to copy resume task: %w", err)
			}
			m.debugLogger.Log("DEBUG", "Resume task copied successfully")
		} else {
			m.debugLogger.Log("DEBUG", "Resume task already exists, skipping copy")
		}
	}

	// Build and write combined system prompt
	systemPrompt, err := m.buildSystemPrompt(gscHome, turnType)
	if err != nil {
		m.debugLogger.LogError("Failed to build system prompt", err)
		return fmt.Errorf("failed to build system prompt: %w", err)
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

	if err := m.writePrompt(m.config.GetTurnDir(turn), turn, workdirsMarkdown, refFilesMarkdown, turnType, orphanedFiles); err != nil {
		m.debugLogger.LogError("Failed to write prompt", err)
		return fmt.Errorf("failed to write prompt: %w", err)
	}
	m.debugLogger.Log("DEBUG", "Prompt written successfully")

	// PRE-FLIGHT VERIFICATION: Ensure all required control files exist before spawning
	// This prevents race conditions where files may not be fully written to disk
	turnDir := m.config.GetTurnDir(turn)
	requiredFiles := []string{"task.md", "system-prompt.md"}
	
	if turnType == "discovery" {
		requiredFiles = append(requiredFiles, "discovery.md", "response-format.md")
	} else if turnType == "change" {
		requiredFiles = append(requiredFiles, "response-format.md", "change_meta_format.md")
	} else if isResume {
		// Resume turns need format files based on base type
		if baseType == "discovery" {
			requiredFiles = append(requiredFiles, "discovery.md", "response-format.md")
		} else if baseType == "change" {
			requiredFiles = append(requiredFiles, "response-format.md", "change_meta_format.md")
		}
	}
	
	m.debugLogger.Log("DEBUG", "Performing pre-flight file verification")
	for _, filename := range requiredFiles {
		filePath := filepath.Join(turnDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			m.debugLogger.LogError("Pre-flight check failed", fmt.Errorf("required file not found: %s", filename))
			m.markAsError("PREFLIGHT_FAILED", fmt.Sprintf("Required file missing before spawn: %s", filename))
			return fmt.Errorf("pre-flight check failed: %s not found in turn directory %s", filename, turnDir)
		}
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Verified: %s", filename))
	}
	m.debugLogger.Log("DEBUG", "Pre-flight verification complete")

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Windows: Write prompt to file and use @file syntax
		m.debugLogger.Log("DEBUG", "Using Windows file-based approach for prompt")

		// Build claude command
		args := []string{
			"--allowedTools", "Read,Write,Bash",
			"--verbose",
			"--dangerously-skip-permissions",
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
			m.markAsError("TASK_READ_FAILED", fmt.Sprintf("Failed to read task prompt: %v", err))
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
			m.markAsError("TASK_READ_FAILED", fmt.Sprintf("Failed to read task prompt: %v", err))
			return fmt.Errorf("failed to read task prompt: %w", err)
		}

		// Create bash script content using heredoc for -p flag
		// Add gsc directory to PATH
		// IMPORTANT: Trap SIGTERM to forward to Claude process for graceful shutdown
		scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

# Set Claude Code file reading max tokens (default: 15000, user-overridable)
export CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS="${CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS:-%d}"

# Add gsc directory to PATH
export PATH="%s:$PATH"

# Trap SIGTERM and forward to child process for graceful shutdown
trap 'echo "Received SIGTERM, forwarding to Claude process..."; kill -TERM $PID 2>/dev/null; wait $PID; exit 143' TERM

# Verify gsc is available
if ! command -v gsc &> /dev/null; then
    echo "ERROR: gsc command not found in PATH"
    echo "Current PATH: $PATH"
    echo "Expected gsc at: %s"
    exit 1
fi

echo "=== Starting Claude Agent subprocess ==="
echo "Working directory: $(pwd)"
echo "Turn: %d"
echo "Turn Type: %s"
echo "Session ID: %s"
echo "gsc location: $(which gsc)"
echo "=== Executing Claude command (PID: $$) ==="

claude --allowedTools Read,Bash,Write \
--verbose \
--dangerously-skip-permissions \
--include-partial-messages \
--output-format stream-json \
--append-system-prompt-file system-prompt.md \
%s \
%s \
-p <<'__GSC_PROMPT_END__' &
%s
__GSC_PROMPT_END__

PID=$!
echo "Claude process started with PID: $PID"
wait $PID
CLAUDE_EXIT_CODE=$?
echo "=== Claude subprocess completed with exit code: $CLAUDE_EXIT_CODE ==="
exit $CLAUDE_EXIT_CODE
`, defaultFileReadMaxTokens, gscDir, gscPath, turn, turnType, m.session.SessionID, addDirFlagsStr, modelFlag, string(taskContent))

		// Write bash script to turn directory
		scriptPath := filepath.Join(m.config.GetTurnDir(turn), "run-claude.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			m.debugLogger.LogError("Failed to write bash script", err)
			m.markAsError("SCRIPT_FAILED", fmt.Sprintf("Failed to write bash script: %v", err))
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
		m.markAsError("START_FAILED", fmt.Sprintf("Failed to start subprocess: %v", err))
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

	// CRITICAL FIX: Persist PID to session.json immediately
	// This allows StopSession() to recover the PID when called from CLI stop command
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == turn {
			m.session.Turns[i].ProcessInfo.PID = cmd.Process.Pid
			m.session.Turns[i].ProcessInfo.Command = cmd.String()
			break
		}
	}
	m.writeSessionState()

	// Start background goroutine to process stream
	m.debugLogger.Log("DEBUG", "Starting stream processing goroutine")
	m.wg.Add(1)
	go m.processStream(stdout, turn)

	// Start background goroutine to reap zombie process
	m.debugLogger.Log("DEBUG", "Starting process reaper goroutine")
	m.wg.Add(1)
	m.session.Stopped = false // Initialize stopped flag
	go func() {
		m.debugLogger.Log("DEBUG", "Process reaper: waiting for process to exit")
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				m.debugLogger.Log("DEBUG", fmt.Sprintf("Process reaper: process exited with code %d (stopped: %v)", exitCode, m.session.Stopped))
			}
		}
		m.debugLogger.LogProcessExit(cmd.Process.Pid, exitCode, err)

		// CRITICAL FIX: Check for .stopped file ONCE at the beginning of the goroutine
		// This fixes bug where file was checked three times and removed after first check,
		// causing session status to be incorrectly set to "error" instead of "stopped"
		stoppedFilePath := filepath.Join(m.config.GetTurnDir(m.currentTurn), ".stopped")
		wasStopped := false
		if _, err := os.Stat(stoppedFilePath); err == nil {
			wasStopped = true
		}

		// Delegate finalization to lifecycle manager
		m.finalizeTurn(exitCode, wasStopped)

		m.wg.Done()
	}()

	// Start background goroutine to capture stderr
	m.wg.Add(1)
	go m.captureStderr(stderr)
	return nil
}
