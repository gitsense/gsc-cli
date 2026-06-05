/**
 * Component: Change CLI Resume Command
 * Block-UUID: 6067628e-71c7-45e3-88fd-0f89a6e9642d
 * Parent-UUID: deededcc-2514-4199-95cd-200bf76161b9
 * Version: 2.6.0
 * Description: Implements 'gsc claude change resume' command for resuming failed change turns. Updated to support code provenance enforcement during resumption. It now reads the EnableCodeProvenance state from the persisted session and passes it to the PostProcessor, ensuring that recovered turns maintain the same provenance recording behavior as the original turn.
 * Language: Go
 * Created-at: 2026-04-30T15:26:49.649Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.0.2), Gemini 3 Flash (v2.1.0), GLM-4.7 (v2.2.0), Gemini 3 Flash (v2.3.0), GLM-4.7 (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.6.0)
 */


package change

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/gitsense/gsc-cli/internal/cli/claude/shared"
	"github.com/spf13/cobra"
)

// ResumeCmd creates the "change resume" subcommand
func ResumeCmd() *cobra.Command {
	flags := &ResumeFlags{}

	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a failed change turn to generate missing metadata",
		Long: `Resume a change turn that failed due to missing .change-meta.json files.

This command will:
1. Load the session and identify the failed change turn
2. Extract Git provenance metadata (SHAs) for modified files
3. Spawn a Claude subprocess to generate missing metadata files
4. Validate and process the generated metadata
5. Complete the change turn successfully

Use this command when a change turn fails with "missing .change-meta.json files" error.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResumeCommand(cmd, flags)
		},
	}

	RegisterResumeFlags(cmd, flags)

	return cmd
}

// runResumeCommand executes the resume command logic
func runResumeCommand(cmd *cobra.Command, flags *ResumeFlags) error {
	// If --watch-worker flag is set, this is the background process
	if flags.WatchWorker {
		return runBackgroundResumeWorker(cmd, flags)
	}

	// Validate that unsupported flags are not set
	if err := ValidateChangeFlags(cmd); err != nil {
		return err
	}

	// Validate flags
	if err := ValidateResumeFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Load the session
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Get current status to check if running
	status, err := manager.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to get session status: %w", err)
	}

	// Find the failed change turn
	var failedTurn *intent_workflow.TurnState
	for i := len(status.Turns) - 1; i >= 0; i-- {
		if status.Turns[i].TurnType == "change" && status.Turns[i].Status == "error" {
			failedTurn = &status.Turns[i]
			break
		}
	}

	if failedTurn == nil {
		return fmt.Errorf("no failed change turn found in session %s", flags.Session)
	}

	// Check if the error is due to missing metadata
	if failedTurn.Error == nil || !containsMissingMetadataError(*failedTurn.Error) {
		return fmt.Errorf("turn %d failed but not due to missing metadata: %s", failedTurn.TurnNumber, *failedTurn.Error)
	}

	// Spawn background worker with --watch-worker flag
	workerPID, err := spawnResumeWorker(flags, failedTurn.TurnNumber)
	if err != nil {
		return fmt.Errorf("failed to spawn resume worker: %w", err)
	}

	// Output based on format
	if flags.Format == "json" {
		response := shared.StartResponse{
			SessionID:  flags.Session,
			Turn:       failedTurn.TurnNumber,
			Status:     "resuming",
			ProcessPID: workerPID,
			Message:    fmt.Sprintf("Resuming change turn %d", failedTurn.TurnNumber),
		}

		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Text output
		fmt.Fprintf(cmd.OutOrStdout(), "Resuming change turn %d\n", failedTurn.TurnNumber)
		fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", shared.FormatSessionLabel("scout", flags.Session))
		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", flags.Session)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", flags.Session)
	}

	return nil
}

// runBackgroundResumeWorker executes the resume turn in the background worker process
func runBackgroundResumeWorker(cmd *cobra.Command, flags *ResumeFlags) error {
	// Create debug log immediately
	config, err := intent_workflow.NewSessionConfig(flags.Session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create session config: %v\n", err)
		return fmt.Errorf("failed to create session config: %w", err)
	}

	debugLogger, err := intent_workflow.NewDebugLogger(config.GetSessionDir(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create debug log: %v\n", err)
		return fmt.Errorf("failed to create debug log: %w", err)
	}
	defer debugLogger.Close()

	debugLogger.Log("WORKER", "Background resume worker started")
	debugLogger.Log("WORKER", fmt.Sprintf("Session ID: %s", flags.Session))

	// Load existing session
	debugLogger.Log("WORKER", "Loading session...")
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		debugLogger.LogError("Failed to load session", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load session: %v\n", err)
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Find the failed change turn
	session := manager.GetSession()
	var failedTurn *intent_workflow.TurnState
	for i := len(session.Turns) - 1; i >= 0; i-- {
		if session.Turns[i].TurnType == "change" && session.Turns[i].Status == "error" {
			failedTurn = &session.Turns[i]
			break
		}
	}

	if failedTurn == nil {
		err := fmt.Errorf("no failed change turn found")
		debugLogger.LogError("Failed to find turn", err)
		return err
	}

	debugLogger.Log("WORKER", fmt.Sprintf("Found failed turn %d", failedTurn.TurnNumber))

	// Call the manager to start the resume-change turn
	// This handles spawning, waiting, and stream processing
	if err := manager.StartResumeChangeTurn(failedTurn.TurnNumber); err != nil {
		debugLogger.LogError("Resume turn failed", err)
		return fmt.Errorf("resume turn failed: %w", err)
	}
	debugLogger.Log("WORKER", "Resume turn subprocess completed")

	// Run centralized post-processing
	// This handles logging, validation, enrichment, correction, and result writing
	debugLogger.Log("WORKER", "Starting post-processing...")
	// Pass EnableCodeProvenance from the persisted session state
	pp := intent_workflow.NewPostProcessor(manager, failedTurn.TurnNumber, failedTurn.StartedAt, false, session.EnableCodeProvenance, debugLogger)
	if err := pp.Run(); err != nil {
		debugLogger.LogError("Post-processing failed", err)
		return fmt.Errorf("post-processing failed: %w", err)
	}

	debugLogger.Log("WORKER", "Resume turn completed successfully")
	return nil
}

// spawnResumeWorker spawns a background worker process to handle the resume turn
func spawnResumeWorker(flags *ResumeFlags, turnNumber int) (int, error) {
	// Build args for worker
	args := []string{"claude", "change", "resume"}
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")
	args = append(args, "--turn", fmt.Sprintf("%d", turnNumber))

	// Spawn worker in background
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = newSysProcAttr()

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

	return cmd.Process.Pid, nil
}

// containsMissingMetadataError checks if the error message indicates missing metadata
func containsMissingMetadataError(errMsg string) bool {
	return strings.Contains(errMsg, "missing .change-meta.json") ||
		strings.Contains(errMsg, "Missing .change-meta.json")
}

// ResumeFlags contains flags for the resume command
type ResumeFlags struct {
	Session string
	Format  string
	Turn    int
	WatchWorker bool
}

// RegisterResumeFlags registers flags for the resume command
func RegisterResumeFlags(cmd *cobra.Command, flags *ResumeFlags) {
	cmd.Flags().StringVarP(
		&flags.Session,
		"session", "s",
		"",
		"Change session ID to resume",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
	)

	cmd.Flags().IntVar(
		&flags.Turn,
		"turn",
		0,
		"Turn number to resume (defaults to latest failed turn)",
	)

	cmd.Flags().BoolVar(
		&flags.WatchWorker,
		"watch-worker",
		false,
		"Run as background worker process",
	)
	cmd.Flags().MarkHidden("watch-worker")
}

// ValidateResumeFlags validates the resume command flags
func ValidateResumeFlags(flags *ResumeFlags) error {
	if flags.Session == "" {
		return &shared.FlagError{Flag: "session", Message: "session ID is required"}
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[flags.Format] {
		return &shared.FlagError{Flag: "format", Message: "format must be 'text' or 'json'"}
	}

	return nil
}
