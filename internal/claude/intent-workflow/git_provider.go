/**
 * Component: Git Provider
 * Block-UUID: 3a8f2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c
 * Parent-UUID: 9091a2a2-9125-4a6d-934e-3a2ae3491441
 * Version: 1.3.0
 * Description: Encapsulates git-based provenance extraction using os/exec with git plumbing commands. Provides FileProvenance data (blob SHAs, change type, line counts) for all files changed in the working tree across one or more working directories. Added GetGitContext() method to capture branch head information (branch name, HEAD SHA, commit message, author, timestamp, detached HEAD state, dirty state, remote URL) for comprehensive audit trail.
 * Language: Go
 * Created-at: 2026-04-26T14:21:54.256Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package intent_workflow

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FileProvenance holds git-derived metadata for a single changed file.
type FileProvenance struct {
	OldBlobSHA   string
	NewBlobSHA   string
	ChangeType   string // "modified", "added", "deleted", "renamed"
	LinesAdded   int
	LinesDeleted int
}

// GitProvider extracts change provenance from one or more working directories.
type GitProvider struct {
	workdirs []WorkingDirectory
}

// NewGitProvider creates a GitProvider for the given working directories.
func NewGitProvider(workdirs []WorkingDirectory) *GitProvider {
	return &GitProvider{workdirs: workdirs}
}

// GetChanges returns FileProvenance for every changed file across all working
// directories. The map key is the absolute path to the changed file.
func (g *GitProvider) GetChanges() (map[string]FileProvenance, error) {
	result := make(map[string]FileProvenance)

	for _, wd := range g.workdirs {
		rawOut, err := runGitCmd(wd.Path, "diff", "--raw", "--abbrev=40")
		if err != nil {
			return nil, fmt.Errorf("git diff --raw in %s: %w", wd.Path, err)
		}

		numstatOut, err := runGitCmd(wd.Path, "diff", "--numstat")
		if err != nil {
			return nil, fmt.Errorf("git diff --numstat in %s: %w", wd.Path, err)
		}

		lineCounts := parseNumstat(string(numstatOut))

		for _, line := range strings.Split(string(rawOut), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, ":") {
				continue
			}

			// Format: ":old_mode new_mode old_sha new_sha STATUS\tpath"
			// Renames/copies:  "…STATUS\told_path\tnew_path"
			tabParts := strings.SplitN(line, "\t", 2)
			if len(tabParts) < 2 {
				continue
			}

			metaParts := strings.Fields(tabParts[0])
			if len(metaParts) < 5 {
				continue
			}

			oldSHA := normalizeZeroSHA(metaParts[2])
			newSHA := normalizeZeroSHA(metaParts[3])
			status := metaParts[4]

			var relPath string
			if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
				// Use the new (destination) path for renames and copies.
				pathParts := strings.SplitN(tabParts[1], "\t", 2)
				if len(pathParts) == 2 {
					relPath = pathParts[1]
				} else {
					relPath = pathParts[0]
				}
				status = string(status[0])
			} else {
				relPath = tabParts[1]
			}

			absPath := filepath.Join(wd.Path, relPath)
			prov := FileProvenance{
				OldBlobSHA: oldSHA,
				NewBlobSHA: newSHA,
				ChangeType: rawStatusToChangeType(status),
			}
			if lc, ok := lineCounts[relPath]; ok {
				prov.LinesAdded = lc.added
				prov.LinesDeleted = lc.deleted
			}
			// git diff --raw reports all-zeros for the new blob when the file
			// is modified in the working tree but not yet staged. Use
			// git hash-object to derive the actual blob SHA in that case.
			if prov.NewBlobSHA == "" && prov.ChangeType != "deleted" {
				if sha, err := computeBlobSHA(wd.Path, absPath); err == nil {
					prov.NewBlobSHA = sha
				}
			}

			result[absPath] = prov
		}
	}

	return result, nil
}

// GetGitContext captures git repository state for a specific working directory.
// Returns branch name, HEAD commit info, detached HEAD state, dirty state, and remote URL.
func (g *GitProvider) GetGitContext(workingDir string) (*GitContext, error) {
	ctx := &GitContext{}

	// Get branch name (returns "HEAD" if detached)
	branchName, err := runGitCmd(workingDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// If this fails, we might not be in a git repo or have no commits
		ctx.BranchName = "unknown"
	} else {
		ctx.BranchName = strings.TrimSpace(string(branchName))
	}

	// Get HEAD SHA
	headSHA, err := runGitCmd(workingDir, "rev-parse", "HEAD")
	if err != nil {
		// No commits yet
		ctx.HeadSHA = ""
	} else {
		ctx.HeadSHA = strings.TrimSpace(string(headSHA))
	}

	// Get commit message (first line)
	if ctx.HeadSHA != "" {
		headMessage, err := runGitCmd(workingDir, "log", "-1", "--pretty=%s", "HEAD")
		if err == nil {
			ctx.HeadMessage = strings.TrimSpace(string(headMessage))
		}

		// Get author
		headAuthor, err := runGitCmd(workingDir, "log", "-1", "--pretty=%an", "HEAD")
		if err == nil {
			ctx.HeadAuthor = strings.TrimSpace(string(headAuthor))
		}

		// Get timestamp
		headTimestamp, err := runGitCmd(workingDir, "log", "-1", "--pretty=%cI", "HEAD")
		if err == nil {
			ctx.HeadTimestamp = strings.TrimSpace(string(headTimestamp))
		}
	}

	// Check if detached HEAD (exit code 0 = attached, 1 = detached)
	_, err = runGitCmd(workingDir, "symbolic-ref", "-q", "HEAD")
	ctx.IsDetached = (err != nil)

	// Check if working directory is dirty (exit code 0 = clean, 1 = dirty)
	_, err = runGitCmd(workingDir, "diff", "--quiet")
	ctx.IsDirty = (err != nil)

	// Get remote URL (optional, no error if missing)
	remoteURL, err := runGitCmd(workingDir, "remote", "get-url", "origin")
	if err == nil {
		ctx.RemoteURL = strings.TrimSpace(string(remoteURL))
	}

	return ctx, nil
}

func runGitCmd(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// computeBlobSHA returns the git blob SHA for the current content of absPath
// without requiring the file to be staged first.
func computeBlobSHA(workdir, absPath string) (string, error) {
	out, err := runGitCmd(workdir, "hash-object", absPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseNumstat(output string) map[string]struct{ added, deleted int } {
	m := make(map[string]struct{ added, deleted int })
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		path := strings.Join(parts[2:], " ")
		m[path] = struct{ added, deleted int }{added, deleted}
	}
	return m
}

func normalizeZeroSHA(sha string) string {
	if strings.TrimLeft(sha, "0") == "" {
		return ""
	}
	return sha
}

func rawStatusToChangeType(status string) string {
	switch status {
	case "A":
		return "added"
	case "D":
		return "deleted"
	case "R", "C":
		return "renamed"
	default:
		return "modified"
	}
}
