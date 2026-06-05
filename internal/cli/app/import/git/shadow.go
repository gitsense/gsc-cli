/**
 * Component: Shadow Repo Manager
 * Block-UUID: d0f55e85-b9e0-4c10-8b39-68faeaee4133
 * Parent-UUID: a6f35621-fb85-43c4-b18f-ee7b9cf5c244
 * Version: 2.8.0
 * Description: Updated ShadowPath and ListShadows functions to use the new ShadowReposRelPath constant from settings package, moving shadow repositories from $GSC_HOME/shadow-repos/ to $GSC_HOME/data/shadow-repos/ for better organization and consistency with other data storage.
 * Language: Go
 * Created-at: 2026-05-28T17:26:22.007Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.4.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.0.2), GLM-4.7 (v2.0.3), Gemini 2.5 Flash Lite (v2.1.0), GLM-4.7 (v2.2.0), Gemini 2.5 Flash Lite (v2.3.0), GLM-4.7 (v2.4.0), Gemini 2.5 Flash Lite (v2.5.0), GLM-4.7 (v2.6.0), DeepSeek V4 Pro (v2.6.1), GLM-4.7 (v2.7.0), GLM-4.7 (v2.8.0)
 */


package importgit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ShadowPhase represents the current stage of the shadow operation
type ShadowPhase int

const (
	PhaseScanning  ShadowPhase = iota
	PhaseCopying
	PhaseStaging
	PhaseCommitting
	PhaseDone
)

// ErrNoChanges is returned by UpdateShadow when the source has not changed since the last snapshot.
var ErrNoChanges = errors.New("shadow repo: no changes detected since last snapshot")

// ShadowProgressFn is a callback function to report progress during shadow operations
type ShadowProgressFn func(phase ShadowPhase, current string, copied, total int)

// ShadowInfo holds metadata about a shadow repository
type ShadowInfo struct {
	Owner       string
	Repo        string
	Branch      string
	Path        string
	SizeBytes   int64
	LastUpdated time.Time
}

// branchToFS converts a git branch name to a filesystem-safe name by replacing '/' with '::'
// This allows slash-based branch names (e.g., feature/fix/foo) to be stored as flat directories
func branchToFS(branch string) string {
	return strings.ReplaceAll(branch, "/", "::")
}

// branchFromFS converts a filesystem-safe name back to a git branch name by replacing '::' with '/'
func branchFromFS(name string) string {
	return strings.ReplaceAll(name, "::", "/")
}

// ShadowPath returns the deterministic path for a shadow repository
// Uses branchToFS to flatten slash-based branch names for filesystem storage
func ShadowPath(gscHome, owner, repo, branch string) string {
	return filepath.Join(gscHome, settings.ShadowReposRelPath, owner, repo, branchToFS(branch))
}

// ShadowExists checks if a shadow repository exists at the given path
func ShadowExists(shadowPath string) bool {
	info, err := os.Stat(filepath.Join(shadowPath, ".git"))
	return err == nil && info.IsDir()
}

