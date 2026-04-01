/**
 * Component: Scout CLI Stop Command
 * Block-UUID: 89367aef-a915-4407-825e-ec9722608de8
 * Parent-UUID: d4c1d171-4a3f-4aa1-b050-f42067e9583e
 * Version: 1.1.0
 * Description: Implements 'gsc claude scout stop' command for terminating Scout sessions
 * Language: Go
 * Created-at: 2026-04-01T05:37:15.179Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.1.0)
 */


package scoutcli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
	"github.com/spf13/cobra"
)

// StopCmd creates the "scout stop" subcommand
func StopCmd() *cobra.Command {
	flags := &StopFlags{}

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running Scout session",
		Long: `Stop a Scout discovery or verification session.

This will:
1. Terminate the background Scout process
2. Write a stop event to the session log
3. Mark the session as stopped
4. Preserve all discovered candidates for later review

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
	if err := ValidateScoutFlags(cmd); err != nil {
		return err
	}

	// Validate flags
	if err := ValidateStopFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Load the session
	manager, err := claudescout.LoadSession(flags.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Get current status to check if running
	status, err := manager.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to get session status: %w", err)
	}

	if !status.ProcessInfo.Running {
		fmt.Fprintf(cmd.OutOrStdout(), "Session %s is not running\n", flags.SessionID)
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
	if err := manager.StopSession(); err != nil {
		if !flags.Force {
			return fmt.Errorf("failed to stop session: %w", err)
		}
		// If force flag is set, continue despite error
	}

	// Display confirmation
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Scout session stopped\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", status.Status)

	// Show shutdown method
	if status.Error != nil {
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

	if status.TotalFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Candidates discovered: %d\n", status.TotalFound)
		fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", flags.SessionID)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNo candidates discovered yet.\n")
	}

	// Show error if applicable
	if status.Error != nil {
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

// CanStopSession checks if a session can be stopped
func CanStopSession(sessionID string) (bool, error) {
	manager, err := claudescout.LoadSession(sessionID)
	if err != nil {
		return false, err
	}

	status, err := manager.GetSessionStatus()
	if err != nil {
		return false, err
	}

	return status.ProcessInfo.Running, nil
}

// GetSessionProcessInfo retrieves process information for a session
func GetSessionProcessInfo(sessionID string) (*claudescout.ProcessInfo, error) {
	manager, err := claudescout.LoadSession(sessionID)
	if err != nil {
		return nil, err
	}

	status, err := manager.GetSessionStatus()
	if err != nil {
		return nil, err
	}

	return &status.ProcessInfo, nil
}
