/**
 * Component: Intent Workflow CLI Retry Command
 * Block-UUID: 416f5084-758b-47b9-8691-81c72de96516
 * Parent-UUID: 60e6adec-e561-4f54-9660-34140b9450f1
 * Version: 1.3.0
 * Description: Implements 'gsc claude agent retry' command for deleting and restarting the latest turn. Updated to allow retry on stopped and error states.
 * Language: Go
 * Created-at: 2026-04-20T15:37:40.455Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package intentworkflowcli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/spf13/cobra"
)

// RetryResult represents the JSON response for the retry command
type RetryResult struct {
	SessionID  string `json:"session_id"`
	Turn       int    `json:"turn"`
	Status     string `json:"status"`
	ProcessPID int    `json:"process_pid"`
	Message    string `json:"message"`
}

// RetryCmd creates the "agent retry" subcommand
func RetryCmd() *cobra.Command {
	flags := &RetryFlags{}

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "Retry the latest turn",
		Long: `Delete the latest turn and automatically restart it with the same turn type.

This will:
1. Delete the latest turn directory
2. Remove the turn from session state
3. Reset session status to previous state
4. Automatically restart the turn as a background process`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRetryCommand(cmd, flags)
		},
	}

	RegisterRetryFlags(cmd, flags)

	return cmd
}

// runRetryCommand executes the retry command logic
func runRetryCommand(cmd *cobra.Command, flags *RetryFlags) error {
	// If --watch-worker flag is set, this is the background process
	if flags.WatchWorker {
		return runRetryWorker(flags)
	}

	// Load session
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	session := manager.GetSession()
	turnCount := len(session.Turns)

	if turnCount == 0 {
		return fmt.Errorf("session has no turns to retry")
	}

	// Get latest turn
	lastTurn := session.Turns[turnCount-1]

	// Check if turn is running (only state that blocks retry)
	if lastTurn.Status == "running" {
		return fmt.Errorf("cannot retry turn: turn is still running (status: %s)", lastTurn.Status)
	}

	// Delete turn directory
	turnDir := manager.GetConfig().GetTurnDir(lastTurn.TurnNumber)
	if err := os.RemoveAll(turnDir); err != nil {
		return fmt.Errorf("failed to delete turn directory: %w", err)
	}

	// Remove latest turn from session.Turns
	session.Turns = session.Turns[:turnCount-1]

	// Reset session state
	if turnCount > 1 {
		prevTurn := session.Turns[turnCount-2]
		switch prevTurn.TurnType {
		case "discovery":
			session.Status = "discovery_complete"
		case "change":
			session.Status = "change_complete"
		default:
			session.Status = "stopped"
		}
	} else {
		session.Status = "discovery"
	}

	// Clear error and completion fields
	session.Error = nil
	session.CompletedAt = nil
	session.WatcherPID = nil

	// Write updated session state before spawning worker
	if err := manager.WriteSessionState(); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	// Capture new turn number before spawning (session state is now reset)
	newTurnNumber := manager.GetNextTurnNumber()

	// Spawn background worker to restart the turn
	workerPID, err := spawnRetryWorker(flags)
	if err != nil {
		return fmt.Errorf("failed to spawn background worker: %w", err)
	}

	// Output result
	result := RetryResult{
		SessionID:  flags.Session,
		Turn:       newTurnNumber,
		Status:     "in_progress",
		ProcessPID: workerPID,
		Message:    "Turn restarted successfully",
	}

	if flags.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Turn restarted\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", result.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "  Process PID: %d\n", result.ProcessPID)
	}

	return nil
}

// spawnRetryWorker spawns a detached background worker to restart the turn.
// Returns the worker PID immediately (non-blocking).
func spawnRetryWorker(flags *RetryFlags) (int, error) {
	args := []string{"claude", "intent-workflow", "retry"}
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")

	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = newSysProcAttr()

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to spawn worker: %w", err)
	}

	// Wait briefly to verify the process is alive
	time.Sleep(100 * time.Millisecond)
	process, err := os.FindProcess(cmd.Process.Pid)
	if err != nil {
		return 0, fmt.Errorf("failed to find worker process: %w", err)
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, fmt.Errorf("worker process died immediately (check debug.log for details)")
	}

	// Store worker PID in session state
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		return cmd.Process.Pid, fmt.Errorf("failed to load session to store watcher PID: %w", err)
	}
	manager.SetWatcherPID(cmd.Process.Pid)
	if err := manager.WriteSessionState(); err != nil {
		return cmd.Process.Pid, fmt.Errorf("failed to write session state: %w", err)
	}

	return cmd.Process.Pid, nil
}

// runRetryWorker executes the turn restart in the background worker process (blocking).
// Turn type is inferred from current session status after the foreground reset.
func runRetryWorker(flags *RetryFlags) error {
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load session: %v\n", err)
		return fmt.Errorf("failed to load session: %w", err)
	}

	session := manager.GetSession()

	switch session.Status {
	case "discovery":
		return manager.StartDiscoveryTurn()
	case "discovery_complete":
		return manager.StartChangeTurn(session.Intent)
	case "stopped":
		return manager.StartDiscoveryTurn()
	default:
		return fmt.Errorf("unexpected session status for retry worker: %s", session.Status)
	}
}

// RetryFlags contains flags for the retry command
type RetryFlags struct {
	Session     string
	Format      string
	WatchWorker bool
}

// RegisterRetryFlags registers flags for the retry command
func RegisterRetryFlags(cmd *cobra.Command, flags *RetryFlags) {
	cmd.Flags().StringVarP(
		&flags.Session,
		"session", "s",
		"",
		"Session ID",
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
