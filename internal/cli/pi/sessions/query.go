/**
 * Component: Pi Sessions Query Command
 * Block-UUID: 28c28a9f-e833-4bdd-96dd-7a8c6ab25cf0
 * Parent-UUID: N/A
 * Version: 1.3.0
 * Description: Implements phase-one discovery queries over the Pi sessions SQLite mirror.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v1.1.0, v1.2.0, v1.3.0)
 */

package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

// hiddenTextFlagName is the flag name for the hidden --text alias.
const hiddenTextFlagName = "text"

// hiddenTypeFlagName is the flag name for the hidden --type alias.
const hiddenTypeFlagName = "type"

// ANSI color codes
const (
	ansiReset  = "\033[0m"
	ansiYellow = "\033[1;33m"
)

func queryCmd() *cobra.Command {
	var options pisessions.QueryOptions
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:          "query",
		Short:        "Query the Pi sessions mirror",
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

			view := strings.ToLower(options.View)
			if view == "" {
				view = "events"
			}

			if view == "sessions" {
				results, err := pisessions.QuerySessions(cmd.Context(), options)
				if err != nil {
					return err
				}
				return writeSessionResults(results, format)
			}

			results, err := pisessions.Query(cmd.Context(), options)
			if err != nil {
				return err
			}
			return writeQueryResults(results, format, options.WithBranches, options.Color)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi-sessions.sqlite3)")
	cmd.Flags().StringVar(&options.View, "view", "events", "Query view: events (flat) or sessions (aggregated)")
	cmd.Flags().StringVar(&options.File, "file", "", "Repo-root-relative file path to recall")
	cmd.Flags().StringVar(&options.AbsFile, "abs-file", "", "Absolute file path to recall")
	cmd.Flags().StringVar(&options.Repo, "repo", "", "Repo root filter")
	cmd.Flags().StringVar(&options.SessionID, "session-id", "", "Pi session ID filter")
	cmd.Flags().StringVar(&options.SessionName, "session-name", "", "Exact session name match")
	cmd.Flags().StringVar(&options.SessionNamePrefix, "session-name-starts-with", "", "Session name prefix match")
	cmd.Flags().StringVar(&options.Tool, "tool", "", "Tool name filter (bash, read, edit, write)")
	cmd.Flags().StringVar(&options.Op, "op", "", "File operation filter (read, edit, write)")
	cmd.Flags().StringVarP(&options.Text, "message", "q", "", "Full-text search over user/assistant messages")
	cmd.Flags().StringVar(&options.Text, hiddenTextFlagName, "", "")
	cmd.Flags().MarkHidden(hiddenTextFlagName)
	cmd.Flags().StringVar(&options.CommandStartsWith, "command-starts-with", "", "Bash command prefix match (implies --tool bash)")
	cmd.Flags().StringVar(&options.CommandContains, "command-contains", "", "Bash command substring match (implies --tool bash)")
	cmd.Flags().StringVar(&options.OutputContains, "output-contains", "", "Tool output substring match")
	cmd.Flags().StringVar(&options.ToolArgsContains, "tool-args-contains", "", "Tool arguments JSON substring match")
	cmd.Flags().BoolVarP(&options.CaseInsensitive, "case-insensitive", "i", false, "Case-insensitive matching for --command-*, --output-*, --tool-args-*")
	cmd.Flags().StringVar(&options.Since, "since", "", "Inclusive lower timestamp bound")
	cmd.Flags().StringVar(&options.Until, "until", "", "Inclusive upper timestamp bound")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Provider filter")
	cmd.Flags().StringVar(&options.Model, "model", "", "Model filter")
	cmd.Flags().StringVar(&options.EntryType, "entry-type", "", "Entry type filter (message, model_change, compaction, etc.)")
	cmd.Flags().StringVar(&options.Type, hiddenTypeFlagName, "", "")
	cmd.Flags().MarkHidden(hiddenTypeFlagName)
	cmd.Flags().StringVar(&options.Role, "role", "", "Message role filter")
	cmd.Flags().StringVar(&options.EntryID, "entry", "", "Entry id filter")
	cmd.Flags().StringVar(&options.Sort, "sort", "recent", "Sort order: recent, oldest, match-count")
	cmd.Flags().BoolVar(&options.WithBranches, "with-branches", false, "Enrich results with branch metadata (branch_leaf_ids, nearest_compaction_id, nearest_branch_summary_id)")
	cmd.Flags().StringVar(&options.Color, "color", "auto", "Color output: auto, always, never")
	cmd.Flags().IntVar(&options.Limit, "limit", 50, "Maximum results")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

