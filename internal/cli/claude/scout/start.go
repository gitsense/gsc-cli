/*
 * Component: Scout CLI Start Command
 * Block-UUID: c0f0143d-d746-40ef-b447-021a4248af77
 * Parent-UUID: 737f2699-e093-4294-8828-5d88ae039f68
 * Version: 1.19.0
 * Description: Implements 'gsc claude scout start' command for Intent Workflow with smart discovery (metadata search + code validation). Simplified to discovery-only mode with discovery → change workflow. Refactored to use shared agent helper utilities, removing duplicated code for intent resolution, working directory parsing, background worker spawning, and session initialization. Updated to pass flags.DisableExperts to InitializeSession to support --disable-experts flag for forcing generic discovery mode.
 * Language: Go
 * Created-at: 2026-04-13T14:04:01.074Z
 * Authors: claude-haiku-4-5-20251001 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), claude-haiku-4-5-20251001 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0), GLM-4.7 (v1.17.0), GLM-4.7 (v1.18.0), GLM-4.7 (v1.19.0)
 */


package scoutcli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/gitsense/gsc-cli/internal/cli/claude/shared"
	"github.com/spf13/cobra"
)

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
3. Validate candidates by reading actual code (score > 0.7)
4. Return validated results with reasoning and metadata

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

	// Resolve intent (from flag or file)
	intent, err := shared.ResolveIntent(flags.Intent, flags.IntentFile)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	// Generate a unique session ID and handle session logic
	sessionID := flags.Session
	if sessionID == "" {
		// Auto-generate if not provided
		sessionID = fmt.Sprintf("%012x", os.Getpid()) // Simple 12-char hex ID
	}

	// Create session config to check if session exists
	config, _ := intent_workflow.NewSessionConfig(sessionID)

	// Parse working directories and reference files
	workdirs, err := shared.ParseWorkdirs(flags.WorkingDirectories)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to parse working directories: %w", err)
	}

	refFilesContext, err := shared.ParseReferenceFilesNDJSON(flags.ReferenceFilesJSON)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to parse reference files: %w", err)
	}

	// Create or load scout manager
	var manager *intent_workflow.Manager
	
	if config.SessionExists() {
		// Session exists: load it (this is a subsequent discovery turn)
		var err error
		manager, err = intent_workflow.LoadSession(sessionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to load existing session: %w", err)
		}
	} else {
		// Session doesn't exist: create new (this is the first discovery turn)
		var err error
		manager, err = intent_workflow.NewManagerWithDebug(sessionID, flags.Debug)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create scout manager: %w", err)
		}
		
		// Initialize the session (only for new sessions)
		if err := manager.InitializeSession(intent, workdirs, refFilesContext, flags.AutoReview, flags.Model, flags.DisableExperts); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to initialize session: %w", err)
		}
	}

	// Spawn background worker with --watch-worker flag
	// The background worker will execute the discovery turn
	args := []string{"claude", "scout", "start"}
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")
	if flags.ReviewFiles != "" {
		args = append(args, "--review-files", flags.ReviewFiles)
	}

	workerPID, err := shared.SpawnBackgroundWorker(args, flags.Session)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to spawn background worker: %w", err)
	}

	// Output based on format
	if flags.Format == "json" {
		response := shared.StartResponse{
			SessionID:  sessionID,
			Turn:       manager.GetNextTurnNumber(),
			Status:     "in_progress",
			ProcessPID: workerPID,
			Message:    "Scout discovery session started successfully",
		}

		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Existing text output
		fmt.Fprintf(cmd.OutOrStdout(), "Scout discovery session started\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", shared.FormatSessionLabel("scout", sessionID))

		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nWhen discovery completes, proceed to change with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude change start --session %s --intent \"<your change request>\"\n", sessionID)

		fmt.Fprintf(cmd.OutOrStdout(), "\nStop the session with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout stop -s %s\n", sessionID)
	}

	return nil
}

// runBackgroundWorker executes the scout session in the background worker process
func runBackgroundWorker(cmd *cobra.Command, flags *StartFlags) error {
	// Use shared helper to initialize worker session
	manager, debugLogger, err := shared.InitWorkerSession(flags.Session, flags.Debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return err
	}
	defer debugLogger.Close()

	debugLogger.Log("WORKER", "Background worker started")
	debugLogger.Log("WORKER", fmt.Sprintf("Session ID: %s", flags.Session))
	debugLogger.Log("WORKER", fmt.Sprintf("Review Files: %s", flags.ReviewFiles))

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

	debugLogger.Log("WORKER", fmt.Sprintf("Session has %d turns", len(status.Turns)))

	nextTurn := manager.GetNextTurnNumber()

	// If this is Turn 1, no previous turn validation needed (new session)
	if nextTurn == 1 {
		debugLogger.Log("WORKER", "Starting Turn 1 (new session)")
	} else {
		// For Turn 2+, validate that the last turn is complete
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
	}

	// Execute the discovery turn (this blocks until complete)
	debugLogger.Log("WORKER", "Starting discovery turn")
	return manager.StartDiscoveryTurn()
}
