/**
 * Component: Scout CLI Status Command
 * Block-UUID: d8069e6b-eb38-4460-a3ea-5475befb4f28
 * Parent-UUID: e30f8d5b-5b07-49d3-a911-4f09abe61179
 * Version: 1.0.8
 * Description: Implements 'gsc claude scout status' command for monitoring Scout sessions
 * Language: Go
 * Created-at: 2026-03-28T02:31:09.286Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), Gemini 3 Flash (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), GLM-4.7 (v1.0.7), GLM-4.7 (v1.0.8)
 */


package scoutcli

import (
	"encoding/json"
	"fmt"
	"time"

	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
	"github.com/spf13/cobra"
)

// StatusCmd creates the "scout status" subcommand
func StatusCmd() *cobra.Command {
	flags := &StatusFlags{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Monitor a Scout discovery session",
		Long: `Display status and progress of a running Scout session.

Use -f/--follow to stream events in real-time as the session progresses.
Use --format to control output format (json, table, pretty).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, flags)
		},
	}

	RegisterStatusFlags(cmd, flags)

	return cmd
}

// runStatusCommand executes the status command logic
func runStatusCommand(cmd *cobra.Command, flags *StatusFlags) error {
	// Validate flags
	if err := ValidateStatusFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Load the session
	manager, err := claudescout.LoadSession(flags.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Get current status
	status, err := manager.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to get session status: %w", err)
	}

	// Display status
	if err := displayStatus(cmd, status, flags.Format); err != nil {
		return fmt.Errorf("failed to display status: %w", err)
	}

	// If follow mode, stream events in real-time
	if flags.Follow {
		return followSessionEvents(cmd, manager.GetConfig())
	}

	return nil
}

// displayStatus outputs status in the requested format
func displayStatus(cmd *cobra.Command, status *claudescout.StatusData, format string) error {
	switch format {
	case "json":
		return displayStatusJSON(cmd, status)
	case "table":
		return displayStatusTable(cmd, status)
	case "pretty":
		return displayStatusPretty(cmd, status)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// displayStatusJSON outputs status as JSON
func displayStatusJSON(cmd *cobra.Command, status *claudescout.StatusData) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// displayStatusTable outputs status in table format
func displayStatusTable(cmd *cobra.Command, status *claudescout.StatusData) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", status.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", status.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Phase: %s\n", status.Phase)
	fmt.Fprintf(cmd.OutOrStdout(), "Started: %s\n", status.StartedAt.Format(time.RFC3339))

	if status.CompletedAt != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Completed: %s\n", status.CompletedAt.Format(time.RFC3339))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Elapsed: %ds\n", status.ElapsedSeconds)

	if status.EstimatedRemaining != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Estimated Remaining: %ds\n", *status.EstimatedRemaining)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nWorking Directories: %d\n", len(status.WorkingDirectories))
	for _, wd := range status.WorkingDirectories {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", wd.Name, wd.Path)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nCandidates Found: %d\n", status.TotalFound)
	if len(status.Candidates) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Top candidates:\n")
		for i, cand := range status.Candidates {
			if i >= 5 {
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (score: %.2f)\n", i+1, cand.FilePath, cand.Score)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nProcess: PID %d (%v)\n", status.ProcessInfo.PID, status.ProcessInfo.Running)

	if status.Error != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nError: %s\n", *status.Error)
	}

	if status.NextAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNext Action: %s\n", status.NextAction.Message)
	}

	return nil
}

// displayStatusPretty outputs status in a user-friendly format
func displayStatusPretty(cmd *cobra.Command, status *claudescout.StatusData) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\n  Scout Session: %s\n", status.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", colorizeStatus(status.Status))
	fmt.Fprintf(cmd.OutOrStdout(), "  Phase: %s\n", status.Phase)
	fmt.Fprintf(cmd.OutOrStdout(), "  Process: PID %d", status.ProcessInfo.PID)

	if status.ProcessInfo.Running {
		fmt.Fprintf(cmd.OutOrStdout(), " (running)")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), " (stopped)")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	elapsed := time.Since(status.StartedAt)
	fmt.Fprintf(cmd.OutOrStdout(), "  Elapsed: %v\n", elapsed)

	if len(status.WorkingDirectories) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Working Directories: %d\n", len(status.WorkingDirectories))
		for _, wd := range status.WorkingDirectories {
			fmt.Fprintf(cmd.OutOrStdout(), "    • %s\n", wd.Name)
		}
	}

	if status.TotalFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Candidates Found: %d\n", status.TotalFound)
		if len(status.Candidates) > 0 {
			maxShow := 3
			if len(status.Candidates) < maxShow {
				maxShow = len(status.Candidates)
			}
			for i := 0; i < maxShow; i++ {
				cand := status.Candidates[i]
				fmt.Fprintf(cmd.OutOrStdout(), "    %d. %s (%.1f%%)\n", i+1, cand.FilePath, cand.Score*100)
			}
			if len(status.Candidates) > maxShow {
				fmt.Fprintf(cmd.OutOrStdout(), "    ... and %d more\n", len(status.Candidates)-maxShow)
			}
		}
	}

	if status.Error != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  ⚠ Error: %s\n", *status.Error)
	}

	if status.NextAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  → %s\n", status.NextAction.Message)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	return nil
}

// colorizeStatus returns a colorized status string using ANSI codes
func colorizeStatus(status string) string {
	switch status {
	case "discovery", "discovery_complete", "verification", "verification_complete":
		// Green for active/completed states
		return fmt.Sprintf("\033[32m%s\033[0m", status)
	case "stopped":
		// Yellow for stopped state
		return fmt.Sprintf("\033[33m%s\033[0m", status)
	case "error":
		// Red for error state
		return fmt.Sprintf("\033[31m%s\033[0m", status)
	default:
		// No color for unknown states
		return status
	}
}

// followSessionEvents streams events from the log file as they arrive
func followSessionEvents(cmd *cobra.Command, config *claudescout.SessionConfig) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Following session events (Ctrl+C to stop)...\n\n")

	startTime := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Define completion events that signal session end
	completionEvents := map[string]bool{"done": true, "error": true, "stopped": true}

	// Add timeout (30 minutes max follow time)
	timeout := time.After(30 * time.Minute)

	lastEventCount := 0

	for {
		select {
		case <-ticker.C:
			// Print progress indicator every 5 seconds if no new events
			if lastEventCount == 0 && time.Since(startTime) > 5*time.Second {
				fmt.Fprintf(cmd.OutOrStdout(), ".")
			}

			// Try to read the latest log file
			logFile, _, err := claudescout.NewProcessorHelper(config).GetLatestLogFile()
			if err != nil {
				// Log file not yet created
				continue
			}

			reader, err := claudescout.NewEventReader(logFile)
			if err != nil {
				continue
			}

			events, err := reader.ReadAllEvents()
			reader.Close()
			if err != nil {
				continue
			}

			// Display new events
			for i := lastEventCount; i < len(events); i++ {
				event := events[i]
				displayEvent(cmd, &event)
			}

			lastEventCount = len(events)

			// Check if session is complete
			if lastEventCount > 0 {
				lastEvent := events[len(events)-1]
				if completionEvents[lastEvent.Type] {
					fmt.Fprintf(cmd.OutOrStdout(), "\n")
					return nil
				}
			}

		case <-timeout:
			fmt.Fprintf(cmd.OutOrStdout(), "\n\nTimeout: Session did not complete within 30 minutes\n")
			return fmt.Errorf("follow timeout exceeded")
		}
	}
}

// truncateTimestamp truncates a timestamp string to 19 characters (YYYY-MM-DDTHH:MM:SS)
func truncateTimestamp(ts string) string {
	if len(ts) > 19 {
		return ts[:19]
	}
	return ts
}

// displayEvent displays a single event in the stream
func displayEvent(cmd *cobra.Command, event *claudescout.StreamEvent) {
	timestamp := truncateTimestamp(event.Timestamp)

	fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", timestamp, event.Type)

	switch event.Type {
	case "init":
		// Init events are handled during session initialization, not displayed in follow mode
		return

	case "status":
		if statusEvent, ok := event.Data.(map[string]interface{}); ok {
			if msg, exists := statusEvent["message"]; exists {
				fmt.Fprintf(cmd.OutOrStdout(), "  %v\n", msg)
			}
		}

	case "candidates":
		if candEvent, ok := event.Data.(map[string]interface{}); ok {
			if found, exists := candEvent["total_found"]; exists {
				fmt.Fprintf(cmd.OutOrStdout(), "  Found: %v candidates\n", found)
			}
		}

	case "done":
		if doneEvent, ok := event.Data.(map[string]interface{}); ok {
			if status, exists := doneEvent["status"]; exists {
				fmt.Fprintf(cmd.OutOrStdout(), "  Status: %v\n", status)
			}
		}

	case "error":
		if errEvent, ok := event.Data.(map[string]interface{}); ok {
			if msg, exists := errEvent["message"]; exists {
				fmt.Fprintf(cmd.OutOrStdout(), "  Error: %v\n", msg)
			}
		}
	}
}