// useColor returns true if ANSI colors should be used.
func useColor(colorOption string) bool {
	switch strings.ToLower(colorOption) {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		// Check if stdout is a terminal
		fi, err := os.Stdout.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
}

// highlightText applies ANSI highlighting to regions of text based on match ranges.
func highlightText(text string, ranges []pisessions.MatchRange, useAnsi bool) string {
	if len(ranges) == 0 {
		return text
	}

	if !useAnsi {
		// Use brackets for non-ANSI mode
		return applyBrackets(text, ranges)
	}

	// Apply ANSI colors
	var result strings.Builder
	result.Grow(len(text) + len(ranges)*20)
	lastEnd := 0
	for _, r := range ranges {
		if r.Start > lastEnd {
			result.WriteString(text[lastEnd:r.Start])
		}
		if r.Start < len(text) && r.End <= len(text) {
			result.WriteString(ansiYellow)
			result.WriteString(text[r.Start:r.End])
			result.WriteString(ansiReset)
		}
		lastEnd = r.End
	}
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}
	return result.String()
}

// applyBrackets wraps matched regions with brackets.
func applyBrackets(text string, ranges []pisessions.MatchRange) string {
	if len(ranges) == 0 {
		return text
	}

	var result strings.Builder
	result.Grow(len(text) + len(ranges)*2)
	lastEnd := 0
	for _, r := range ranges {
		if r.Start > lastEnd {
			result.WriteString(text[lastEnd:r.Start])
		}
		if r.Start < len(text) && r.End <= len(text) {
			result.WriteByte('[')
			result.WriteString(text[r.Start:r.End])
			result.WriteByte(']')
		}
		lastEnd = r.End
	}
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}
	return result.String()
}