// CreateShadow initializes a new shadow repository from the source
// The branch parameter is used to rename the default branch after git init
func CreateShadow(shadowPath, sourcePath, branch string, progressFn ShadowProgressFn) error {
	if ShadowExists(shadowPath) {
		return fmt.Errorf("shadow repository already exists at %s", shadowPath)
	}

	startTime := time.Now()

	// 1. Create directory
	if err := os.MkdirAll(shadowPath, 0755); err != nil {
		return fmt.Errorf("failed to create shadow directory: %w", err)
	}

	// 2. Get tracked files
	ctx := context.Background()
	
	// FIX: Start PhaseScanning BEFORE git.GetTrackedFiles for accurate timing
	if progressFn != nil {
		progressFn(PhaseScanning, "", 0, 0)
	}
	
	files, err := git.GetTrackedFiles(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get tracked files: %w", err)
	}

	// 3. Copy files with APFS optimization on macOS
	// Note: git init and branch -m are now handled inside tryAPFSClone for macOS
	// or will be done by fastCopy fallback for non-APFS systems
	copyStartTime := time.Now()
	if err := fastCopy(files, sourcePath, shadowPath, branch, true, progressFn); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}
	copyDuration := time.Since(copyStartTime)

	// 4. Stage files using fast git update-index
	stageStartTime := time.Now()
	if err := fastStageFiles(shadowPath, files, progressFn); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}
	stageDuration := time.Since(stageStartTime)

	// 5. Commit with timing
	commitStartTime := time.Now()
	if err := commitShadow(shadowPath, sourcePath, progressFn); err != nil {
		return err
	}
	commitDuration := time.Since(commitStartTime)

	totalDuration := time.Since(startTime)

	if progressFn != nil {
		// Report timing metrics
		logger.Debug("Shadow creation timing", 
			"copy", copyDuration, 
			"stage", stageDuration, 
			"commit", commitDuration,
			"total", totalDuration)
		progressFn(PhaseDone, "", len(files), len(files))
	}

	return nil
}

// UpdateShadow refreshes an existing shadow repository
// This ensures the shadow always has exactly 2 commits (initial + update)
// which is required for gscb-cli's incremental diff detection
func UpdateShadow(shadowPath, sourcePath string, progressFn ShadowProgressFn) error {
	if !ShadowExists(shadowPath) {
		return fmt.Errorf("shadow repository does not exist at %s", shadowPath)
	}

	startTime := time.Now()

	// 1. Bulk clean (remove everything except .git)
	if err := bulkClean(shadowPath); err != nil {
		return fmt.Errorf("failed to clean shadow repo: %w", err)
	}

	// 2. Get tracked files
	ctx := context.Background()
	
	// FIX: Start PhaseScanning BEFORE git.GetTrackedFiles for accurate timing
	if progressFn != nil {
		progressFn(PhaseScanning, "", 0, 0)
	}
	
	files, err := git.GetTrackedFiles(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get tracked files: %w", err)
	}

	// 3. Copy files - updates always use parallelCopy to preserve existing .git history
	copyStartTime := time.Now()
	if err := fastCopy(files, sourcePath, shadowPath, "", false, progressFn); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}
	copyDuration := time.Since(copyStartTime)

	// 4. Stage files using fast git update-index
	stageStartTime := time.Now()
	if err := fastStageFiles(shadowPath, files, progressFn); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}
	stageDuration := time.Since(stageStartTime)

	// git diff --cached --exit-code exits 0 when index matches HEAD (nothing to commit)
	noop := exec.Command("git", "diff", "--cached", "--exit-code")
	noop.Dir = shadowPath
	if err := noop.Run(); err == nil {
		return ErrNoChanges
	}

	// 5. Commit with timing
	commitStartTime := time.Now()
	if err := commitShadow(shadowPath, sourcePath, progressFn); err != nil {
		return err
	}
	commitDuration := time.Since(commitStartTime)

	totalDuration := time.Since(startTime)

	if progressFn != nil {
		// Report timing metrics
		logger.Debug("Shadow update timing", 
			"copy", copyDuration, 
			"stage", stageDuration, 
			"commit", commitDuration,
			"total", totalDuration)
		progressFn(PhaseDone, "", len(files), len(files))
	}

	return nil
}

// DeleteShadow removes the shadow repository entirely
func DeleteShadow(shadowPath string) error {
	if err := os.RemoveAll(shadowPath); err != nil {
		return fmt.Errorf("failed to delete shadow repo: %w", err)
	}
	
	// Try to clean up empty parent directories
	// Stop before deleting the shadow-repos root directory
	parent := filepath.Dir(shadowPath)
	shadowReposRoot := filepath.Dir(filepath.Dir(parent))
	
	for parent != shadowReposRoot {
		if err := os.Remove(parent); err != nil {
			break // Stop if directory is not empty
		}
		parent = filepath.Dir(parent)
	}
	
	return nil
}

