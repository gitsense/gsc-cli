/**
 * Component: Experts Config
 * Block-UUID: c3ecb088-5de2-481e-a11b-ee8d66307ad7
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Configuration and path resolution logic for the gsc experts command. Defines the location of the experts context file and helpers for finding the git repository root.
 * Language: Go
 * Created-at: 2026-05-01T16:12:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package experts

import (
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
)

// ContextFileName is the name of the file where the expert context is stored.
const ContextFileName = "experts-context.md"

// ContextFilePath returns the full path to the experts context file for the given repository root.
func ContextFilePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".gitsense", ContextFileName)
}

// FindGitRoot walks up the directory tree from the current working directory to find the .git folder.
// It returns the absolute path to the repository root.
func FindGitRoot() (string, error) {
	return git.FindProjectRoot()
}
