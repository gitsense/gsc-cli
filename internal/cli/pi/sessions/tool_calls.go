/**
 * Component: Pi Sessions Tool Calls Command
 * Block-UUID: f6a7b8c9-d0e1-2345-fabc-678901234567
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc pi sessions tool-calls for viewing tool calls from a session.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"fmt"

	sessionspkg "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

func toolCallsCmd() *cobra.Command {
	var (
		sessionPath string
		leafID      string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "tool-calls",
		Short: "Show tool calls from the active branch",
		Long: `Display tool calls and their results from the active branch of a Pi JSONL session file.

Tool calls are joined with their results by toolCallId. This is useful for
understanding what operations were performed during a session.`,
		Example: `  # Show tool calls as JSON
  gsc pi sessions tool-calls --session /path/to/session.jsonl --leaf entry-123 --format json

  # Show tool calls as human-readable list
  gsc pi sessions tool-calls --session /path/to/session.jsonl --leaf entry-123`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionPath == "" {
				return fmt.Errorf("--session is required")
			}
			if leafID == "" {
				return fmt.Errorf("--leaf is required")
			}

			result, err := sessionspkg.ExtractToolCalls(sessionPath, leafID)
			if err != nil {
				return fmt.Errorf("failed to extract tool calls: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			default:
				printToolCallsHuman(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to session JSONL file (required)")
	cmd.Flags().StringVar(&leafID, "leaf", "", "Leaf entry ID (required)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")

	return cmd
}

func printToolCallsHuman(result *sessionspkg.ToolCallsResult) {
	fmt.Printf("Session: %s\n", result.Session.Path)
	fmt.Printf("ID: %s\n", result.Session.ID)
	fmt.Printf("CWD: %s\n", result.Session.CWD)
	fmt.Printf("Leaf: %s\n", result.Leaf)
	fmt.Printf("Tool Calls: %d\n\n", len(result.ToolCalls))

	for i, tc := range result.ToolCalls {
		status := "✓"
		if tc.IsError {
			status = "✗"
		}
		fmt.Printf("[%d] %s %s %s (%s)\n", i, status, tc.ToolName, tc.ToolCallID, tc.Timestamp)
		if tc.ResultText != "" {
			preview := tc.ResultText
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("    Result: %s\n", preview)
		}
	}
}
