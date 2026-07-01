/**
 * Component: Pi Sessions Errors Command
 * Block-UUID: b8c9d0e1-f2a3-4567-bcde-890123456789
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc pi sessions errors for viewing failed tool results from a session.
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

func errorsCmd() *cobra.Command {
	var (
		sessionPath string
		leafID      string
		tool        string
		contains    string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "errors",
		Short: "Show failed tool results from the active branch",
		Long: `Display failed tool results from the active branch of a Pi JSONL session file.

Use --tool to filter by tool name and --contains to filter by error text.`,
		Example: `  # Show all errors as JSON
  gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --format json

  # Show only bash errors
  gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --tool bash --format json

  # Show errors containing specific text
  gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --contains TS2307 --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionPath == "" {
				return fmt.Errorf("--session is required")
			}
			if leafID == "" {
				return fmt.Errorf("--leaf is required")
			}

			opts := sessionspkg.ErrorsOptions{
				Tool:     tool,
				Contains: contains,
			}

			result, err := sessionspkg.ExtractErrors(sessionPath, leafID, opts)
			if err != nil {
				return fmt.Errorf("failed to extract errors: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			default:
				printErrorsHuman(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to session JSONL file (required)")
	cmd.Flags().StringVar(&leafID, "leaf", "", "Leaf entry ID (required)")
	cmd.Flags().StringVar(&tool, "tool", "", "Filter by tool name")
	cmd.Flags().StringVar(&contains, "contains", "", "Filter by substring in error text")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")

	return cmd
}

func printErrorsHuman(result *sessionspkg.ErrorsResult) {
	fmt.Printf("Session: %s\n", result.Session.Path)
	fmt.Printf("ID: %s\n", result.Session.ID)
	fmt.Printf("CWD: %s\n", result.Session.CWD)
	fmt.Printf("Leaf: %s\n", result.Leaf)
	fmt.Printf("Errors: %d\n\n", len(result.Errors))

	for i, err := range result.Errors {
		fmt.Printf("[%d] %s %s (%s)\n", i, err.ToolName, err.ToolCallID, err.Timestamp)
		preview := err.ResultText
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("    %s\n", preview)
	}
}
