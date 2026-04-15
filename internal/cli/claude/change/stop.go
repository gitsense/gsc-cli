/**
 * Component: Change CLI Stop Command
 * Block-UUID: 94007be8-e9b9-47e8-a7a3-a24243d88641
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Implements 'gsc claude change stop' command for terminating change turns. Updated to import from agent package instead of scout.
 * Language: Go
 * Created-at: 2026-04-15T04:09:30.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package change

import (
	"fmt"
	"os"
	"syscall"
	"time"

	agent "github.com/gitsense/gsc-cli/internal/claude/agent"
	"github.com/spf13/cobra"
)

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
	manager, err := agent.LoadSession(flags.Session)
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
	if err := manager.StopSession(); err != nil {
		if !flags.Force {
			return fmt.Errorf("failed to stop session: %w", err)
		}
		// If force flag is set, continue despite error
	}

	// Display confirmation
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Change turn stopped\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
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

	// Show change results if available
	if status.Turns != nil && len(status.Turns) > 0 {
		lastTurn := status.Turns[len(status.Turns)-1]
		if lastTurn.TurnType == "change" && lastTurn.Results != nil && lastTurn.Results.ChangeResults != nil {
			changeResults := lastTurn.Results.ChangeResults
			fmt.Fprintf(cmd.OutOrStdout(), "  Files modified: %d\n", changeResults.ChangeSummary.FilesModifiedCount)
			fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", flags.Session)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNo changes made yet.\n")
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

// StopFlags contains flags for the stop command
type StopFlags struct {
	Session string
	Force   bool
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
}

// ValidateStopFlags validates the stop command flags
func ValidateStopFlags(flags *StopFlags) error {
	if flags.Session == "" {
		return &FlagError{Flag: "session", Message: "session ID is required"}
	}

	return nil
}
