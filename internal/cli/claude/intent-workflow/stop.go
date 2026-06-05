/**
 * Component: Intent Workflow CLI Stop Command
 * Block-UUID: 206143a2-12a9-428a-ac8b-df868b50250c
 * Parent-UUID: fbefacd4-a249-40a3-99b4-3088aeee207c
 * Version: 1.2.1
 * Description: Implements 'gsc claude agent stop' command for stopping any running turn type (discovery, change, etc.).
 * Language: Go
 * Created-at: 2026-04-26T18:19:39.580Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1)
 */


package intentworkflowcli

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/spf13/cobra"
)

// StopResult represents the JSON response for the stop command
type StopResult struct {
	SessionID          string      `json:"session_id"`
	TurnNumber         int         `json:"turn_number"`
	TurnType           string      `json:"turn_type"`
	Status             string      `json:"status"`
	ShutdownMethod     string      `json:"shutdown_method"`
	ShutdownDurationMs int64       `json:"shutdown_duration_ms"`
	ProcessExited      bool        `json:"process_exited"`
	Error              *StopError  `json:"error,omitempty"`
	Duration           string      `json:"duration"`
}

// StopError represents an error that occurred during stop
type StopError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StopCmd creates the "agent stop" subcommand
func StopCmd() *cobra.Command {
	flags := &StopFlags{}

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running agent turn",
		Long: `Stop a running agent turn (discovery, change, etc.).

This will:
1. Terminate the background agent process
2. Kill the watcher process if running
3. Mark the session as stopped
4. Preserve all results for later review

Use --force to forcefully kill the process without cleanup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStopCommand(cmd, flags)
		},
	}

	RegisterStopFlags(cmd, flags)

	return cmd
}

// runStopCommand executes the stop command logic
func runStopCommand(cmd *cobra.Command, flags *StopFlags) error {
	// Validate flags
	if err := ValidateStopFlags(flags); err != nil {
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

	if !status.ProcessInfo.Running {
		fmt.Fprintf(cmd.OutOrStdout(), "Session %s is not running\n", flags.Session)
		return nil
	}

	// Get latest turn
	session := manager.GetSession()
	turnCount := len(session.Turns)
	if turnCount == 0 {
		return fmt.Errorf("session has no turns")
	}
	lastTurn := session.Turns[turnCount-1]

	// Kill watcher process if running
	if watcherPID := manager.GetWatcherPID(); watcherPID > 0 {
		process, err := os.FindProcess(watcherPID)
		if err == nil {
			// Send SIGTERM to watcher process
			process.Signal(syscall.SIGTERM)
			// Give it a moment to clean up
			time.Sleep(1 * time.Second)
			// Force kill if still running
			process.Signal(syscall.SIGKILL)
		}
	}

	// Stop the session
	stopStartTime := time.Now()
	if err := manager.StopSession(); err != nil {
		if !flags.Force {
			return fmt.Errorf("failed to stop session: %w", err)
		}
		// If force flag is set, continue despite error
	}

	// Wait for process to actually exit (with timeout)
	maxWaitTime := 10 * time.Second
	checkInterval := 100 * time.Millisecond
	elapsed := 0 * time.Second
	processExited := false
	var finalStatus string
	var shutdownError *StopError

	for elapsed < maxWaitTime {
		newStatus, err := manager.GetSessionStatus()
		if err != nil {
			// Failed to get status, assume error
			finalStatus = "error"
			shutdownError = &StopError{
				Code:    "STATUS_CHECK_FAILED",
				Message: fmt.Sprintf("Failed to verify process status: %v", err),
			}
			break
		}

		if !newStatus.ProcessInfo.Running {
			// Process is dead!
			processExited = true
			finalStatus = "stopped"
			break
		}

		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	// If still running after timeout
	if elapsed >= maxWaitTime && !processExited {
		finalStatus = "stopping"
		shutdownError = &StopError{
			Code:    "TIMEOUT",
			Message: fmt.Sprintf("Process did not exit within %v timeout", maxWaitTime),
		}
	}

	shutdownDuration := time.Since(stopStartTime).Milliseconds()

	// Build result
	result := StopResult{
		SessionID:  flags.Session,
		TurnNumber: lastTurn.TurnNumber,
		TurnType:   lastTurn.TurnType,
		Status:     finalStatus,
	}

	// Determine shutdown method
	if shutdownError != nil {
		// Use the error we detected during wait
		result.ShutdownMethod = "Timeout"
		result.Error = shutdownError
	} else if status.Error != nil {
		errorMsg := *status.Error
		if idx := findColonIndex(errorMsg); idx > 0 {
			errorCode := errorMsg[:idx]
			switch errorCode {
			case "USER_STOPPED":
				result.ShutdownMethod = "Graceful (SIGTERM)"
			case "FORCE_STOPPED":
				result.ShutdownMethod = "Forced (SIGKILL)"
			case "PROCESS_NOT_FOUND":
				result.ShutdownMethod = "Process already exited"
			case "KILL_FAILED":
				result.ShutdownMethod = "Kill failed"
			case "ZOMBIE_PROCESS":
				result.ShutdownMethod = "Zombie process"
			default:
				result.ShutdownMethod = errorCode
			}
		}
	} else {
		result.ShutdownMethod = "Graceful (SIGTERM)"
	}

	// Set additional fields
	result.ShutdownDurationMs = shutdownDuration
	result.ProcessExited = processExited

	// Calculate duration
	if status.CompletedAt != nil {
		duration := status.CompletedAt.Sub(status.StartedAt)
		result.Duration = duration.Round(time.Second).String()
	}

	// Output result
	if flags.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ %s turn stopped\n", capitalizeFirst(lastTurn.TurnType))
		fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", finalStatus)
		fmt.Fprintf(cmd.OutOrStdout(), "  Shutdown: %s\n", result.ShutdownMethod)
		fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %dms\n", shutdownDuration)
		if !processExited {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⚠ Warning: Process may still be running\n")
		}
		if result.Duration != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %s\n", result.Duration)
		}

		// Show turn-specific results
		if lastTurn.TurnType == "discovery" && status.TotalFound > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Candidates discovered: %d\n", status.TotalFound)
			fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude agent status -s %s\n", flags.Session)
		} else if lastTurn.TurnType == "change" && lastTurn.Result != nil && lastTurn.Result.Change != nil {
			changeResults := lastTurn.Result.Change
			fmt.Fprintf(cmd.OutOrStdout(), "  Files modified: %d\n", changeResults.FilesModified.TotalCount)
			fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude agent status -s %s\n", flags.Session)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nNo results yet.\n")
		}

		// Show error if applicable
		if shutdownError != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ Error: %s\n", shutdownError.Message)
		} else if status.Error != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ Warning: %s\n", *status.Error)
		}
	}

	return nil
}

// findColonIndex finds the first colon in a string
func findColonIndex(s string) int {
	for i, c := range s {
		if c == ':' {
			return i
		}
	}
	return -1
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// StopFlags contains flags for the stop command
type StopFlags struct {
	Session string
	Force   bool
	Format  string
}

// RegisterStopFlags registers flags for the stop command
func RegisterStopFlags(cmd *cobra.Command, flags *StopFlags) {
	cmd.Flags().StringVarP(
		&flags.Session,
		"session", "s",
		"",
		"Session ID",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().BoolVar(
		&flags.Force,
		"force",
		false,
		"Force kill the process without cleanup",
	)

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
	)
}

// ValidateStopFlags validates the stop command flags
func ValidateStopFlags(flags *StopFlags) error {
	if flags.Session == "" {
		return fmt.Errorf("session ID is required")
	}

	if flags.Format != "text" && flags.Format != "json" {
		return fmt.Errorf("invalid format: %s (must be 'text' or 'json')", flags.Format)
	}

	return nil
}
