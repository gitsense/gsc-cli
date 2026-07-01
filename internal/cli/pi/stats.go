/**
 * Component: Pi Stats (Brains Flag Handler)
 * Block-UUID: [to-be-generated]
 * Parent-UUID: d8b3f5a1-2e74-4c69-8f30-1a7d9c4e6b25
 * Version: 1.2.0
 * Description: Implements the -b/--brains flag for `gsc pi`: displays session statistics including context tokens, model/provider, touched files, and active GitSense Brains. Groups files by operation (EDITED/READ/WRITTEN) and repository association (IN REPO/OUTSIDE REPO). Shows context usage from the current conversation path with explanation. Outputs plain text suitable for piping or embedding in other tools. If no session-id is provided, infers from the current directory.
 * Language: Go
 * Created-at: 2026-06-20T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0, v1.1.0)
 */


package pi

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
)

func runStats(w io.Writer, dbPath, sessionID, format string) error {
	resolvedDB, err := resolvePiSessionsDBPath(dbPath)
	if err != nil {
		return err
	}

	// If no session ID provided, find the most recent session for the current directory.
	if sessionID == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine current directory: %w", err)
		}
		sessionID, err = findSessionByCWD(resolvedDB, cwd)
		if err != nil {
			return err
		}
	}

	self, _ := os.Executable()

	show, err := pisessions.Show(context.Background(), resolvedDB, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	usage, _ := pisessions.LastUsage(context.Background(), resolvedDB, sessionID)
	branchUsage, _ := pisessions.CurrentBranchUsage(context.Background(), resolvedDB, sessionID)
	files, _ := pisessions.TouchedFiles(context.Background(), resolvedDB, sessionID)
	brains := fetchBrains(self, show.RepoRoot)

	switch format {
	case "json":
		return writeStatsJSON(w, show, usage, files, brains)
	default:
		return writeStatsHuman(w, show, usage, branchUsage, files, brains)
	}
}