// ListShadows returns all shadow repositories under the GSC_HOME
func ListShadows(gscHome string) ([]ShadowInfo, error) {
	shadowRoot := filepath.Join(gscHome, settings.ShadowReposRelPath)
	var shadows []ShadowInfo

	owners, err := os.ReadDir(shadowRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []ShadowInfo{}, nil
		}
		return nil, err
	}

	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		ownerPath := filepath.Join(shadowRoot, ownerEntry.Name())

		repos, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}

		for _, repoEntry := range repos {
			if !repoEntry.IsDir() {
				continue
			}
			repoPath := filepath.Join(ownerPath, repoEntry.Name())

			branches, err := os.ReadDir(repoPath)
			if err != nil {
				continue
			}

			for _, branchEntry := range branches {
				if !branchEntry.IsDir() {
					continue
				}
				branchPath := filepath.Join(repoPath, branchEntry.Name())

				// Verify it's a valid git repo
				if !ShadowExists(branchPath) {
					continue
				}

				size, _ := ShadowSize(branchPath)
				info, _ := os.Stat(branchPath)

				// Convert filesystem-safe branch name back to git branch name
				branchName := branchFromFS(branchEntry.Name())

				shadows = append(shadows, ShadowInfo{
					Owner:       ownerEntry.Name(),
					Repo:        repoEntry.Name(),
					Branch:      branchName,
					Path:        branchPath,
					SizeBytes:   size,
					LastUpdated: info.ModTime(),
				})
			}
		}
	}

	return shadows, nil
}

// ShadowSize calculates the total size of the shadow repository
func ShadowSize(shadowPath string) (int64, error) {
	var size int64
	err := filepath.Walk(shadowPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// commitShadow stages all changes and creates a commit mirroring the source metadata
func commitShadow(shadowPath, sourcePath string, progressFn ShadowProgressFn) error {
	if progressFn != nil {
		progressFn(PhaseCommitting, "", 0, 0)
	}

	// Get source commit metadata
	message, author, _, err := getSourceCommitMeta(sourcePath)
	if err != nil {
		logger.Warning("Failed to get source commit metadata, using defaults", "error", err)
		message = "Shadow snapshot"
		author = "GitSense Shadow Bot <bot@gitsense.io>"
	}

	// Commit with shadow bot as committer
	args := []string{"commit", "-m", message}
	if author != "" {
		args = append(args, "--author", author)
	}
	
	// Set committer identity to shadow bot
	cmd := exec.Command("git", args...)
	cmd.Dir = shadowPath
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=GitSense Shadow Bot",
		"GIT_COMMITTER_EMAIL=bot@gitsense.io",
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit shadow snapshot: %s", string(output))
	}

	return nil
}

// bulkClean removes all files and directories in the shadow path except .git
func bulkClean(shadowPath string) error {
	entries, err := os.ReadDir(shadowPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" {
			continue
		}
		fullPath := filepath.Join(shadowPath, name)
		if err := os.RemoveAll(fullPath); err != nil {
			return err
		}
	}
	return nil
}

// fastCopy copies files using the fastest available method for the platform.
// On macOS with APFS and isNew=true, uses clonefile for instant copy-on-write.
// Updates always use parallelCopy to preserve the existing .git history.
func fastCopy(files []string, srcRoot, dstRoot string, branch string, isNew bool, progressFn ShadowProgressFn) error {
	// APFS clonefile only for new shadows - updates must preserve .git
	if runtime.GOOS == "darwin" && isNew {
		var err error
		if err = tryAPFSClone(files, srcRoot, dstRoot, branch, progressFn); err == nil {
			logger.Debug("Using APFS clonefile for instant file copying")
			return nil
		} else {
			logger.Debug("APFS clonefile failed, falling back to parallel copy", "error", err)
		}
		// v2.7.0: APFS path failed before git init could run - initialize git now so
		// parallelCopy has a valid shadow repo to stage into.
		if err := gitExec(dstRoot, "init"); err != nil {
			return fmt.Errorf("failed to init shadow git repo (fallback): %w", err)
		}
		if branch != "" {
			if err := gitExec(dstRoot, "branch", "-m", branch); err != nil {
				return fmt.Errorf("failed to rename shadow branch to %s (fallback): %w", branch, err)
			}
		}
	}

	// Fall back to parallel copy
	return parallelCopy(files, srcRoot, dstRoot, progressFn)
}

