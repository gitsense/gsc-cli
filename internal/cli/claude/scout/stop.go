/*
 * Component: Scout CLI Stop Command
 * Block-UUID: 6c4e9f3d-5a7b-4d2c-8e1f-7a3c9f5e2b8d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements 'gsc claude scout stop' command for terminating Scout sessions
 * Language: Go
 * Created-at: 2026-03-27T00:00:00.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0)
 */


package scout

import (
	"fmt"

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
	// Validate flags
	if err := ValidateStopFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Load the session
	manager, err := LoadSession(flags.SessionID)
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

	// Stop the session
	if err := manager.StopSession(); err != nil {
		if !flags.Force {
			return fmt.Errorf("failed to stop session: %w", err)
		}
		// If force flag is set, continue despite error
	}

	// Display confirmation
	fmt.Fprintf(cmd.OutOrStdout(), "Scout session stopped\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", flags.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "Status: stopped\n")

	if status.TotalFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Candidates discovered: %d\n", status.TotalFound)
		fmt.Fprintf(cmd.OutOrStdout(), "\nView results with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", flags.SessionID)
	}

	return nil
}

// CanStopSession checks if a session can be stopped
func CanStopSession(sessionID string) (bool, error) {
	manager, err := LoadSession(sessionID)
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
func GetSessionProcessInfo(sessionID string) (*ProcessInfo, error) {
	manager, err := LoadSession(sessionID)
	if err != nil {
		return nil, err
	}

	status, err := manager.GetSessionStatus()
	if err != nil {
		return nil, err
	}

	return &status.ProcessInfo, nil
}
