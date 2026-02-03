/*
 * Component: Git Repository Info
 * Block-UUID: b9d39a71-38d6-4fd0-9d3f-523f846f904a
 * Parent-UUID: 11c91bf9-d2d6-4b44-b913-544c871d5000
 * Version: 2.0.0
 * Description: Extracts repository information from .git/config and system information. Added GetSystemInfo to provide OS and project root details.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0)
 */


package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RepositoryInfo holds details about the git repository.
type RepositoryInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Remote string `json:"remote"`
}

// SystemInfo holds details about the execution environment.
type SystemInfo struct {
	OS          string `json:"os"`
	ProjectRoot string `json:"project_root"`
}

// GetRepositoryInfo reads .git/config and extracts repository metadata.
func GetRepositoryInfo() (*RepositoryInfo, error) {
	// 1. Find Project Root
	root, err := FindProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	// 2. Open .git/config
	gitConfigPath := filepath.Join(root, ".git", "config")
	file, err := os.Open(gitConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .git/config: %w", err)
	}
	defer file.Close()

	// 3. Parse Config
	scanner := bufio.NewScanner(file)
	var url string
	var remote string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Look for [remote "origin"] section
		if strings.Contains(line, `[remote "origin"]`) {
			remote = "origin"
		}

		// Look for url = ...
		if strings.HasPrefix(line, "url =") {
			url = strings.TrimPrefix(line, "url =")
			url = strings.TrimSpace(url)
		}
	}

	if url == "" {
		return nil, fmt.Errorf("no remote URL found in .git/config")
	}

	// 4. Extract Repo Name from URL
	// e.g., "https://github.com/yourusername/gsc-cli.git" -> "gsc-cli"
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")

	return &RepositoryInfo{
		Name:   name,
		URL:    url,
		Remote: remote,
	}, nil
}

// GetSystemInfo returns operating system and project root information.
func GetSystemInfo() (*SystemInfo, error) {
	root, err := FindProjectRoot()
	if err != nil {
		// If not in a git repo, return current working directory or empty string
		// For now, we'll return empty string if root not found
		return &SystemInfo{
			OS:          runtime.GOOS,
			ProjectRoot: "",
		}, nil
	}

	return &SystemInfo{
		OS:          runtime.GOOS,
		ProjectRoot: root,
	}, nil
}
