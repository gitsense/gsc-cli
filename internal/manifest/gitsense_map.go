/*
 * Component: GitSense Map Loader
 * Block-UUID: 6bd75f8e-b0fe-424b-80cf-6a438e6e23fa
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Handles loading and validation of the project-level .gitsense-map file. This file defines team-wide defaults for Focus Scope that are version-controlled.
 * Language: Go
 * Created-at: 2026-02-04T22:50:15.123Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// GitSenseMap represents the structure of the .gitsense-map file.
// It mirrors the ScopeConfig structure defined in scope.go.
type GitSenseMap struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// LoadGitSenseMap reads the .gitsense-map file from the repository root.
// If the file does not exist, it returns nil (no error), indicating no project-level scope is defined.
// If the file exists but is invalid, it returns an error.
func LoadGitSenseMap() (*ScopeConfig, error) {
	// 1. Find Project Root
	root, err := git.FindProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	// 2. Construct Path to .gitsense-map
	// Note: This file lives at the repository root, next to .git, not inside .gitsense/
	mapPath := filepath.Join(root, ".gitsense-map")

	// 3. Check if file exists
	if _, err := os.Stat(mapPath); os.IsNotExist(err) {
		logger.Debug("No .gitsense-map file found at project root")
		return nil, nil
	}

	// 4. Read File
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .gitsense-map: %w", err)
	}

	// 5. Parse JSON
	var gitsenseMap GitSenseMap
	if err := json.Unmarshal(data, &gitsenseMap); err != nil {
		return nil, fmt.Errorf("failed to parse .gitsense-map JSON: %w", err)
	}

	// 6. Convert to ScopeConfig
	scopeConfig := &ScopeConfig{
		Include: gitsenseMap.Include,
		Exclude: gitsenseMap.Exclude,
	}

	logger.Debug("Successfully loaded .gitsense-map", "include_count", len(scopeConfig.Include), "exclude_count", len(scopeConfig.Exclude))
	return scopeConfig, nil
}

// ValidateGitSenseMap checks if the .gitsense-map file is valid.
// It performs basic structural validation and checks for obvious pattern errors.
func ValidateGitSenseMap() error {
	// 1. Load the map
	scope, err := LoadGitSenseMap()
	if err != nil {
		return err
	}

	// 2. If no map exists, that is valid
	if scope == nil {
		return nil
	}

	// 3. Validate patterns (basic check for empty strings or obvious syntax issues)
	// Note: Full glob validation happens during scope resolution/usage
	for _, pattern := range scope.Include {
		if pattern == "" {
			return fmt.Errorf(".gitsense-map contains empty include pattern")
		}
	}

	for _, pattern := range scope.Exclude {
		if pattern == "" {
			return fmt.Errorf(".gitsense-map contains empty exclude pattern")
		}
	}

	logger.Success(".gitsense-map is valid")
	return nil
}
