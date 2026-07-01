/**
 * Component: Pi Sessions Show Command
 * Block-UUID: 5f7a8b9c-0d1e-2f3a-4b5c-6d7e8f9a0b1c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Shows detailed information about a specific Pi session.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: MiMo-v2.5-pro
 */

package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

func showCmd() *cobra.Command {
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:          "show <session-id>",
		Short:        "Show detailed information about a session",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDB, err := resolvePiSessionsDBPath(dbPath)
			if err != nil {
				return err
			}

			sessionID := args[0]
			result, err := pisessions.Show(cmd.Context(), resolvedDB, sessionID)
			if err != nil {
				return err
			}
			return writeShowResult(result, format)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi/pi-sessions.sqlite3)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

func writeShowResult(result *pisessions.ShowResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "human", "":
		return writeShowHuman(result)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeShowHuman(r *pisessions.ShowResult) error {
	// Header
	fmt.Printf("Session %s\n", r.SessionID)
	fmt.Println(strings.Repeat("-", 75))

	// Name (if any)
	if r.Name != "" {
		fmt.Printf("  Name:       %s\n", r.Name)
	}

	// Location
	if r.RepoRoot != "" {
		location := "~/" + homeRelative(r.RepoRoot)
		if r.CWD != "" && r.CWD != r.RepoRoot {
			rel, err := filepath.Rel(r.RepoRoot, r.CWD)
			if err == nil && !strings.HasPrefix(rel, "..") {
				location += "/" + rel
			}
		}
		fmt.Printf("  Repo:       %s\n", location)
	} else if r.CWD != "" {
		fmt.Printf("  CWD:        ~/%s\n", homeRelative(r.CWD))
	}

	// Provider/Model
	if r.Provider != "" {
		fmt.Printf("  Provider:   %s\n", r.Provider)
	}
	if r.Model != "" {
		fmt.Printf("  Model:      %s\n", r.Model)
	}

	// Timestamps
	fmt.Printf("  Created:    %s (%s)\n", formatTimestamp(r.CreatedAt), relativeTime(r.CreatedAt))
	if r.LastMessageAt != "" {
		fmt.Printf("  Last msg:   %s (%s)\n", formatTimestamp(r.LastMessageAt), relativeTime(r.LastMessageAt))
	}
	if r.CreatedAt != "" && r.LastMessageAt != "" {
		duration := computeDuration(r.CreatedAt, r.LastMessageAt)
		fmt.Printf("  Duration:   %s\n", duration)
	}

	// Stats
	fmt.Println()
	fmt.Printf("  Messages:   %d\n", r.MessageCount)
	fmt.Printf("  Tool calls: %d\n", r.ToolCallCount)
	fmt.Printf("  File refs:  %d\n", r.FileRefCount)

	// First/last user text
	fmt.Println()
	if r.FirstUserText != "" {
		fmt.Printf("  First:      %s\n", truncate(r.FirstUserText, 72))
	}
	if r.LastUserText != "" {
		fmt.Printf("  Last user:  %s\n", truncate(r.LastUserText, 72))
	}
	if r.LastText != "" && r.LastText != r.LastUserText {
		fmt.Printf("  Last msg:   %s\n", truncate(r.LastText, 72))
	}

	return nil
}
