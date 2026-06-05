/**
 * Component: Change CLI Stop Command
 * Block-UUID: 10162fed-e250-4be9-a3da-079821cabbe0
 * Parent-UUID: 92cbd216-07e0-43d3-aeac-3240b3ef7393
 * Version: 1.7.0
 * Description: Implements 'gsc claude change stop' command for terminating change turns. Updated to import from agent package instead of scout.
 * Language: Go
 * Created-at: 2026-04-30T15:27:28.702Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0)
 */


package change

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/gitsense/gsc-cli/internal/cli/claude/shared"
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
}

// StopError represents an error that occurred during stop
type StopError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StopCmd creates the "change stop" subcommand
func StopCmd() *cobra.Command {
	flags := &StopFlags{}

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running change turn",
		Long: `Stop a change turn session.

This will:
1. Terminate the background change process
2. Write a stop event to the session log
3. Mark the session as stopped
4. Preserve all change results for later review

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
	// Validate that unsupported flags are not set
	if err := ValidateChangeFlags(cmd); err != nil {
		return err
	}

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

	// Get latest turn for result
	session := manager.GetSession()
	turnCount := len(session.Turns)
	var lastTurn *intent_workflow.TurnState
	if turnCount > 0 {
		lastTurn = &session.Turns[turnCount-1]
	}

	// Display confirmation
	if flags.Format == "json" {
		result := StopResult{
			SessionID:          flags.Session,
			TurnNumber:         0,
			TurnType:           "change",
			Status:             finalStatus,
			ShutdownMethod:     "Graceful (SIGTERM)",
			ShutdownDurationMs: shutdownDuration,
			ProcessExited:      processExited,
			Error:              shutdownError,
		}

		if lastTurn != nil {
			result.TurnNumber = lastTurn.TurnNumber
			result.TurnType = lastTurn.TurnType
		}

		// Determine shutdown method
		if shutdownError != nil {
			result.ShutdownMethod = "Timeout"
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
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text output
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Change turn stopped\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", finalStatus)
	fmt.Fprintf(cmd.OutOrStdout(), "  Shutdown: %s\n", "Graceful (SIGTERM)")
	fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %dms\n", shutdownDuration)
	if !processExited {
		fmt.Fprintf(cmd.OutOrStdout(), "  ⚠ Warning: Process may still be running\n")
	}

	// Show shutdown method
	if shutdownError != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Error: %s\n", shutdownError.Message)
	} else if status.Error != nil {
		// Extract error code from error message
		errorMsg := *status.Error
		shutdownMethod := "unknown"
		if len(errorMsg) > 0 {
			// Error format is "CODE: message"
			if idx := findColonIndex(errorMsg); idx > 0 {
				errorCode := errorMsg[:idx]
				switch errorCode {
				case "USER_STOPPED":
					shutdownMethod = "Graceful (SIGTERM)"
				case "FORCE_STOPPED":
					shutdownMethod = "Forced (SIGKILL)"
				case "PROCESS_NOT_FOUND":
					shutdownMethod = "Process already exited"
				case "KILL_FAILED":
					shutdownMethod = "Kill failed"
				case "ZOMBIE_PROCESS":
					shutdownMethod = "Zombie process"
				default:
					shutdownMethod = errorCode
				}
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Shutdown: %s\n", shutdownMethod)
	}
	
	// Show session duration
	if status.CompletedAt != nil {
		duration := status.CompletedAt.Sub(status.StartedAt)
		fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %v\n", duration.Round(time.Second))
	}

	// Show change results if available
	if status.Turns != nil && len(status.Turns) > 0 {
		lastTurn := status.Turns[len(status.Turns)-1]
		if lastTurn.TurnType == "change" && lastTurn.Result != nil && lastTurn.Result.Change != nil {
			changeResults := lastTurn.Result.Change
			fmt.Fprintf(cmd.OutOrStdout(), "  Files modified: %d\n", changeResults.FilesModified.TotalCount)
			fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", flags.Session)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nNo changes made yet.\n")
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNo changes made yet.\n")
	}

	// Show error if applicable
	if shutdownError != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ Error: %s\n", shutdownError.Message)
	} else if status.Error != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ Warning: %s\n", *status.Error)
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
		"Change session ID to stop",
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
		return &shared.FlagError{Flag: "session", Message: "session ID is required"}
	}

	return nil
}
