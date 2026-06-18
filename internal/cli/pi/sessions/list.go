/**
 * Component: Pi Sessions List Command
 * Block-UUID: 4e8f2a1c-9b3d-4e5f-8a7c-1d2e3f4a5b6c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Lists Pi sessions with compact one-line-per-session format.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: MiMo-v2.5-pro
 */

package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var options pisessions.ListOptions
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List Pi sessions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDB, err := resolvePiSessionsDBPath(dbPath)
			if err != nil {
				return err
			}
			options.DBPath = resolvedDB
			if options.Limit == 0 {
				options.Limit = 50
			}

			results, err := pisessions.List(cmd.Context(), options)
			if err != nil {
				return err
			}
			return writeListResults(results, format)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi-sessions.sqlite3)")
	cmd.Flags().StringVar(&options.Repo, "repo", "", "Repo root filter")
	cmd.Flags().StringVar(&options.Since, "since", "", "Inclusive lower timestamp bound")
	cmd.Flags().StringVar(&options.Until, "until", "", "Inclusive upper timestamp bound")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Provider filter")
	cmd.Flags().StringVar(&options.Model, "model", "", "Model filter")
	cmd.Flags().StringVar(&options.Sort, "sort", "recent", "Sort order: recent, oldest, messages")
	cmd.Flags().IntVar(&options.Limit, "limit", 50, "Maximum results")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

func writeListResults(results []pisessions.ListResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "human", "":
		return writeListHuman(results)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeListHuman(results []pisessions.ListResult) error {
	if len(results) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	for _, r := range results {
		// Short ID (13 chars: 019edc1e-bf7f)
		shortID := r.SessionID
		if len(shortID) > 13 {
			shortID = shortID[:13]
		}

		// Last activity date (Jun 18 19:29)
		dateStr := formatListDate(r.LastMessageAt)

		// Message count (right-aligned, 4 digits)
		countStr := fmt.Sprintf("%4d msgs", r.MessageCount)

		// Path (home-relative, middle-truncated to 32 chars)
		path := formatListPath(r.RepoRoot, r.CWD)

		// Preview (max 60 chars)
		preview := truncateMiddle(r.LastUserText, 60)

		fmt.Printf("%s  %s  %s  %-32s  %s\n", shortID, dateStr, countStr, path, preview)
	}

	return nil
}

func formatListDate(ts string) string {
	if ts == "" {
		return "?"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?"
	}
	return t.Local().Format("Jan 02 15:04")
}

func formatListPath(repoRoot, cwd string) string {
	path := ""
	if repoRoot != "" {
		path = repoRoot
	} else if cwd != "" {
		path = cwd
	}

	if path == "" {
		return "?"
	}

	// Home-relative
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}

	// Middle-truncate if too long
	if len(path) > 32 {
		path = truncateMiddle(path, 32)
	}

	return path
}

func truncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Leave room for "..."
	half := (maxLen - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}
