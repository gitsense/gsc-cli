/**
 * Component: Scout CLI Results Command
 * Block-UUID: 5d30c452-0b3c-42b2-b7cb-3a6bb41b553e
 * Parent-UUID: 467e59e3-80ad-46c3-b6fa-1b15b1e9adf6
 * Version: 1.0.1
 * Description: Implements 'gsc claude scout results' command for retrieving finalized turn results
 * Language: Go
 * Created-at: 2026-03-31T02:27:12.973Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1)
 */


package scoutcli

import (
	"encoding/json"
	"fmt"

	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
	"github.com/spf13/cobra"
)

// ResultsCmd creates the "scout results" subcommand
func ResultsCmd() *cobra.Command {
	flags := &ResultsFlags{}

	cmd := &cobra.Command{
		Use:   "results",
		Short: "Retrieve finalized results for a Scout turn",
		Long: `Retrieve the finalized candidates and metadata for a completed Scout turn.

This command returns lightweight results without process metadata or status information.
For Turn 2 (verification), the results include both verified candidates and the original
unvalidated candidates from Turn 1 discovery for reference.

Use this command when you need clean, finalized results for display or downstream processing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResultsCommand(cmd, flags)
		},
	}

	RegisterResultsFlags(cmd, flags)

	return cmd
}

// runResultsCommand executes the results command logic
func runResultsCommand(cmd *cobra.Command, flags *ResultsFlags) error {
	// Validate that unsupported flags are not set
	if err := ValidateScoutFlags(cmd); err != nil {
		return err
	}

	// Validate flags
	if err := ValidateResultsFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Load the session
	manager, err := claudescout.LoadSession(flags.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Get finalized results for the turn
	results, err := manager.GetFinalizedTurnResults(flags.Turn)
	if err != nil {
		// Check if it's a "not complete" error
		if err == claudescout.ErrTurnNotComplete {
			return fmt.Errorf(
				"turn %d has not yet completed. Check status with:\n"+
					"  gsc claude scout status -s %s",
				flags.Turn, flags.SessionID,
			)
		}
		return fmt.Errorf("failed to retrieve results: %w", err)
	}

	// Output based on format
	if flags.Format == "json" {
		return outputResultsJSON(cmd, results)
	}

	// Text format output
	return outputResultsText(cmd, results)
}

// outputResultsJSON outputs results as JSON
func outputResultsJSON(cmd *cobra.Command, results *claudescout.FinalizedTurnResults) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputResultsText outputs results in human-readable text format
func outputResultsText(cmd *cobra.Command, results *claudescout.FinalizedTurnResults) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Scout Results\n")
	fmt.Fprintf(cmd.OutOrStdout(), "=======================================\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", results.SessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "Turn: %d\n", results.Turn)
	fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", results.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// Display candidates
	fmt.Fprintf(cmd.OutOrStdout(), "Candidates (%d total):\n", results.TotalFound)
	fmt.Fprintf(cmd.OutOrStdout(), "---------------------------------------\n")

	if len(results.Candidates) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  (no candidates)\n")
	} else {
		for i, candidate := range results.Candidates {
			if i >= 10 {
				fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(results.Candidates)-i)
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (score: %.2f)\n", i+1, candidate.FilePath, candidate.Score)
			if candidate.Reasoning != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     -> %s\n", candidate.Reasoning)
			}
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// For Turn 2, display original candidates section
	if results.Turn == 2 && len(results.OriginalCandidates) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Original Unvalidated Candidates (Turn 1):\n")
		fmt.Fprintf(cmd.OutOrStdout(), "---------------------------------------\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Note: These are from discovery before verification\n\n")

		for i, candidate := range results.OriginalCandidates {
			if i >= 10 {
				fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(results.OriginalCandidates)-i)
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (score: %.2f)\n", i+1, candidate.FilePath, candidate.Score)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
	}

	return nil
}
