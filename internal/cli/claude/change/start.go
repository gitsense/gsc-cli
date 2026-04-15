/**
 * Component: Change CLI Start Command
 * Block-UUID: aa473d5e-3aff-4888-b3c2-21cc3c17e56c
 * Parent-UUID: 7a8f9c2d-3e4f-4a5b-9c6d-7e8f9a0b1c2d
 * Version: 1.1.0
 * Description: Implements 'gsc claude change start' command for in-place code editing with git diff generation. Updated to import from agent package instead of scout.
 * Language: Go
 * Created-at: 2026-04-15T04:08:45.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package change

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	agent "github.com/gitsense/gsc-cli/internal/claude/agent"
	"github.com/spf13/cobra"
)

// StartResponse represents the JSON response for the start command
type StartResponse struct {
	SessionID  string `json:"session_id"`
	Turn       int    `json:"turn"`
	Status     string `json:"status"`
	ProcessPID int    `json:"process_pid,omitempty"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
}

// StartCmd creates the "change start" subcommand
func StartCmd() *cobra.Command {
	flags := &StartFlags{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a change turn for in-place code editing",
		Long: `Start a change turn to apply code changes based on verified discovery results.

The change turn will:
1. Validate that verification is complete
2. Spawn Claude subprocess to edit files in place
3. Generate git diffs for each working directory
4. Write result.json with change summary and git diffs

The session runs as a background subprocess and can be monitored with 'gsc claude scout status'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStartCommand(cmd, flags)
		},
	}

	RegisterStartFlags(cmd, flags)

	return cmd
}

// runStartCommand executes the start command logic
func runStartCommand(cmd *cobra.Command, flags *StartFlags) error {
	// If --watch-worker flag is set, this is the background process
	if flags.WatchWorker {
		return runBackgroundWorker(cmd, flags)
	}

	// Validate that unsupported flags are not set
	if err := ValidateChangeFlags(cmd); err != nil {
		cmd.SilenceUsage = true
		return err
	}

	// Validate flags
	if err := ValidateStartFlags(flags); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Determine intent (from flag or file)
	intent := flags.Intent
	if flags.IntentFile != "" {
		content, err := os.ReadFile(flags.IntentFile)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to read intent file: %w", err)
		}
		intent = string(content)
	}

	// Session ID is required for change turns
	sessionID := flags.Session
	if sessionID == "" {
		cmd.SilenceUsage = true
		return fmt.Errorf("--session is required for change turns")
	}

	// Create session config to check if session exists
	config, _ := agent.NewSessionConfig(sessionID)

	// Change turn requires existing session
	if !config.SessionExists() {
		cmd.SilenceUsage = true
		return fmt.Errorf(
			"session '%s' not found. Please run discovery and verification turns first:\n"+
				"  gsc claude scout start --session-id %s --turn-type discovery --intent-file <intent-file> --workdir <workdir>\n"+
				"  gsc claude scout start --session-id %s --turn-type verification",
			sessionID, sessionID, sessionID,
		)
	}

	// Load existing session
	manager, err := agent.LoadSession(sessionID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Validate session status
	status, err := manager.GetSessionStatus()
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to get session status: %w", err)
	}

	if status.Status != "verification_complete" {
		cmd.SilenceUsage = true
		return fmt.Errorf(
			"session status is %s, expected verification_complete. Please complete verification first:\n"+
				"  gsc claude scout start --session-id %s --turn-type verification",
			status.Status, sessionID,
		)
	}

	// Spawn background worker with --watch-worker flag
	workerPID, err := spawnBackgroundWorker(flags)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to spawn background worker: %w", err)
	}

	// Output based on format
	if flags.Format == "json" {
		response := StartResponse{
			SessionID:  sessionID,
			Turn:       manager.GetNextTurnNumber(),
			Status:     "in_progress",
			ProcessPID: workerPID,
			Message:    "Change turn started successfully",
		}

		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Existing text output
		fmt.Fprintf(cmd.OutOrStdout(), "Change turn started\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", FormatSessionPath(sessionID))
		fmt.Fprintf(cmd.OutOrStdout(), "Intent: %s\n", truncateForDisplay(intent, 100))

		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)

		fmt.Fprintf(cmd.OutOrStdout(), "\nStop the session with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude change stop -s %s\n", sessionID)
	}

	return nil
}

// spawnBackgroundWorker spawns a background worker process to handle the change turn
// Returns the worker PID immediately (non-blocking)
func spawnBackgroundWorker(flags *StartFlags) (int, error) {
	// Build args for worker
	args := []string{"claude", "change", "start"}
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")
	if flags.Intent != "" {
		args = append(args, "--intent", flags.Intent)
	}
	if flags.IntentFile != "" {
		args = append(args, "--intent-file", flags.IntentFile)
	}

	// Spawn worker in background
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Detach from parent process
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to spawn worker: %w", err)
	}

	// Wait a moment and check if process is still alive
	time.Sleep(100 * time.Millisecond)
	process, err := os.FindProcess(cmd.Process.Pid)
	if err != nil {
		return 0, fmt.Errorf("failed to find worker process: %w", err)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process died immediately
		return 0, fmt.Errorf("worker process died immediately (check debug.log for details)")
	}

	// Store worker PID in session state
	manager, err := agent.LoadSession(flags.Session)
	if err != nil {
		return cmd.Process.Pid, fmt.Errorf("failed to load session to store watcher PID: %w", err)
	}
	manager.SetWatcherPID(cmd.Process.Pid)
	if err := manager.WriteSessionState(); err != nil {
		return cmd.Process.Pid, fmt.Errorf("failed to write session state: %w", err)
	}

	return cmd.Process.Pid, nil
}

// runBackgroundWorker executes the change turn in the background worker process
func runBackgroundWorker(cmd *cobra.Command, flags *StartFlags) error {
	// Create debug log immediately
	config, err := agent.NewSessionConfig(flags.Session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create session config: %v\n", err)
		return fmt.Errorf("failed to create session config: %w", err)
	}

	debugLogger, err := agent.NewDebugLogger(config.GetSessionDir(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create debug log: %v\n", err)
		return fmt.Errorf("failed to create debug log: %w", err)
	}
	defer debugLogger.Close()

	debugLogger.Log("WORKER", "Background worker started")
	debugLogger.Log("WORKER", fmt.Sprintf("Session ID: %s", flags.Session))
	debugLogger.Log("WORKER", "Change turn")

	// Load existing session
	debugLogger.Log("WORKER", "Loading session...")
	manager, err := agent.LoadSession(flags.Session)
	if err != nil {
		debugLogger.LogError("Failed to load session", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load session: %v\n", err)
		return fmt.Errorf("failed to load session: %w", err)
	}
	debugLogger.Log("WORKER", "Session loaded successfully")

	// Validate session state
	debugLogger.Log("WORKER", "Getting session status...")
	status, err := manager.GetSessionStatus()
	if err != nil {
		debugLogger.LogError("Failed to get session status", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get session status: %v\n", err)
		return fmt.Errorf("failed to get session status: %w", err)
	}
	debugLogger.Log("WORKER", fmt.Sprintf("Session status: %s", status.Status))

	// Validate session state for change
	if status.Status != "verification_complete" {
		err := fmt.Errorf("session status is %s, expected verification_complete for change turn", status.Status)
		debugLogger.LogError("Invalid session status", err)
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return err
	}

	debugLogger.Log("WORKER", fmt.Sprintf("Session has %d turns", len(status.Turns)))

	nextTurn := manager.GetNextTurnNumber()

	// Validate that the last turn is complete
	lastTurn := status.Turns[len(status.Turns)-1]
	debugLogger.Log("WORKER", fmt.Sprintf("Last turn: %d (type: %s, status: %s)",
		lastTurn.TurnNumber, lastTurn.TurnType, lastTurn.Status))

	if lastTurn.Status != "complete" {
		err := fmt.Errorf("last turn %d is not complete (status: %s)", lastTurn.TurnNumber, lastTurn.Status)
		debugLogger.LogError("Last turn not complete", err)
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return err
	}
	debugLogger.Log("WORKER", fmt.Sprintf("Starting Turn %d (previous turn complete)", nextTurn))

	// Determine intent
	intent := flags.Intent
	if flags.IntentFile != "" {
		content, err := os.ReadFile(flags.IntentFile)
		if err != nil {
			debugLogger.LogError("Failed to read intent file", err)
			fmt.Fprintf(os.Stderr, "ERROR: Failed to read intent file: %v\n", err)
			return fmt.Errorf("failed to read intent file: %w", err)
		}
		intent = string(content)
	}

	// Execute the change turn (this blocks until complete)
	debugLogger.Log("WORKER", "Starting change turn...")
	if err := manager.StartChangeTurn(intent); err != nil {
		debugLogger.LogError("Change turn failed", err)
		fmt.Fprintf(os.Stderr, "ERROR: Change turn failed: %v\n", err)
		return err
	}

	// Generate git diffs for each working directory
	debugLogger.Log("WORKER", "Generating git diffs...")
	gitDiffs := make(map[string]string)
	for _, wd := range status.WorkingDirectories {
		debugLogger.Log("WORKER", fmt.Sprintf("Generating diff for workdir: %s", wd.Path))
		diff, err := generateGitDiff(wd.Path)
		if err != nil {
			debugLogger.LogError("Failed to generate git diff", err)
			fmt.Fprintf(os.Stderr, "WARNING: Failed to generate git diff for %s: %v\n", wd.Path, err)
			gitDiffs[wd.Path] = fmt.Sprintf("Error generating diff: %v", err)
		} else {
			gitDiffs[wd.Path] = diff
			debugLogger.Log("WORKER", fmt.Sprintf("Generated diff for %s: %d bytes", wd.Path, len(diff)))
		}
	}

	// Write result.json with change summary and git diffs
	debugLogger.Log("WORKER", "Writing result.json...")
	if err := writeChangeResult(manager, nextTurn, gitDiffs); err != nil {
		debugLogger.LogError("Failed to write result.json", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to write result.json: %v\n", err)
		return fmt.Errorf("failed to write result.json: %w", err)
	}

	debugLogger.Log("WORKER", "Change turn completed successfully")
	return nil
}

// generateGitDiff generates a git diff for the specified working directory
func generateGitDiff(workdirPath string) (string, error) {
	cmd := exec.Command("git", "diff", "--no-color")
	cmd.Dir = workdirPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// writeChangeResult writes the change result JSON file
func writeChangeResult(manager *agent.Manager, turn int, gitDiffs map[string]string) error {
	// Get the turn results
	status, err := manager.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to get session status: %w", err)
	}

	// Find the current turn
	var currentTurn *agent.TurnState
	for i := range status.Turns {
		if status.Turns[i].TurnNumber == turn {
			currentTurn = &status.Turns[i]
			break
		}
	}

	if currentTurn == nil {
		return fmt.Errorf("turn %d not found", turn)
	}

	// Extract change results from turn state
	var changeResults *agent.ChangeResults
	if currentTurn.Results != nil && currentTurn.Results.ChangeResults != nil {
		changeResults = currentTurn.Results.ChangeResults
	} else {
		// Create empty change results if not present
		changeResults = &agent.ChangeResults{
			ChangeSummary: agent.ChangeSummary{
				TurnNumber:         turn,
				ChangeRequest:      manager.GetSession().Intent,
				FilesModifiedCount: 0,
				FilesModified:      []agent.FileMod{},
			},
			GitDiff: gitDiffs,
			Notes:   "",
			Errors:  "",
		}
	}

	// Update git diffs with generated diffs
	changeResults.GitDiff = gitDiffs

	// Write result.json
	resultPath := filepath.Join(manager.GetConfig().GetTurnDir(turn), "result.json")
	resultData, err := json.MarshalIndent(changeResults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change results: %w", err)
	}

	if err := os.WriteFile(resultPath, resultData, 0644); err != nil {
		return fmt.Errorf("failed to write result.json: %w", err)
	}

	return nil
}

// FormatSessionPath returns a user-friendly session path for display
func FormatSessionPath(sessionID string) string {
	return fmt.Sprintf("scout:%s", strings.TrimSpace(sessionID))
}

// truncateForDisplay truncates a string for display purposes
func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StartFlags contains flags for the change start command
type StartFlags struct {
	Intent     string
	IntentFile string
	Session    string
	Format     string
	WatchWorker bool
}

// RegisterStartFlags registers flags for the start command
func RegisterStartFlags(cmd *cobra.Command, flags *StartFlags) {
	cmd.Flags().StringVarP(
		&flags.Intent,
		"intent", "i",
		"",
		"The change request/intent for Claude to apply",
	)

	cmd.Flags().StringVarP(
		&flags.IntentFile,
		"intent-file", "f",
		"",
		"Read intent from a file (alternative to --intent)",
	)

	cmd.Flags().StringVar(
		&flags.Session,
		"session",
		"",
		"Session ID (required for change turns)",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
	)

	cmd.Flags().BoolVar(
		&flags.WatchWorker,
		"watch-worker",
		false,
		"Run as background worker process",
	)
	cmd.Flags().MarkHidden("watch-worker")
}

// ValidateStartFlags validates the start command flags
func ValidateStartFlags(flags *StartFlags) error {
	// Ensure either --intent or --intent-file is provided (but not both)
	intent := strings.TrimSpace(flags.Intent)
	if intent == "" && flags.IntentFile == "" {
		return &FlagError{Flag: "intent", Message: "either --intent or --intent-file is required"}
	}
	if intent != "" && flags.IntentFile != "" {
		return &FlagError{Flag: "intent", Message: "cannot specify both --intent and --intent-file"}
	}

	// Validate intent file exists if provided
	if flags.IntentFile != "" {
		if _, err := os.Stat(flags.IntentFile); err != nil {
			return &FlagError{Flag: "intent-file", Message: fmt.Sprintf("intent file not found: %s", flags.IntentFile)}
		}
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[flags.Format] {
		return &FlagError{Flag: "format", Message: "format must be 'text' or 'json'"}
	}

	return nil
}

// ValidateChangeFlags checks that unsupported flags are not set
func ValidateChangeFlags(cmd *cobra.Command) error {
	// Check --code flag (from root gsc command)
	if code, _ := cmd.Flags().GetString("code"); code != "" {
		return &FlagError{Flag: "code", Message: "--code flag is not supported for change commands"}
	}

	// Check --uuid flag (from gsc claude command)
	if uuid, _ := cmd.Flags().GetString("uuid"); uuid != "" {
		return &FlagError{Flag: "uuid", Message: "--uuid flag is not supported for change commands"}
	}

	// Check --parent-id flag (from gsc claude command)
	if parentID, _ := cmd.Flags().GetInt64("parent-id"); parentID != 0 {
		return &FlagError{Flag: "parent-id", Message: "--parent-id flag is not supported for change commands"}
	}

	return nil
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
