/*
 * Component: Agent Helper Utilities
 * Block-UUID: 7ab9a320-c18f-48f9-84a3-13d9c8bf1b6b
 * Parent-UUID: 5bd851a1-7b2f-4bc5-83a5-3fffb6008a1d
 * Version: 1.1.0
 * Description: Shared utilities for Claude-based agentic commands in the CLI. Provides common functionality for intent resolution, working directory parsing, background worker spawning, and session initialization. Designed to be reusable across intent workflow, chat, expert, and other future Claude-based features.
 * Language: Go
 * Created-at: 2026-04-30T15:20:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package shared

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
)

// StartResponse represents a standard JSON response for starting any background agent task
type StartResponse struct {
	SessionID  string `json:"session_id"`
	Turn       int    `json:"turn,omitempty"` // Optional: only for workflow-based agents
	Status     string `json:"status"`
	ProcessPID int    `json:"process_pid,omitempty"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
}

// FlagError represents a flag validation error
type FlagError struct {
	Flag    string
	Message string
}

// Error implements the error interface
func (e *FlagError) Error() string {
	return "flag " + e.Flag + ": " + e.Message
}

// FormatSessionLabel returns a user-friendly label like "scout:abc123" or "chat:xyz789"
func FormatSessionLabel(prefix, sessionID string) string {
	return fmt.Sprintf("%s:%s", prefix, strings.TrimSpace(sessionID))
}

// ResolveIntent handles reading intent from string or file
func ResolveIntent(intent, intentFile string) (string, error) {
	if intentFile != "" {
		content, err := os.ReadFile(intentFile)
		if err != nil {
			return "", fmt.Errorf("failed to read intent file: %w", err)
		}
		return string(content), nil
	}
	return intent, nil
}

// ParseWorkdirs is a generic utility for resolving multiple working directories
func ParseWorkdirs(paths []string) ([]intent_workflow.WorkingDirectory, error) {
	workdirs := make([]intent_workflow.WorkingDirectory, len(paths))
	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
		}
		workdirs[i] = intent_workflow.WorkingDirectory{
			ID:   i + 1,
			Name: filepath.Base(absPath),
			Path: absPath,
		}
	}
	return workdirs, nil
}

// ParseReferenceFilesNDJSON reads and parses an NDJSON file of reference files
func ParseReferenceFilesNDJSON(filePath string) ([]intent_workflow.ReferenceFileContext, error) {
	if filePath == "" {
		return []intent_workflow.ReferenceFileContext{}, nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open reference files: %w", err)
	}
	defer file.Close()

	var refs []intent_workflow.ReferenceFileContext
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var ref intent_workflow.ReferenceFileContext
		if err := json.Unmarshal(scanner.Bytes(), &ref); err != nil {
			return nil, fmt.Errorf("invalid reference file line: %w", err)
		}
		refs = append(refs, ref)
	}
	return refs, scanner.Err()
}

// SpawnBackgroundWorker is a generic utility to detach a process and record its PID
func SpawnBackgroundWorker(args []string, sessionID string) (int, error) {
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = newSysProcAttr()

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to spawn worker: %w", err)
	}

	// Wait a moment to ensure it didn't immediately crash
	time.Sleep(100 * time.Millisecond)
	process, err := os.FindProcess(cmd.Process.Pid)
	if err != nil {
		return 0, fmt.Errorf("failed to find worker process: %w", err)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, fmt.Errorf("worker process died immediately")
	}

	// Store the worker PID in the session state
	manager, err := intent_workflow.LoadSession(sessionID)
	if err == nil {
		manager.SetWatcherPID(cmd.Process.Pid)
		_ = manager.WriteSessionState()
	}

	return cmd.Process.Pid, nil
}

// InitWorkerSession handles the boilerplate for background worker startup
func InitWorkerSession(sessionID string, debugEnabled bool) (*intent_workflow.Manager, *intent_workflow.DebugLogger, error) {
	config, err := intent_workflow.NewSessionConfig(sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session config: %w", err)
	}

	debugLogger, err := intent_workflow.NewDebugLogger(config.GetSessionDir(), debugEnabled)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create debug log: %w", err)
	}

	manager, err := intent_workflow.LoadSession(sessionID)
	if err != nil {
		debugLogger.Close()
		return nil, nil, fmt.Errorf("failed to load session: %w", err)
	}

	return manager, debugLogger, nil
}
