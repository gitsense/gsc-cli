/**
 * Component: Scout CLI Start Command
 * Block-UUID: 55eec20b-639c-41bc-a5b2-0285ffb2651b
 * Parent-UUID: 51539f39-9175-4f8e-9d2b-10e0ca08d80a
 * Version: 1.3.1
 * Description: Implements 'gsc claude scout start' command with turn-aware session handling
 * Language: Go
 * Created-at: 2026-04-01T05:31:57.927Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1)
 */


package scoutcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
	"github.com/google/uuid"
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

// StartCmd creates the "scout start" subcommand
func StartCmd() *cobra.Command {
	flags := &StartFlags{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new Scout discovery session",
		Long: `Start a new fire-and-forget Scout session to discover relevant files.

The Scout will:
1. Search working directories using gsc insights and gsc grep
2. Discover candidate files using the Tiny Overview brain
3. Score and rank candidates by relevance
4. Optionally proceed to verification (re-scoring with Claude)

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
	if err := ValidateScoutFlags(cmd); err != nil {
		cmd.SilenceUsage = true
		return err
	}

	// Validate flags
	if err := ValidateStartFlags(flags); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid flags: %w", err)
	}

	// EARLY VALIDATION: Reject turns > 2 for now
	if flags.Turn > 2 {
		cmd.SilenceUsage = true
		return fmt.Errorf(
			"Turn %d is not yet supported. Currently only Turn 1 (discovery) and Turn 2 (verification) are implemented.\n"+
				"Turn 1: gsc claude scout start --turn 1 ...\n"+
				"Turn 2: gsc claude scout start --turn 2 ... (after Turn 1 completes)",
			flags.Turn,
		)
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

	// Generate a unique session ID and handle turn-aware session logic
	sessionID := flags.SessionID
	if sessionID == "" {
		// Auto-generate if not provided
		sessionID = uuid.New().String()[:12]
	} else {
		// Validate session ID format (already done in ValidateStartFlags)
		// Check if session already exists
		config, _ := claudescout.NewSessionConfig(sessionID)

		// TURN-AWARE SESSION HANDLING
		if flags.Turn == 1 {
			// Turn 1: Error if session exists (unless --force)
			if config.SessionExists() {
				if !flags.Force {
					cmd.SilenceUsage = true
					return fmt.Errorf("session '%s' already exists. Use --force to overwrite", sessionID)
				}
				// Delete existing session only for Turn 1
				if err := config.CleanupSessionDir(); err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to cleanup existing session: %w", err)
				}
			}
		} else if flags.Turn == 2 {
			// Turn 2: Session must exist (will load it)
			if !config.SessionExists() {
				cmd.SilenceUsage = true
				return fmt.Errorf(
					"session '%s' not found. Please run Turn 1 discovery first:\n"+
						"  gsc claude scout start --session-id %s --turn 1 --intent-file <intent-file> --workdir <workdir>",
					sessionID, sessionID,
				)
			}
			// For Turn 2, load existing session instead of creating new
			tempManager, err := claudescout.LoadSession(sessionID)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to load existing session: %w", err)
			}

			// Validate Turn 1 is complete
			lastCompleted, err := tempManager.GetLastCompletedTurn()
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to check session status: %w", err)
			}
			if lastCompleted < 1 {
				cmd.SilenceUsage = true
				return fmt.Errorf(
					"Turn 1 discovery has not completed yet. Check status with:\n"+
						"  gsc claude scout status -s %s",
					sessionID,
				)
			}
		}
	}

	// Parse working directories and reference files
	workdirs, err := ParseWorkdirs(flags.WorkingDirectories)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to parse working directories: %w", err)
	}

	refFilesContext, err := ParseReferenceFilesNDJSON(flags.ReferenceFilesJSON)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to parse reference files: %w", err)
	}

	// Create or load scout manager based on turn
	var manager *claudescout.Manager
	if flags.Turn == 1 {
		// Turn 1: Create new manager with debug logging enabled if requested
		var err error
		manager, err = claudescout.NewManagerWithDebug(sessionID, flags.Debug)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create scout manager: %w", err)
		}

		// Initialize the session for Turn 1
		if err := manager.InitializeSession(intent, workdirs, refFilesContext, flags.AutoReview, flags.Model); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to initialize session: %w", err)
		}
	} else if flags.Turn == 2 {
		// Turn 2: Load existing manager
		var err error
		manager, err = claudescout.LoadSession(sessionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to load session: %w", err)
		}
		// Turn 2 will use existing session data from Turn 1
	}

	// Execute based on turn
	switch flags.Turn {
	case 1:
		if err := manager.StartTurn1Discovery(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to start discovery: %w", err)
		}
	case 2:
		// For Turn 2, start verification with existing session
		if err := manager.StartTurn2Verification(nil); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to start verification: %w", err)
		}
	default:
		// Should not reach here due to early validation, but keep as safety net
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid turn: %d (must be 1 or 2)", flags.Turn)
	}

	// Spawn background worker with --watch-worker flag
	if err := spawnBackgroundWorker(flags); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to spawn background worker: %w", err)
	}

	// Get process info for JSON output
	var processPID int
	if flags.Format == "json" {
		status, err := manager.GetSessionStatus()
		if err == nil && status.ProcessInfo.Running {
			processPID = status.ProcessInfo.PID
		}
	}

	// Output based on format
	if flags.Format == "json" {
		response := StartResponse{
			SessionID:  sessionID,
			Turn:       flags.Turn,
			Status:     "in_progress",
			ProcessPID: processPID,
			Message:    "Scout session started successfully",
		}

		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Existing text output
		fmt.Fprintf(cmd.OutOrStdout(), "Scout session started\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", FormatSessionPath(sessionID))
		fmt.Fprintf(cmd.OutOrStdout(), "Turn: %d\n", flags.Turn)

		if flags.Turn == 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "\nWhen discovery completes, proceed to verification with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout start --session-id %s --turn 2\n", sessionID)
		} else if flags.Turn == 2 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor verification progress with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nStop the session with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout stop -s %s\n", sessionID)
	}

	return nil
}

// spawnBackgroundWorker spawns a background worker process to handle the scout session
func spawnBackgroundWorker(flags *StartFlags) error {
	// Build args for worker
	args := []string{"claude", "scout", "start"}
	args = append(args, "--session-id", flags.SessionID)
	args = append(args, "--watch-worker")
	args = append(args, "--turn", fmt.Sprintf("%d", flags.Turn))

	// Spawn worker in background
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Detach from parent process
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn worker: %w", err)
	}

	// Store worker PID in session state
	manager, err := claudescout.LoadSession(flags.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load session to store watcher PID: %w", err)
	}
	manager.SetWatcherPID(cmd.Process.Pid)
	if err := manager.WriteSessionState(); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	return nil
}

// runBackgroundWorker executes the scout session in the background worker process
func runBackgroundWorker(cmd *cobra.Command, flags *StartFlags) error {
	// Load existing session
	manager, err := claudescout.LoadSession(flags.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Execute the turn (this blocks until complete)
	if flags.Turn == 1 {
		return manager.StartTurn1Discovery()
	} else {
		return manager.StartTurn2Verification(nil)
	}
}

// ParseWorkdirs converts working directory strings to WorkingDirectory structs
func ParseWorkdirs(paths []string) ([]claudescout.WorkingDirectory, error) {
	workdirs := make([]claudescout.WorkingDirectory, len(paths))

	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve working directory path %s: %w", path, err)
		}

		workdirs[i] = claudescout.WorkingDirectory{
			ID:   i + 1,
			Name: filepath.Base(absPath),
			Path: absPath,
		}
	}

	return workdirs, nil
}

// ParseReferenceFilesNDJSON reads and parses an NDJSON file containing reference files
func ParseReferenceFilesNDJSON(filePath string) ([]claudescout.ReferenceFileContext, error) {
	if filePath == "" {
		return []claudescout.ReferenceFileContext{}, nil // Reference files are optional
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open reference files: %w", err)
	}
	defer file.Close()

	var refFilesContext []claudescout.ReferenceFileContext
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var ref claudescout.ReferenceFileContext
		if err := json.Unmarshal(scanner.Bytes(), &ref); err != nil {
			return nil, fmt.Errorf("invalid reference file line: %w", err)
		}
		refFilesContext = append(refFilesContext, ref)
	}

	if err := scanner.Err(); err != nil {
		if err != nil {
			return nil, fmt.Errorf("error reading reference files: %w", err)
		}
	}

	return refFilesContext, nil
}

// FormatSessionPath returns a user-friendly session path for display
func FormatSessionPath(sessionID string) string {
	return fmt.Sprintf("scout:%s", strings.TrimSpace(sessionID))
}

// GetSessionShortID returns a shortened session ID for display
func GetSessionShortID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}
