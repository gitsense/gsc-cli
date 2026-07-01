/**
 * Component: Pi Session Path Normalization
 * Block-UUID: 7c3e1a05-9b42-4d68-8f1a-2e6d0b94c537
 * Parent-UUID: 0790b593-8531-4a12-9054-cad3d3072667
 * Version: 1.2.0
 * Description: Normalizes Pi file reference paths lexically for repo-relative and absolute recall; expands tilde (~) to home directory before normalization; adds isEphemeralCWD to detect Pi's throwaway pi-runtime-cwd working directories so the importer can skip them.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v1.1.0)
 */


package sessions

import (
	"os"
	"path/filepath"
	"strings"
)

type normalizedPath struct {
	rawPath     string
	absPath     string
	repoRoot    string
	filePathRel string
	cwdRelPath  string
}

func normalizePath(rawPath string, cwd string, repoRoot string) normalizedPath {
	// Expand tilde to home directory
	path := rawPath
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}

	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, path)
	}
	absPath = filepath.Clean(absPath)

	cwdRelPath := ""
	if cwd != "" {
		if rel, err := filepath.Rel(filepath.Clean(cwd), absPath); err == nil {
			cwdRelPath = filepath.ToSlash(rel)
		}
	}

	filePathRel := ""
	if repoRoot != "" {
		cleanRoot := filepath.Clean(repoRoot)
		if rel, err := filepath.Rel(cleanRoot, absPath); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			filePathRel = filepath.ToSlash(rel)
		}
	}

	return normalizedPath{
		rawPath:     rawPath,
		absPath:     absPath,
		repoRoot:    repoRoot,
		filePathRel: filePathRel,
		cwdRelPath:  cwdRelPath,
	}
}

// isEphemeralCWD reports whether a session's working directory is ephemeral and
// should be excluded from the mirror. The inclusion rule is positive: keep a
// session only when its cwd is a persistent (non-temp) directory. A cwd is
// treated as ephemeral when it is under the OS temp dir, or contains a
// "/pi-runtime-" path segment — Pi's throwaway scratch dirs for RPC/subagent and
// test-suite runs (pi-runtime-cwd-*, pi-runtime-suite-*, …). The marker is kept
// alongside the temp-dir check so detection still works if Pi and the importer
// resolve a different temp root.
func isEphemeralCWD(cwd string) bool {
	if cwd == "" {
		return false
	}
	clean := filepath.Clean(cwd)
	if strings.Contains(filepath.ToSlash(clean), "/pi-runtime-") {
		return true
	}
	tmp := os.TempDir()
	for _, base := range []string{tmp, resolveSymlink(tmp)} {
		base = filepath.Clean(base)
		if base == "" || base == "." {
			continue
		}
		if rel, err := filepath.Rel(base, clean); err == nil &&
			rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func resolveSymlink(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func findRepoRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	current := filepath.Clean(cwd)
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
