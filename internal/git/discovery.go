/**
 * Component: Git Discovery
 * Block-UUID: 87c64cfc-b3f5-4133-9324-bb21e4630013
 * Parent-UUID: c99ac801-d417-4bf4-a869-9ecc819f785e
 * Version: 1.11.0
 * Description: Provides functionality to discover the project root by locating the .git directory. Added GetTrackedFiles to execute 'git ls-files' for scope validation and coverage analysis. Added GetCommitCount and GetFileCount for Phase 2 threshold detection. v1.11.0: Added HasUncommittedChanges to detect uncommitted changes using git status --porcelain for fast checking.
 * Language: Go
 * Created-at: 2026-05-14T15:11:34.862Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0)
 */


package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// FindGitRootFrom starts at the given path and walks up to find the Git root.
// It returns the absolute path to the git repository root or an error if not found.
func FindGitRootFrom(startPath string) (string, error) {
	path, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

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

// GetCommitCount returns the total number of commits on HEAD.
// Runs: git rev-list --count HEAD
func GetCommitCount(repoRoot string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		logger.Error("Failed to get commit count", "error", err, "output", out.String())
		return 0, err
	}

	count, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetFileCount returns the number of git-tracked files.
// Reuses GetTrackedFiles and counts the result to avoid spawning external processes like wc -l.
func GetFileCount(repoRoot string) (int, error) {
	ctx := context.Background()
	files, err := GetTrackedFiles(ctx, repoRoot)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// GetCommitHash returns the current HEAD commit hash.
// Runs: git rev-parse HEAD
func GetCommitHash(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// HasUncommittedChanges checks if the repository has uncommitted changes
// (both staged and unstaged) using git status --porcelain.
// Returns true if there are any uncommitted changes, false otherwise.
func HasUncommittedChanges(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// If output is empty, there are no uncommitted changes
	return out.Len() > 0, nil
}