func writeStatsHuman(w io.Writer, show *pisessions.ShowResult, usage *pisessions.SessionUsage, branchUsage *pisessions.SessionUsage, files []pisessions.TouchedFile, brains []string) error {
	// Explanation header
	fmt.Fprintf(w, "Context usage: total tokens in **current** conversation path (input + output + cached).\n")
	fmt.Fprintf(w, "This matches the context window limit, not the input-only count shown in Pi's footer.\n\n")

	// Context section
	if branchUsage == nil {
		fmt.Fprintf(w, "  (no response yet)\n")
	} else {
		ctx := branchUsage.ContextTokens()
		abbr := abbrevTokens(ctx)

		// Render big glyphs for context size
		for _, line := range renderGlyphLines(abbr) {
			fmt.Fprintf(w, "%s\n", line)
		}

		fmt.Fprintf(w, "%s tokens\n", abbr)
	}
	fmt.Fprintf(w, "\n")

	// Files section - grouped by operation
	fmt.Fprintf(w, "FILES TOUCHED (%d)\n", len(files))
	fmt.Fprintf(w, "───────────────\n")
	if len(files) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	} else {
		// Group files by operation
		written := []pisessions.TouchedFile{}
		edited := []pisessions.TouchedFile{}
		read := []pisessions.TouchedFile{}
		outside := []pisessions.TouchedFile{}

		for _, f := range files {
			// Files outside repo root have empty file_path_rel
			if f.FilePathRel == "" && f.RepoRoot == "" {
				outside = append(outside, f)
				continue
			}
			switch f.Op {
			case "write":
				written = append(written, f)
			case "edit":
				edited = append(edited, f)
			case "read":
				read = append(read, f)
			default:
				read = append(read, f)
			}
		}

		if len(written) > 0 {
			fmt.Fprintf(w, "  WRITTEN\n")
			var inRepo, outside []pisessions.TouchedFile
			for _, f := range written {
				if f.FilePathRel != "" {
					inRepo = append(inRepo, f)
				} else {
					outside = append(outside, f)
				}
			}
			if len(inRepo) > 0 {
				fmt.Fprintf(w, "    IN REPO (%d)\n", len(inRepo))
				for _, f := range inRepo {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
			if len(outside) > 0 {
				fmt.Fprintf(w, "    OUTSIDE REPO (%d)\n", len(outside))
				for _, f := range outside {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
		}
		if len(edited) > 0 {
			fmt.Fprintf(w, "  EDITED\n")
			var inRepo, outside []pisessions.TouchedFile
			for _, f := range edited {
				if f.FilePathRel != "" {
					inRepo = append(inRepo, f)
				} else {
					outside = append(outside, f)
				}
			}
			if len(inRepo) > 0 {
				fmt.Fprintf(w, "    IN REPO (%d)\n", len(inRepo))
				for _, f := range inRepo {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
			if len(outside) > 0 {
				fmt.Fprintf(w, "    OUTSIDE REPO (%d)\n", len(outside))
				for _, f := range outside {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
		}
		if len(read) > 0 {
			fmt.Fprintf(w, "  READ\n")
			var inRepo, outside []pisessions.TouchedFile
			for _, f := range read {
				if f.FilePathRel != "" {
					inRepo = append(inRepo, f)
				} else {
					outside = append(outside, f)
				}
			}
			if len(inRepo) > 0 {
				fmt.Fprintf(w, "    IN REPO (%d)\n", len(inRepo))
				for _, f := range inRepo {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
			if len(outside) > 0 {
				fmt.Fprintf(w, "    OUTSIDE REPO (%d)\n", len(outside))
				for _, f := range outside {
					fmt.Fprintf(w, "    ├─ %s\n", displayPath(f))
				}
			}
		}
		if len(outside) > 0 {
			fmt.Fprintf(w, "  OUTSIDE REPO (%d)\n", len(outside))
		}
	}
	fmt.Fprintf(w, "\n")

	// Brains section
	if len(brains) > 0 {
		fmt.Fprintf(w, "BRAINS\n")
		fmt.Fprintf(w, "──────\n")
		for _, name := range brains {
			fmt.Fprintf(w, "  ✓ %s\n", name)
		}
	}

	// Footer with session metadata
	fmt.Fprintf(w, "\n─────────────────────────────────────────\n")
	footer := fmt.Sprintf("Session: %s", show.SessionID[:8])
	if show.Model != "" {
		footer += fmt.Sprintf(" · %s", show.Model)
	}
	if show.Provider != "" {
		footer += fmt.Sprintf(" · %s", show.Provider)
	}
	if show.RepoRoot != "" {
		footer += fmt.Sprintf(" · %s", homeRelative(show.RepoRoot))
	}
	fmt.Fprintf(w, "%s\n", footer)

	return nil
}

// findSessionByCWD finds the most recently updated session for the given directory.
func findSessionByCWD(dbPath, cwd string) (string, error) {
	results, err := pisessions.List(context.Background(), pisessions.ListOptions{
		DBPath: dbPath,
		Sort:   "recent",
		Limit:  10,
	})
	if err != nil {
		return "", fmt.Errorf("query sessions: %w", err)
	}

	// Find the most recent session that matches the current directory.
	for _, r := range results {
		if r.CWD == cwd {
			return r.SessionID, nil
		}
	}

	return "", fmt.Errorf("no session found for current directory: %s\nRun `gsc pi -b <session-id>` with an explicit session ID", cwd)
}

func writeStatsJSON(w io.Writer, show *pisessions.ShowResult, usage *pisessions.SessionUsage, files []pisessions.TouchedFile, brains []string) error {
	fmt.Fprintf(w, `{
  "session_id": %q,
  "model": %q,
  "provider": %q,
  "repo_root": %q,
`, show.SessionID, show.Model, show.Provider, show.RepoRoot)

	if usage != nil {
		ctx := usage.ContextTokens()
		win := contextWindow(show.Model)
		pct := 0
		if win > 0 {
			pct = int(float64(ctx) / float64(win) * 100)
		}
		fmt.Fprintf(w, `  "context": {
    "tokens": %d,
    "window": %d,
    "percent": %d,
    "input": %d,
    "output": %d,
    "cache_read": %d,
    "cost": %.4f
  },
`, ctx, win, pct, usage.InputTokens, usage.OutputTokens, usage.CacheRead, usage.CostTotal)
	} else {
		fmt.Fprintf(w, `  "context": null,
`)
	}

	fmt.Fprintf(w, `  "files": %d,
`, len(files))
	fmt.Fprintf(w, `  "brains": [`)
	for i, name := range brains {
		if i > 0 {
			fmt.Fprintf(w, `, `)
		}
		fmt.Fprintf(w, `%q`, name)
	}
	fmt.Fprintf(w, `]
}
`)

	return nil
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}

// displayPath returns a display-friendly path with ~ for home directory.
func displayPath(file pisessions.TouchedFile) string {
	// Use file_path_rel if it's not empty and doesn't start with ~ (inside repo)
	if file.FilePathRel != "" && !strings.HasPrefix(file.FilePathRel, "~") {
		if file.RepoRoot != "" {
			return homeRelative(file.RepoRoot) + "/" + file.FilePathRel
		}
		return file.FilePathRel
	}
	// For paths with ~ or empty file_path_rel, use abs_path
	if file.AbsPath != "" {
		return homeRelative(file.AbsPath)
	}
	// Fallback to file_path_rel with tilde expansion
	if file.FilePathRel != "" {
		return homeRelative(expandTilde(file.FilePathRel))
	}
	return ""
}