// tryAPFSClone attempts to use macOS APFS clonefile for instant copying
// v2.7.0: Fixed to check os.Lstat BEFORE cp -c to prevent failures on directory symlinks.
// All symlinks (file or directory) are now recreated natively using os.Readlink/os.Symlink,
// while cp -c is only used for regular files.
func tryAPFSClone(files []string, srcRoot, dstRoot string, branch string, progressFn ShadowProgressFn) error {
	if progressFn != nil {
		progressFn(PhaseCopying, "", 0, len(files))
	}
	if len(files) == 0 {
		return fmt.Errorf("no tracked files to clone")
	}
	probeSrc := filepath.Join(srcRoot, files[0])
	probeDst := filepath.Join(dstRoot, ".gsc-apfs-probe")
	if err := os.MkdirAll(filepath.Dir(probeDst), 0755); err != nil {
		return fmt.Errorf("probe mkdir failed: %w", err)
	}
	out, err := exec.Command("cp", "-c", probeSrc, probeDst).CombinedOutput()
	os.Remove(probeDst)
	if err != nil {
		return fmt.Errorf("clonefile not supported: %s", string(out))
	}
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	jobs := make(chan string, len(files))
	results := make(chan error, numWorkers)
	var copiedCount int64
	for i := 0; i < numWorkers; i++ {
		go func() {
			for file := range jobs {
				src := filepath.Join(srcRoot, file)
				dst := filepath.Join(dstRoot, file)
				if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
					results <- fmt.Errorf("mkdir failed for %s: %w", file, err)
					return
				}
				
				// v2.7.0: Check if source is a symlink BEFORE copying
				srcInfo, err := os.Lstat(src)
				if err != nil {
					results <- fmt.Errorf("failed to stat source %s: %w", file, err)
					return
				}
				
				if srcInfo.Mode()&os.ModeSymlink != 0 {
					// All symlinks (to files OR directories): recreate natively, never use cp
					target, err := os.Readlink(src)
					if err != nil {
						results <- fmt.Errorf("failed to read symlink %s: %w", file, err)
						return
					}
					if err := os.Symlink(target, dst); err != nil {
						results <- fmt.Errorf("failed to create symlink %s: %w", file, err)
						return
					}
				} else {
					// Regular file: use cp -c for APFS copy-on-write benefit
					out, err := exec.Command("cp", "-c", src, dst).CombinedOutput()
					if err != nil {
						results <- fmt.Errorf("cp -c failed for %s: %s", file, string(out))
						return
					}
				}
				
				count := atomic.AddInt64(&copiedCount, 1)
				if progressFn != nil {
					progressFn(PhaseCopying, file, int(count), len(files))
				}
			}
			results <- nil
		}()
	}
	for _, file := range files {
		jobs <- file
	}
	close(jobs)
	for i := 0; i < numWorkers; i++ {
		if err := <-results; err != nil {
			return err
		}
	}
	if err := gitExec(dstRoot, "init"); err != nil {
		return fmt.Errorf("failed to init shadow git repo: %w", err)
	}
	if branch != "" {
		if err := gitExec(dstRoot, "branch", "-m", branch); err != nil {
			return fmt.Errorf("failed to rename shadow branch to %s: %w", branch, err)
		}
	}
	return nil
}