func writeQueryResults(results []pisessions.QueryResult, format string, withBranches bool, colorOption string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "human", "":
		useAnsi := useColor(colorOption)
		for _, result := range results {
			fmt.Printf("%s", result.Kind)
			if result.SessionID != "" {
				fmt.Printf(" %s", result.SessionID)
			}
			if result.Timestamp != "" {
				fmt.Printf(" %s", result.Timestamp)
			}
			if result.Command != "" {
				fmt.Printf(" cmd=%s", result.Command)
			} else if result.FilePathRel != "" {
				fmt.Printf(" %s", result.FilePathRel)
			} else if result.AbsPath != "" {
				fmt.Printf(" %s", result.AbsPath)
			}
			if result.ToolName != "" {
				fmt.Printf(" tool=%s", result.ToolName)
			}
			if result.Op != "" {
				fmt.Printf(" op=%s", result.Op)
			}
			if result.EntryID != "" {
				fmt.Printf(" entry=%s", result.EntryID)
			}
			if result.Text != "" {
				// Use highlighted snippet if available, otherwise plain text
				if len(result.MatchRanges) > 0 {
					highlighted := highlightText(result.Text, result.MatchRanges, useAnsi)
					fmt.Printf("\n  %s", highlighted)
				} else {
					fmt.Printf("\n  %s", result.Text)
				}
			}
			// Branch enrichment output
			if withBranches {
				if len(result.BranchLeafIDs) > 0 {
					fmt.Printf("\n  branch leaves: %s", strings.Join(result.BranchLeafIDs, ", "))
				}
				if result.NearestCompactionID != "" {
					fmt.Printf("\n  nearest compaction: %s", result.NearestCompactionID)
				}
				if result.NearestBranchSummaryID != "" {
					fmt.Printf("\n  nearest branch summary: %s", result.NearestBranchSummaryID)
				}
			}
			fmt.Println()
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeSessionResults(results []pisessions.SessionQueryResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "human", "":
		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			writeSessionHuman(r)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeSessionHuman(r pisessions.SessionQueryResult) {
	// UUID prefix (12 chars)
	uuidPrefix := r.SessionID
	if len(uuidPrefix) > 12 {
		uuidPrefix = uuidPrefix[:12]
	}

	// Title
	title := r.Title
	if title == "" {
		title = "(no messages)"
	}

	// Relative time
	createdAgo := relativeTime(r.CreatedAt)

	// Line 1: UUID, timestamp, title
	fmt.Printf("%s  %s  %s  %s\n", uuidPrefix, formatTimestamp(r.CreatedAt), createdAgo, truncate(title, 80))

	// Location: prefer repo, else cwd
	if r.RepoRoot != "" {
		cwdSuffix := ""
		if r.CWD != "" && r.CWD != r.RepoRoot {
			rel, err := filepath.Rel(r.RepoRoot, r.CWD)
			if err == nil && !strings.HasPrefix(rel, "..") {
				cwdSuffix = "  cwd: " + rel
			}
		}
		fmt.Printf("  repo: ~/%s%s\n", homeRelative(r.RepoRoot), cwdSuffix)
	} else if r.CWD != "" {
		fmt.Printf("  cwd: ~/%s\n", homeRelative(r.CWD))
	}

	// Last activity + duration
	if r.LastMessageAt != "" {
		lastAgo := relativeTime(r.LastMessageAt)
		duration := computeDuration(r.CreatedAt, r.LastMessageAt)
		fmt.Printf("  last: %s  %s  duration: %s\n", formatTimestamp(r.LastMessageAt), lastAgo, duration)
	}

	// Totals
	fmt.Printf("  totals: %d messages | %d tools | %d file refs\n",
		r.MessageCount, r.ToolCallCount, r.FileRefCount)

	// Matches (if any)
	if r.MatchCount > 0 {
		parts := []string{}
		if r.MatchedFileRefCount > 0 {
			parts = append(parts, fmt.Sprintf("%d refs", r.MatchedFileRefCount))
		}
		if r.MatchedToolCallCount > 0 {
			parts = append(parts, fmt.Sprintf("%d tools", r.MatchedToolCallCount))
		}
		if r.MatchedMessageCount > 0 {
			parts = append(parts, fmt.Sprintf("%d messages", r.MatchedMessageCount))
		}
		if len(parts) > 0 {
			fmt.Printf("  matches: %s\n", strings.Join(parts, " | "))
		}
	}

	// Matched paths (capped at 5)
	if len(r.MatchedPaths) > 0 {
		limit := 5
		if len(r.MatchedPaths) <= limit {
			fmt.Printf("  files: %s\n", strings.Join(r.MatchedPaths, ", "))
		} else {
			shown := r.MatchedPaths[:limit]
			remaining := len(r.MatchedPaths) - limit
			fmt.Printf("  files: %s (+%d more)\n", strings.Join(shown, ", "), remaining)
		}
	}
}

func formatTimestamp(ts string) string {
	if ts == "" {
		return ""
	}
	// Try to parse and format as local time
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}

func relativeTime(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "(1 min ago)"
		}
		return fmt.Sprintf("(%d min ago)", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "(1 hour ago)"
		}
		return fmt.Sprintf("(%d hours ago)", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "(1 day ago)"
		}
		return fmt.Sprintf("(%d days ago)", days)
	}
}

func computeDuration(start, end string) string {
	t1, err1 := time.Parse(time.RFC3339, start)
	t2, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return "?"
	}
	d := t2.Sub(t1)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func homeRelative(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return strings.TrimPrefix(path, home)
	}
	return path
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
