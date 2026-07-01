/**
 * Component: Pi Sessions Branch Command
 * Block-UUID: e5f6a7b8-c9d0-1234-efab-567890123456
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc pi sessions branch for viewing the active branch of a session.
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

func branchCmd() *cobra.Command {
	var (
		sessionPath string
		leafID      string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Show the active branch of a Pi session",
		Long: `Display the active branch entries from root to leaf in a Pi JSONL session file.

This is a read-only command that parses the session file and walks the parent
chain from the specified leaf back to the root entry.`,
		Example: `  # Show branch as JSON
  gsc pi sessions branch --session /path/to/session.jsonl --leaf entry-123 --format json

  # Show branch as human-readable table
  gsc pi sessions branch --session /path/to/session.jsonl --leaf entry-123`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionPath == "" {
				return fmt.Errorf("--session is required")
			}
			if leafID == "" {
				return fmt.Errorf("--leaf is required")
			}

			result, err := sessionspkg.WalkBranch(sessionPath, leafID)
			if err != nil {
				return fmt.Errorf("failed to walk branch: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			default:
				printBranchHuman(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to session JSONL file (required)")
	cmd.Flags().StringVar(&leafID, "leaf", "", "Leaf entry ID (required)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")

	return cmd
}

func printBranchHuman(result *sessionspkg.BranchResult) {
	fmt.Printf("Session: %s\n", result.Session.Path)
	fmt.Printf("ID: %s\n", result.Session.ID)
	fmt.Printf("CWD: %s\n", result.Session.CWD)
	fmt.Printf("Leaf: %s\n", result.Leaf)
	fmt.Printf("Entries: %d\n\n", len(result.Entries))

	for i, entry := range result.Entries {
		role := entry.Role
		if role == "" {
			role = entry.Type
		}
		fmt.Printf("[%d] %s %s %s\n", i, entry.ID, role, entry.Timestamp)
	}
}
