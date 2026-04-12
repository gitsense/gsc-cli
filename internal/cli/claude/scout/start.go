/**
 * Component: Scout CLI Start Command
 * Block-UUID: d8f593dd-7ffb-4958-a8b8-2bab0cfb2432
 * Parent-UUID: 917988e1-2083-44b2-9346-60cb95cb8e23
 * Version: 1.7.0
 * Description: Implements 'gsc claude scout start' command with turn-type aware session handling. Supports multiple discovery turns followed by verification. Handles session creation, loading, and background worker spawning for both discovery and verification phases.
 * Language: Go
 * Created-at: 2026-04-12T03:17:22.362Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), claude-haiku-4-5-20251001 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0)
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
2. Discover candidate files using the Code Intent brain
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
	sessionID := flags.Session
	if sessionID == "" {
		// Auto-generate if not provided
		sessionID = uuid.New().String()[:12]
	} else {
		// Validate session ID format (already done in ValidateStartFlags)
		// Check if session already exists
		config, _ := claudescout.NewSessionConfig(sessionID)

		// TURN-TYPE-AWARE SESSION HANDLING
		if flags.TurnType == "discovery" {
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
		} else if flags.TurnType == "verification" {
			// Verification: Session must exist (will load it)
			if !config.SessionExists() {
				cmd.SilenceUsage = true
				return fmt.Errorf(
					"session '%s' not found. Please run discovery turn first:\n"+
						"  gsc claude scout start --session-id %s --turn-type discovery --intent-file <intent-file> --workdir <workdir>",
					sessionID, sessionID,
				)
			}
			// For verification, load existing session instead of creating new
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
					"Discovery turn has not completed yet. Check status with:\n"+
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
	if flags.TurnType == "discovery" {
		// Discovery: Create new manager with debug logging enabled if requested
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
	} else if flags.TurnType == "verification" {
		// Verification: Load existing manager
		var err error
		manager, err = claudescout.LoadSession(sessionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to load session: %w", err)
		}
		// Verification will use existing session data from discovery
	}

	// Spawn background worker with --watch-worker flag
	// The background worker will execute the turn (StartDiscoveryTurn or StartVerificationTurn)
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
			Turn:       1, // Will be updated from session state
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
		fmt.Fprintf(cmd.OutOrStdout(), "Turn Type: %s\n", flags.TurnType)

		if flags.TurnType == "discovery" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "\nWhen discovery completes, proceed to verification with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout start --session-id %s --turn-type verification\n", sessionID)
		} else if flags.TurnType == "verification" {
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
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")
	args = append(args, "--turn-type", flags.TurnType)
	if flags.ReviewFiles != "" {
		args = append(args, "--review-files", flags.ReviewFiles)
	}

	// Spawn worker in background
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Detach from parent process
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn worker: %w", err)
	}

	// Store worker PID in session state
	manager, err := claudescout.LoadSession(flags.Session)
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
	manager, err := claudescout.LoadSession(flags.Session)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Parse review files if provided for verification
	var selectedCandidates *claudescout.SelectedCandidates
	if flags.TurnType == "verification" && flags.ReviewFiles != "" {
		data, err := os.ReadFile(flags.ReviewFiles)
		if err != nil {
			return fmt.Errorf("failed to read review files: %w", err)
		}
		var cand claudescout.SelectedCandidates
		if err := json.Unmarshal(data, &cand); err != nil {
			return fmt.Errorf("failed to parse review files: %w", err)
		}
		selectedCandidates = &cand
	}

	// Execute the turn (this blocks until complete)
	if flags.TurnType == "discovery" {
		return manager.StartDiscoveryTurn()
	} else {
		return manager.StartVerificationTurn(selectedCandidates)
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
