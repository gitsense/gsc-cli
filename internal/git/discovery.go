/*
 * Component: Git Discovery
 * Block-UUID: 9eb3ffa7-cdd9-4fb4-b943-d9df0773f3cb
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides functionality to discover the project root by locating the .gitsense directory.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
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
func findRootFromPath(startPath string) (string, error) {
	path := startPath
	for {
		// Check if .gitsense exists in the current directory
		gitsensePath := filepath.Join(path, GitSenseDirName)
		if _, err := os.Stat(gitsensePath); err == nil {
			// Found it
			return path, nil
		}

		// Move to parent directory
		parent := filepath.Dir(path)
		if parent == path {
			// Reached the root of the filesystem without finding .gitsense
			return "", os.ErrNotExist
		}
		path = parent
	}
}
