/**
 * Component: Pi Sessions Files Command
 * Block-UUID: a7b8c9d0-e1f2-3456-abcd-789012345678
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc pi sessions files for viewing file references from a session.
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

func filesCmd() *cobra.Command {
	var (
		sessionPath string
		leafID      string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "files",
		Short: "Show file references from the active branch",
		Long: `Display file references (read, edit, write) from the active branch of a Pi JSONL session file.

Files are extracted from:
- Tool calls: read, edit, write
- Branch summary fields: readFiles, modifiedFiles`,
		Example: `  # Show files as JSON
  gsc pi sessions files --session /path/to/session.jsonl --leaf entry-123 --format json

  # Show files as human-readable list
  gsc pi sessions files --session /path/to/session.jsonl --leaf entry-123`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionPath == "" {
				return fmt.Errorf("--session is required")
			}
			if leafID == "" {
				return fmt.Errorf("--leaf is required")
			}

			result, err := sessionspkg.ExtractFiles(sessionPath, leafID)
			if err != nil {
				return fmt.Errorf("failed to extract files: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			default:
				printFilesHuman(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to session JSONL file (required)")
	cmd.Flags().StringVar(&leafID, "leaf", "", "Leaf entry ID (required)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")

	return cmd
}

func printFilesHuman(result *sessionspkg.FilesResult) {
	fmt.Printf("Session: %s\n", result.Session.Path)
	fmt.Printf("ID: %s\n", result.Session.ID)
	fmt.Printf("CWD: %s\n", result.Session.CWD)
	fmt.Printf("Leaf: %s\n", result.Leaf)
	fmt.Printf("Files: %d\n\n", len(result.Files))

	for _, f := range result.Files {
		fmt.Printf("  %s\t%s\t(%s)\n", f.Op, f.Path, f.Source)
	}
}
