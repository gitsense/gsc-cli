/**
 * Component: Git Discovery
 * Block-UUID: 2f939aab-f305-4d58-93a1-e1a247b13b65
 * Parent-UUID: f5137746-965d-4909-9d6c-c2d69a4f4d25
 * Version: 1.3.0
 * Description: Provides functionality to discover the project root by locating the .git directory first, then verifying .gitsense exists or can be created. Fixed to support manifest init in Git repos without existing .gitsense.
 * Language: Go
 * Created-at: 2026-02-02T07:11:41.130Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0)
 */


package git

import (
	"os"
	"path/filepath"
)

const (
	// GitSenseDirName is the name of the directory that marks the project root.
	GitSenseDirName = ".gitsense"
)

// FindProjectRoot walks up the directory tree from the current working directory
// until it finds a directory containing the .gitsense folder.
// It returns the absolute path to the project root or an error if not found.
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return findRootFromPath(cwd)
}

// findRootFromPath is a helper that starts the search from a specific path.
// It searches for a directory containing .gitsense. If .gitsense doesn't exist,
// it falls back to finding the Git root (.git directory).
// This allows manifest init to work in a Git repo without an existing .gitsense directory.
func findRootFromPath(startPath string) (string, error) {
	path := startPath
	for {
		// Check if .gitsense exists in the current directory
		gitsensePath := filepath.Join(path, GitSenseDirName)
		if _, err := os.Stat(gitsensePath); err == nil {
			// Found .gitsense, return this as the project root
			return path, nil
		}

		// Move to parent directory
		parent := filepath.Dir(path)
		if parent == path {
			// Reached the root of the filesystem without finding .gitsense
			// Fall back to finding the Git root instead
			return FindGitRoot()
		}
		path = parent
	}
}

// FindGitRoot walks up the directory tree from the current working directory
// until it finds a directory containing the .git folder.
// It returns the absolute path to the git repository root or an error if not found.
func FindGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	path := cwd
	for {
		// Check if .git exists in the current directory
		gitPath := filepath.Join(path, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			// Found it
			return path, nil
		}

		// Move to parent directory
		parent := filepath.Dir(path)
		if parent == path {
			// Reached the root of the filesystem without finding .git
			return "", os.ErrNotExist
		}
		path = parent
	}
}
