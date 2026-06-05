/**
 * Component: Intent Workflow CLI Delete Command
 * Block-UUID: ecff260e-bd72-4230-b0e0-8dda1f646abd
 * Parent-UUID: 13150d10-d2f4-4cba-bc9f-a20eb3285e1b
 * Version: 1.2.0
 * Description: Implements 'gsc claude agent delete' command for deleting turns or entire sessions. Updated to allow delete on stopped and error states.
 * Language: Go
 * Created-at: 2026-04-20T15:39:08.380Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0), GLM-4.7 (v1.2.0)
 */


package intentworkflowcli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/spf13/cobra"
)

// DeleteResult represents the JSON response for the delete command
type DeleteResult struct {
	SessionID  string `json:"session_id"`
	DeleteType string `json:"delete_type"` // "turn" or "session"
	TurnNumber int    `json:"turn_number"`
	Status     string `json:"status"`
}

// DeleteCmd creates the "agent delete" subcommand
func DeleteCmd() *cobra.Command {
	flags := &DeleteFlags{}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a turn or entire session",
		Long: `Delete the latest turn or the entire session.

--type turn: Delete only the latest turn (requires 2+ turns)
--type session: Delete the entire session (any turn count)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeleteCommand(cmd, flags)
		},
	}

	RegisterDeleteFlags(cmd, flags)

	return cmd
}

// runDeleteCommand executes the delete command logic
func runDeleteCommand(cmd *cobra.Command, flags *DeleteFlags) error {
	// Load session
	manager, err := intent_workflow.LoadSession(flags.Session)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	session := manager.GetSession()
	turnCount := len(session.Turns)

	if turnCount == 0 {
		return fmt.Errorf("session has no turns to delete")
	}

	// Validate delete type value
	if flags.DeleteType != "turn" && flags.DeleteType != "session" {
		return fmt.Errorf("invalid delete type: %s (must be 'turn' or 'session')", flags.DeleteType)
	}

	// Validate delete type
	if flags.DeleteType == "turn" && turnCount == 1 {
		return fmt.Errorf("cannot delete the only turn. Use --type session to delete the entire session, or retry to restart the turn")
	}

	// Get latest turn
	lastTurn := session.Turns[turnCount-1]

	// Check if turn is running (only state that blocks delete)
	if lastTurn.Status == "running" {
		return fmt.Errorf("cannot delete turn: turn is still running (status: %s)", lastTurn.Status)
	}

	// Handle session deletion
	if flags.DeleteType == "session" {
		// Delete entire session directory
		if err := manager.GetConfig().CleanupSessionDir(); err != nil {
			return fmt.Errorf("failed to delete session directory: %w", err)
		}

		// Output result
		result := DeleteResult{
			SessionID:  flags.Session,
			DeleteType: "session",
			TurnNumber: lastTurn.TurnNumber,
			Status:     "deleted",
		}

		if flags.Format == "json" {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Session deleted\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
		}

		return nil
	}

	// Handle turn deletion
	// Delete turn directory
	turnDir := manager.GetConfig().GetTurnDir(lastTurn.TurnNumber)
	if err := os.RemoveAll(turnDir); err != nil {
		return fmt.Errorf("failed to delete turn directory: %w", err)
	}

	// Remove latest turn from session.Turns
	session.Turns = session.Turns[:turnCount-1]

	// Reset session state to previous turn's completion state
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
	}

	// Clear error and completion fields
	session.Error = nil
	session.CompletedAt = nil
	session.WatcherPID = nil

	// Write updated session state
	if err := manager.WriteSessionState(); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	// Output result
	result := DeleteResult{
		SessionID:  flags.Session,
		DeleteType: "turn",
		TurnNumber: lastTurn.TurnNumber,
		Status:     "deleted",
	}

	if flags.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Turn deleted\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Session ID: %s\n", flags.Session)
		fmt.Fprintf(cmd.OutOrStdout(), "  Turn: %d\n", lastTurn.TurnNumber)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", session.Status)
	}

	return nil
}

// DeleteFlags contains flags for the delete command
type DeleteFlags struct {
	Session   string
	DeleteType string
	Format    string
}

// RegisterDeleteFlags registers flags for the delete command
func RegisterDeleteFlags(cmd *cobra.Command, flags *DeleteFlags) {
	cmd.Flags().StringVarP(
		&flags.Session,
		"session", "s",
		"",
		"Session ID",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().StringVar(
		&flags.DeleteType,
		"type",
		"",
		"Delete type: 'turn' or 'session'",
	)
	cmd.MarkFlagRequired("type")

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
	)
}