// fastStageFiles uses git update-index --stdin for fast staging
// This is significantly faster than git add -A for large file sets
func fastStageFiles(shadowPath string, files []string, progressFn ShadowProgressFn) error {
	if progressFn != nil {
		progressFn(PhaseStaging, "", 0, len(files))
	}

	// Build null-terminated file list for stdin
	var fileList strings.Builder
	for _, file := range files {
		fileList.WriteString(file)
		fileList.WriteString("\x00")
	}

	// Use git update-index --add --remove -z --stdin
	// This directly updates the index without walking the working tree
	cmd := exec.Command("git", "update-index", "--add", "--remove", "-z", "--stdin")
	cmd.Dir = shadowPath
	cmd.Stdin = strings.NewReader(fileList.String())
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git update-index failed: %s", string(output))
	}

	return nil
}

// parallelCopy copies files using a worker pool (fallback for non-APFS systems)
func parallelCopy(files []string, srcRoot, dstRoot string, progressFn ShadowProgressFn) error {
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}

	jobs := make(chan string, len(files))
	results := make(chan error, len(files))
	
	var copiedCount int64

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go func() {
			for file := range jobs {
				logger.Debug("Worker copying file", "file", file)
				if err := copyFile(filepath.Join(srcRoot, file), filepath.Join(dstRoot, file)); err != nil {
					results <- err
					return
				}
				
				count := atomic.AddInt64(&copiedCount, 1)
				if progressFn != nil {
					progressFn(PhaseCopying, file, int(count), len(files))
				}
			}
			results <- nil
		}()
	}

	// Dispatch jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Collect results
	for i := 0; i < numWorkers; i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a single file or symlink from src to dst
func copyFile(src, dst string) error {
	// Check if source is a symlink
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warning("File listed by git but missing on disk, skipping", "path", src)
			return nil
		}
		return err
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		// Debug: check if destination already exists
		if _, err := os.Lstat(dst); err == nil {
			logger.Warning("Symlink destination already exists, overwriting", "dst", dst)
			if err := os.Remove(dst); err != nil {
				return fmt.Errorf("failed to remove existing file %s: %w", dst, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to lstat destination %s: %w", dst, err)
		}
		return os.Symlink(target, dst)
	}

	// Handle regular files
	// Exclude .gitsense directory
	if strings.Contains(src, ".gitsense") {
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// getSourceCommitMeta retrieves the latest commit message, author, and relative time from the source repo
func getSourceCommitMeta(repoPath string) (message, author, relativeTime string, err error) {
	// Get commit message
	msgBytes, err := exec.Command("git", "-C", repoPath, "log", "-1", "--pretty=%B").Output()
	if err != nil {
		return "", "", "", err
	}
	message = strings.TrimSpace(string(msgBytes))

	// Get author
	authBytes, err := exec.Command("git", "-C", repoPath, "log", "-1", "--pretty=%an <%ae>").Output()
	if err != nil {
		return message, "", "", err
	}
	author = strings.TrimSpace(string(authBytes))

	// Get timestamp
	tsBytes, err := exec.Command("git", "-C", repoPath, "log", "-1", "--pretty=%ct").Output()
	if err != nil {
		return message, author, "", err
	}
	tsStr := strings.TrimSpace(string(tsBytes))
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return message, author, "", err
	}

	relativeTime = formatRelativeTime(ts)

	return message, author, relativeTime, nil
}

// formatRelativeTime converts a Unix timestamp to a human-readable relative time string
func formatRelativeTime(unixTimestamp int64) string {
	now := time.Now()
	commitTime := time.Unix(unixTimestamp, 0)
	duration := now.Sub(commitTime)

	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	// Fallback to date for older commits
	return commitTime.Format("2006-01-02")
}

// gitExec is a helper to run git commands in a specific directory
func gitExec(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), string(output))
	}
	return nil
}
