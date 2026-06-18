/**
 * Component: Pi Session Path Normalization
 * Block-UUID: 0790b593-8531-4a12-9054-cad3d3072667
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Normalizes Pi file reference paths lexically for repo-relative and absolute recall.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
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
	absPath := rawPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, rawPath)
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
