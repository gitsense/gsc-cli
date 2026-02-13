/**
 * Component: Git Discovery
 * Block-UUID: d564e339-ef25-4f2d-894f-611175df95ad
 * Parent-UUID: d6a7296e-f17a-44fa-9aee-5b796e2f64a5
 * Version: 1.7.0
 * Description: Provides functionality to discover the project root by locating the .git directory. Added GetTrackedFiles to execute 'git ls-files' for scope validation and coverage analysis.
 * Language: Go
 * Created-at: 2026-02-06T04:05:10.555Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0)
 */


package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/logger"
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

// GetRepoContext returns the canonical (physical) absolute path to the repository root
// and the relative path from that root to the current working directory.
// This handles symlinks by evaluating them before calculating the offset.
func GetRepoContext() (root string, cwdOffset string, err error) {
	rawRoot, err := FindGitRoot()
	if err != nil {
		return "", "", err
	}

	// Evaluate symlinks for the root
	root, err = filepath.EvalSymlinks(rawRoot)
	if err != nil {
		return "", "", err
	}

	// Get and evaluate symlinks for the CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	absCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", "", err
	}

	// Calculate the relative offset from root to CWD
	cwdOffset, err = filepath.Rel(root, absCwd)
	return root, cwdOffset, err
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

// GetTrackedFiles executes 'git ls-files' to retrieve all tracked files in the repository.
// It respects .gitignore rules inherently.
// It returns a slice of file paths relative to the repository root.
func GetTrackedFiles(ctx context.Context, repoRoot string) ([]string, error) {
	// Prepare the command
	// -z uses null bytes as terminators, handling filenames with spaces/newlines correctly
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z")
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // Capture stderr for error logging

	// Execute
	err := cmd.Run()
	if err != nil {
		logger.Error("Failed to execute git ls-files", "error", err, "output", out.String())
		return nil, err
	}

	// Parse output
	// Split by null byte (0x00)
	output := out.String()
	if output == "" {
		return []string{}, nil
	}

	files := strings.Split(output, "\x00")
	
	// The last element might be an empty string if the output ends with a null byte
	if len(files) > 0 && files[len(files)-1] == "" {
		files = files[:len(files)-1]
	}

	logger.Debug("Retrieved tracked files", "count", len(files), "repo", repoRoot)
	return files, nil
}
