/**
 * Component: Intent Workflow CLI Status Command
 * Block-UUID: 9b0c1d2e-4f5a-6b7c-8d9e-0f1a2b3c4d5e
 * Parent-UUID: 482ce3f6-a964-40f4-9b36-39c4a22adf80
 * Version: 1.2.4
 * Description: Implements 'gsc claude agent status' command for monitoring Intent workflow sessions. Generic version that works with all intent workflow session types (discovery, validation, change). Added getSuccinctResponse helper function and display logic to show AI-generated natural language summaries from discovery turns.
 * Language: Go
 * Created-at: 2026-04-28T23:17:15.869Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.2.3), GLM-4.7 (v1.2.4)
 */


package intentworkflowcli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/spf13/cobra"
)

// FileScoreTracking tracks scores for a file across turns
type FileScoreTracking struct {
	FilePath               string
	LatestDiscoveryScore   float64
	LatestValidationScore  float64
	LatestDiscoveryTurn    int
	LatestValidationTurn   int
	Reasoning              string
	Metadata               intent_workflow.CandidateMetadata
}

// ConsolidatedCandidates represents all unique candidates with their latest scores
type ConsolidatedCandidates struct {
	DiscoveryCandidates  []FileScoreTracking
	ValidationCandidates []FileScoreTracking
}

