/**
 * Component: Git Discovery
 * Block-UUID: 6b39f1a4-918d-47c4-bfdd-0d36cce6aa53
 * Parent-UUID: 5dec3486-fe35-43e9-9e5d-ee673382f1dd
 * Version: 1.5.0
 * Description: Provides functionality to discover the project root by locating the .git directory. Fixed to prioritize .git over .gitsense to prevent home directory collisions. Removed unused variable to fix compilation error.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
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
// until it finds a directory containing the .git folder.
// It returns the absolute path to the project root or an error if not found.
// This function now prioritizes finding the .git directory to avoid collisions
// with a global .gitsense directory in the user's home folder.
func FindProjectRoot() (string, error) {
	return FindGitRoot()
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