// StatusCmd creates the "agent status" subcommand
func StatusCmd() *cobra.Command {
	flags := &StatusFlags{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Monitor an Intent workflow session",
		Long: `Display status and progress of a running Intent workflow session.

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
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Get current status
	status, err := manager.GetSessionStatus()
	if err != nil {
		return fmt.Errorf("failed to get session status: %w", err)
	}

	// Display status
	if err := displayStatus(cmd, status, flags.Format, flags.Verbose); err != nil {
		return fmt.Errorf("failed to display status: %w", err)
	}

	// If follow mode, stream events in real-time
	if flags.Follow {
		return followSessionEvents(cmd, manager.GetConfig())
	}

	return nil
}

// displayStatus outputs status in the requested format
func displayStatus(cmd *cobra.Command, status *intent_workflow.StatusData, format string, verbose bool) error {
	switch format {
	case "json":
		return displayStatusJSON(cmd, status, verbose)
	case "table":
		return displayStatusTable(cmd, status, verbose)
	case "pretty":
		return displayStatusPretty(cmd, status, verbose)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// displayStatusJSON outputs status as JSON
func displayStatusJSON(cmd *cobra.Command, status *intent_workflow.StatusData, verbose bool) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// displayStatusTable outputs status in table format
func displayStatusTable(cmd *cobra.Command, status *intent_workflow.StatusData, verbose bool) error {
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
		maxShow := 5
		if verbose {
			maxShow = len(status.Candidates)
		}
		for i, cand := range status.Candidates {
			if i >= maxShow {
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (score: %.2f)\n", i+1, cand.FilePath, cand.Score)
		}
		if !verbose && len(status.Candidates) > 5 {
			fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more (use --verbose to see all)\n", len(status.Candidates)-5)
		}
	}

	// Display succinct natural language response if available
	if resp := getSuccinctResponse(status.Turns); resp != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %s\n", resp)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nProcess: PID %d (%v)\n", status.ProcessInfo.PID, status.ProcessInfo.Running)

	if status.Error != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nError: %s\n", *status.Error)
	}

	if status.NextAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNext Action: %s\n", status.NextAction.Message)
	}

	if status.CorrectionAttempts > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nCorrection:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Attempts: %d\n", status.CorrectionAttempts)
		fmt.Fprintf(cmd.OutOrStdout(), "  Model: %s\n", status.CorrectionModel)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", status.CorrectionStatus)
		if status.CorrectionCost != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Cost: $%.6f\n", *status.CorrectionCost)
		}
		if status.TotalCost != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Total Cost: $%.6f\n", *status.TotalCost)
		}
	}

	return nil
}

// displayStatusPretty outputs status in a user-friendly format
func displayStatusPretty(cmd *cobra.Command, status *intent_workflow.StatusData, verbose bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\n  Intent Workflow Session: %s\n", status.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "  ===================================================\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", colorizeStatus(getDisplayStatus(status)))
	
	// Show session directory
	if status.SessionDir != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Directory: %s\n", status.SessionDir)
	}
	
	// Show phase name instead of turn number
	if status.Phase != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Phase:  %s\n", getPhaseDisplayName(status.Phase))
	}
	
	fmt.Fprintf(cmd.OutOrStdout(), "  Process: PID %d", status.ProcessInfo.PID)

	if status.ProcessInfo.Running {
		fmt.Fprintf(cmd.OutOrStdout(), " (running)")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), " (stopped)")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	var elapsed time.Duration
	if status.CompletedAt != nil {
		elapsed = status.CompletedAt.Sub(status.StartedAt)
	} else {
		elapsed = time.Since(status.StartedAt)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Elapsed: %v\n", elapsed)

	if len(status.WorkingDirectories) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Working Directories: %d\n", len(status.WorkingDirectories))
		for _, wd := range status.WorkingDirectories {
			fmt.Fprintf(cmd.OutOrStdout(), "    • %s\n", wd.Name)
		}
	}

	// Show current log file path
	if status.CurrentLogPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Log File: %s\n", status.CurrentLogPath)
	}

	if status.TotalFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Candidates Found: %d\n", status.TotalFound)
		if len(status.Candidates) > 0 {
			maxShow := 5
			if verbose {
				maxShow = len(status.Candidates)
			}
			if maxShow > len(status.Candidates) {
				maxShow = len(status.Candidates)
			}
			for i := 0; i < maxShow; i++ {
				cand := status.Candidates[i]
				fmt.Fprintf(cmd.OutOrStdout(), "    %d. %s (%.1f%%)\n", i+1, cand.FilePath, cand.Score*100)
			}
			if !verbose && len(status.Candidates) > maxShow {
				fmt.Fprintf(cmd.OutOrStdout(), "    ... and %d more\n", len(status.Candidates)-maxShow)
			}
		}
	}

	// Display succinct natural language response if available
	if resp := getSuccinctResponse(status.Turns); resp != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Summary: %s\n", resp)
	}

	// Show full results in verbose mode
	if verbose && len(status.Turns) > 0 {
		// Find the current turn
		consolidated := buildConsolidatedCandidates(status.Turns)
		
		var currentTurn *intent_workflow.TurnState
		for i := range status.Turns {
			if status.Turns[i].TurnNumber == status.CurrentTurn {
				currentTurn = &status.Turns[i]
				break
			}
		}
		
		if currentTurn != nil && currentTurn.Result != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Full Results:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  --------------------------------------------------\n")
			
			// Show discovery log if available
			if currentTurn.Result.Discovery != nil && currentTurn.Result.Discovery.DiscoveryLog != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Discovery Log:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "    Intent Keywords: %v\n", currentTurn.Result.Discovery.DiscoveryLog.IntentKeywords)
				fmt.Fprintf(cmd.OutOrStdout(), "    Methodology: %s\n", currentTurn.Result.Discovery.DiscoveryLog.Methodology)
				if len(currentTurn.Result.Discovery.DiscoveryLog.PivotChecks) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    Pivot Checks:\n")
					for _, check := range currentTurn.Result.Discovery.DiscoveryLog.PivotChecks {
						fmt.Fprintf(cmd.OutOrStdout(), "      - %s\n", check)
					}
				}
			}
			
			// Show validation summary if available
			if currentTurn.Result.Discovery != nil && currentTurn.Result.Discovery.ValidationSummary != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Validation Summary:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "    Total Validated: %d\n", currentTurn.Result.Discovery.ValidationSummary.TotalValidated)
				fmt.Fprintf(cmd.OutOrStdout(), "    Promoted: %d\n", currentTurn.Result.Discovery.ValidationSummary.CandidatesPromoted)
				fmt.Fprintf(cmd.OutOrStdout(), "    Demoted: %d\n", currentTurn.Result.Discovery.ValidationSummary.CandidatesDemoted)
				fmt.Fprintf(cmd.OutOrStdout(), "    Removed: %d\n", currentTurn.Result.Discovery.ValidationSummary.CandidatesRemoved)
				fmt.Fprintf(cmd.OutOrStdout(), "    Average Score: %.2f\n", currentTurn.Result.Discovery.ValidationSummary.AverageValidatedScore)
			}
			
			// Show coverage if available
			if currentTurn.Result.Discovery != nil && currentTurn.Result.Discovery.Coverage != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Coverage: %s\n", currentTurn.Result.Discovery.Coverage)
			}
			
			// Show session metrics if available
			if currentTurn.Usage != nil || currentTurn.Cost != nil || currentTurn.Duration != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Session Metrics:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  --------------------------------------------------\n")
				
				if currentTurn.Duration != nil {
					duration := time.Duration(*currentTurn.Duration) * time.Millisecond
					fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %v\n", duration.Round(time.Millisecond))
				}
				
				if currentTurn.Cost != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Cost: $%.6f\n", *currentTurn.Cost)
				}
				
				if currentTurn.Usage != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Usage:\n")
					fmt.Fprintf(cmd.OutOrStdout(), "    Input Tokens: %d\n", currentTurn.Usage.InputTokens)
					fmt.Fprintf(cmd.OutOrStdout(), "    Output Tokens: %d\n", currentTurn.Usage.OutputTokens)
					if currentTurn.Usage.CacheCreationTokens > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "    Cache Creation Tokens: %d\n", currentTurn.Usage.CacheCreationTokens)
					}
					if currentTurn.Usage.CacheReadTokens > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "    Cache Read Tokens: %d\n", currentTurn.Usage.CacheReadTokens)
					}
				}
			}
			
			// Show detailed candidates with reasoning
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Detailed Candidates:\n")
			for i, cand := range consolidated.ValidationCandidates {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  %d. %s\n", i+1, cand.FilePath)
				fmt.Fprintf(cmd.OutOrStdout(), "     Score: %.2f\n", cand.LatestValidationScore)
				
				if cand.Reasoning != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "     Reasoning: %s\n", cand.Reasoning)
				}
				
				// Show score comparison if both discovery and validation scores exist
				if cand.LatestDiscoveryScore > 0 {
					change := cand.LatestValidationScore - cand.LatestDiscoveryScore
					if change > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "     Score Change: ↑ %.1f%% (from %.1f%% in discovery turn %d)\n", 
							change*100, cand.LatestDiscoveryScore*100, cand.LatestDiscoveryTurn)
					} else if change < 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "     Score Change: ↓ %.1f%% (from %.1f%% in discovery turn %d)\n", 
							-change*100, cand.LatestDiscoveryScore*100, cand.LatestDiscoveryTurn)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "     Score Change: unchanged (from %.1f%% in discovery turn %d)\n", 
							cand.LatestDiscoveryScore*100, cand.LatestDiscoveryTurn)
					}
				}
				
				if len(cand.Metadata.Keywords) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "     Keywords: %v\n", cand.Metadata.Keywords)
				}
				if len(cand.Metadata.ParentKeywords) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "     Parent Keywords: %v\n", cand.Metadata.ParentKeywords)
				}
			}
		}
	}

	if status.CorrectionAttempts > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Correction:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "    Attempts: %d\n", status.CorrectionAttempts)
		fmt.Fprintf(cmd.OutOrStdout(), "    Model: %s\n", status.CorrectionModel)
		fmt.Fprintf(cmd.OutOrStdout(), "    Status: %s\n", status.CorrectionStatus)
		if status.CorrectionCost != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "    Correction Cost: $%.6f\n", *status.CorrectionCost)
		}
		if status.TotalCost != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "    Total Cost: $%.6f\n", *status.TotalCost)
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

// getPhaseDisplayName returns a friendly name for the phase
func getPhaseDisplayName(phase string) string {
	switch phase {
	case "discovery":
		return "Discovery"
	case "validation":
		return "Validation"
	case "change":
		return "Change"
	default:
		return phase
	}
}

// colorizeStatus returns a colorized status string using ANSI codes
func colorizeStatus(status string) string {
	switch status {
	case "discovery", "discovery_complete", "validation", "validation_complete", "change", "change_complete":
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

// getDisplayStatus returns the appropriate status string for display
// If status is empty but error is present, returns "Error"
func getDisplayStatus(status *intent_workflow.StatusData) string {
	if status.Status == "" && status.Error != nil {
		return "Error"
	}
	if status.Status == "stopped" && status.Error != nil {
		return "Error"
	}
	return status.Status
}

// getSuccinctResponse returns the most recent discovery turn's natural language
// response, or an empty string if none is available.
func getSuccinctResponse(turns []intent_workflow.TurnState) string {
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		if t.TurnType == "discovery" && t.Result != nil && t.Result.Discovery != nil {
			return t.Result.Discovery.SuccinctNaturalLanguageResponse
		}
	}
	return ""
}

// followSessionEvents streams events from the log file as they arrive
func followSessionEvents(cmd *cobra.Command, config *intent_workflow.SessionConfig) error {
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

			// Read session.json to get the current turn's log path
			session, err := intent_workflow.NewProcessorHelper(config).ReadSession(config.SessionID)
			if err != nil {
				// Session file not yet created
				continue
			}

			// Find the current turn's log path
			var logFile string
			for _, turn := range session.Turns {
				if turn.Status == "running" {
					logFile = turn.LogPath
					break
				}
			}
			
			// If no running turn, use the last turn
			if logFile == "" && len(session.Turns) > 0 {
				logFile = session.Turns[len(session.Turns)-1].LogPath
			}
			
			// If still no log file, skip this iteration
			if logFile == "" {
				continue
			}

			reader, err := intent_workflow.NewEventReader(logFile)
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
func displayEvent(cmd *cobra.Command, event *intent_workflow.StreamEvent) {
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

// buildConsolidatedCandidates builds a consolidated list of candidates with per-file score tracking
// Iterates turns in reverse to capture the latest scores for each file
func buildConsolidatedCandidates(turns []intent_workflow.TurnState) *ConsolidatedCandidates {
	fileScores := make(map[string]*FileScoreTracking)
	
	// Iterate turns in reverse (newest first)
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		
		// Process candidates from TurnResult.Discovery
		if turn.Result != nil && turn.Result.Discovery != nil {
			for _, cand := range turn.Result.Discovery.Candidates {
				key := cand.FilePath
				
				if fileScores[key] == nil {
					fileScores[key] = &FileScoreTracking{
						FilePath: key,
					}
				}
				
				if turn.TurnType == "discovery" {
					// Only record if we haven't seen a discovery score yet (going backwards)
					if fileScores[key].LatestDiscoveryTurn == 0 {
						fileScores[key].LatestDiscoveryScore = cand.Score
						fileScores[key].LatestDiscoveryTurn = turn.TurnNumber
					}
				} else if turn.TurnType == "validation" {
					// Only record if we haven't seen a validation score yet (going backwards)
					if fileScores[key].LatestValidationTurn == 0 {
						fileScores[key].LatestValidationScore = cand.Score
						fileScores[key].LatestValidationTurn = turn.TurnNumber
					}
				}
			}
		}
		
		// Process candidates from full results (for reasoning and metadata)
		if turn.Result != nil && turn.Result.Discovery != nil {
			for _, cand := range turn.Result.Discovery.Candidates {
				key := cand.FilePath
				
				if fileScores[key] == nil {
					fileScores[key] = &FileScoreTracking{
						FilePath: key,
					}
				}
				
				// Store reasoning and metadata from the most recent turn
				if cand.Reasoning != "" {
					fileScores[key].Reasoning = cand.Reasoning
				}
				if len(cand.BrainMetadata.Keywords) > 0 || len(cand.BrainMetadata.ParentKeywords) > 0 {
					fileScores[key].Metadata = cand.BrainMetadata
				}
			}
		}
	}
	
	// Build consolidated lists
	result := &ConsolidatedCandidates{
		DiscoveryCandidates:    []FileScoreTracking{},
		ValidationCandidates: []FileScoreTracking{},
	}
	
	for _, tracking := range fileScores {
		if tracking.LatestDiscoveryScore > 0 {
			result.DiscoveryCandidates = append(result.DiscoveryCandidates, *tracking)
		}
		if tracking.LatestValidationScore > 0 {
			result.ValidationCandidates = append(result.ValidationCandidates, *tracking)
		}
	}
	
	// Sort by score (descending)
	sort.Slice(result.DiscoveryCandidates, func(i, j int) bool {
		return result.DiscoveryCandidates[i].LatestDiscoveryScore > result.DiscoveryCandidates[j].LatestDiscoveryScore
	})
	sort.Slice(result.ValidationCandidates, func(i, j int) bool {
		return result.ValidationCandidates[i].LatestValidationScore > result.ValidationCandidates[j].LatestValidationScore
	})
	
	return result
}

// buildConsolidatedCandidatesLegacy builds a consolidated list of candidates with per-file score tracking
// This is the legacy version that accesses candidates from TurnResult.Discovery
// Kept for backward compatibility with old session files
func buildConsolidatedCandidatesLegacy(turns []intent_workflow.TurnState) *ConsolidatedCandidates {
	fileScores := make(map[string]*FileScoreTracking)
	
	// Iterate turns in reverse (newest first)
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		
		// Process candidates from TurnResult.Discovery (legacy)
		if turn.Result != nil && turn.Result.Discovery != nil {
			for _, cand := range turn.Result.Discovery.Candidates {
				key := cand.FilePath
				
				if fileScores[key] == nil {
					fileScores[key] = &FileScoreTracking{
						FilePath: key,
					}
				}
				
				if turn.TurnType == "discovery" {
					// Only record if we haven't seen a discovery score yet (going backwards)
					if fileScores[key].LatestDiscoveryTurn == 0 {
						fileScores[key].LatestDiscoveryScore = cand.Score
						fileScores[key].LatestDiscoveryTurn = turn.TurnNumber
					}
				} else if turn.TurnType == "validation" {
					// Only record if we haven't seen a validation score yet (going backwards)
					if fileScores[key].LatestValidationTurn == 0 {
						fileScores[key].LatestValidationScore = cand.Score
						fileScores[key].LatestValidationTurn = turn.TurnNumber
					}
				}
			}
		}
		
		// Process candidates from full results (for reasoning and metadata)
		if turn.Result != nil && turn.Result.Discovery != nil {
			for _, cand := range turn.Result.Discovery.Candidates {
				key := cand.FilePath
				
				if fileScores[key] == nil {
					fileScores[key] = &FileScoreTracking{
						FilePath: key,
					}
				}
				
				// Store reasoning and metadata from the most recent turn
				if cand.Reasoning != "" {
					fileScores[key].Reasoning = cand.Reasoning
				}
				if len(cand.BrainMetadata.Keywords) > 0 || len(cand.BrainMetadata.ParentKeywords) > 0 {
					fileScores[key].Metadata = cand.BrainMetadata
				}
			}
		}
	}
	
	// Build consolidated lists
	result := &ConsolidatedCandidates{
		DiscoveryCandidates:    []FileScoreTracking{},
		ValidationCandidates: []FileScoreTracking{},
	}
	
	for _, tracking := range fileScores {
		if tracking.LatestDiscoveryScore > 0 {
			result.DiscoveryCandidates = append(result.DiscoveryCandidates, *tracking)
		}
		if tracking.LatestValidationScore > 0 {
			result.ValidationCandidates = append(result.ValidationCandidates, *tracking)
		}
	}
	
	// Sort by score (descending)
	sort.Slice(result.DiscoveryCandidates, func(i, j int) bool {
		return result.DiscoveryCandidates[i].LatestDiscoveryScore > result.DiscoveryCandidates[j].LatestDiscoveryScore
	})
	sort.Slice(result.ValidationCandidates, func(i, j int) bool {
		return result.ValidationCandidates[i].LatestValidationScore > result.ValidationCandidates[j].LatestValidationScore
	})
	
	return result
}
