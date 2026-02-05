/**
 * Component: Scope Logic
 * Block-UUID: 4a59acca-2ca1-4b4b-b589-695c12d961c9
 * Parent-UUID: b5d5495d-7ccc-49d9-9eeb-4586b29ce5ab
 * Version: 1.0.2
 * Description: Core logic for Focus Scope handling, including parsing, matching, validation, and resolution. Implements lenient parsing, doublestar glob matching, Levenshtein distance suggestions, and the full precedence chain for scope resolution.
 * Language: Go
 * Created-at: 2026-02-05T00:10:41.709Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Claude Haiku 4.5 (v1.0.2)
 */


package manifest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// ScopeConfig defines the include and exclude patterns for a Focus Scope.
type ScopeConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// ScopeValidationResult contains the outcome of a scope validation check.
type ScopeValidationResult struct {
	TotalTrackedFiles int      `json:"total_tracked_files"`
	InScopeFiles      int      `json:"in_scope_files"`
	ExcludedFiles     int      `json:"excluded_files"`
	Warnings          []string `json:"warnings"`
	Suggestions       []string `json:"suggestions"`
}

// ParseScopeOverride parses a string input into a ScopeConfig.
// Supported formats:
// - "include=src/**,lib/**;exclude=3rdparty/**"
// - "include=backend/**"
// - "exclude=tests/**"
// - "" (returns nil, implying default scope)
func ParseScopeOverride(input string) (*ScopeConfig, error) {
	if input == "" {
		return nil, nil
	}

	config := &ScopeConfig{}
	parts := strings.Split(input, ";")

	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid scope format: '%s'. Expected 'include=...' or 'exclude=...'", part)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		patterns := strings.Split(value, ",")
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}

		switch key {
		case "include":
			config.Include = patterns
		case "exclude":
			config.Exclude = patterns
		default:
			return nil, fmt.Errorf("unknown scope key: '%s'. Expected 'include' or 'exclude'", key)
		}
	}

	return config, nil
}

// MatchScope checks if a file path matches the provided scope configuration.
// Rules:
// 1. If Include is empty, all files are initially candidates.
// 2. If Include is not empty, file must match at least one include pattern.
// 3. If Exclude is not empty, file must NOT match any exclude pattern.
func MatchScope(filePath string, scope *ScopeConfig) bool {
	if scope == nil {
		return true // Default scope matches everything
	}

	// Check Include patterns
	if len(scope.Include) > 0 {
		matched := false
		for _, pattern := range scope.Include {
			if ok, _ := doublestar.Match(pattern, filePath); ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check Exclude patterns
	for _, pattern := range scope.Exclude {
		if ok, _ := doublestar.Match(pattern, filePath); ok {
			return false
		}
	}

	return true
}

// ValidateScope checks the scope patterns against the repository's tracked files.
// It returns counts and suggestions for patterns that match zero files.
func ValidateScope(ctx context.Context, scope *ScopeConfig, repoRoot string) (*ScopeValidationResult, error) {
	result := &ScopeValidationResult{}

	// 1. Get all tracked files
	trackedFiles, err := git.GetTrackedFiles(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked files for validation: %w", err)
	}
	result.TotalTrackedFiles = len(trackedFiles)

	// 2. Check Include patterns
	inScopeSet := make(map[string]bool)
	if scope != nil && len(scope.Include) > 0 {
		for _, pattern := range scope.Include {
			matches := 0
			for _, file := range trackedFiles {
				if ok, _ := doublestar.Match(pattern, file); ok {
					matches++
				}
			}
			if matches == 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Include pattern '%s' matched 0 files.", pattern))
				// Note: We still add to suggestions even if matches is 0
				suggestions := suggestPatternCorrection(pattern, trackedFiles)
				result.Suggestions = append(result.Suggestions, suggestions...)
			}
			// Track unique files for accurate counting
			for _, file := range trackedFiles {
				if ok, _ := doublestar.Match(pattern, file); ok {
					inScopeSet[file] = true
				}
			}
		}
	} else {
		// If no include patterns, all tracked files are in scope
		result.InScopeFiles = result.TotalTrackedFiles
	}

	// 3. Check Exclude patterns
	// Note: Excluded files are subtracted from the total tracked, not the in-scope set,
	// to match the logic of "Total - Excluded = In Scope" when no includes are defined.
	// However, strictly speaking, Excluded files should be those that match exclude patterns
	// regardless of whether they were included.
	if scope != nil && len(scope.Exclude) > 0 {
		for _, pattern := range scope.Exclude {
			matches := 0
			for _, file := range trackedFiles {
				if ok, _ := doublestar.Match(pattern, file); ok {
					matches++
				}
			}
			if matches == 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Exclude pattern '%s' matched 0 files.", pattern))
			}
			result.ExcludedFiles += matches
		}
	}

	// Update InScopeFiles with the unique count
	if len(inScopeSet) > 0 {
		result.InScopeFiles = len(inScopeSet)
	}

	return result, nil
}

// suggestPatternCorrection uses Levenshtein distance to suggest similar patterns.
func suggestPatternCorrection(userPattern string, repoFiles []string) []string {
	// Extract the first path component from the user pattern
	// e.g., "srce/**" -> "srce", "src/lib/**" -> "src"
	parts := strings.Split(userPattern, "/")
	if len(parts) == 0 {
		return nil
	}
	userDir := parts[0]

	// Get unique directories from repo files
	repoDirs := make(map[string]bool)
	for _, file := range repoFiles {
		dir := filepath.Dir(file)
		if dir == "." {
			continue
		}
		// Get top-level directory
		topLevel := strings.Split(dir, "/")[0]
		repoDirs[topLevel] = true
	}

	// Calculate distances
	type candidate struct {
		dir      string
		distance int
	}
	var candidates []candidate

	for dir := range repoDirs {
		dist := levenshtein.ComputeDistance(userDir, dir)
		if dist <= 2 && dist > 0 { // Threshold: max 2 char diff, but not exact match
			candidates = append(candidates, candidate{dir: dir, distance: dist})
		}
	}

	// Sort by distance
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Format suggestions
	var suggestions []string
	for i := 0; i < len(candidates) && i < 3; i++ {
		correctedPattern := strings.Replace(userPattern, userDir, candidates[i].dir, 1)
		suggestions = append(suggestions, fmt.Sprintf("Did you mean: '%s'", correctedPattern))
	}

	return suggestions
}

// ResolveScopeForQuery determines the active scope based on the precedence chain:
// 1. Command-line override
// 2. Active profile scope
// 3. Project .gitsense-map
// 4. Default (nil = all files)
func ResolveScopeForQuery(ctx context.Context, profileName string, scopeOverride string) (*ScopeConfig, error) {
	// Priority 1: Command-line override
	if scopeOverride != "" {
		scope, err := ParseScopeOverride(scopeOverride)
		if err != nil {
			return nil, fmt.Errorf("invalid --scope-override: %w", err)
		}
		logger.Debug("Using command-line scope override")
		return scope, nil
	}

	// Priority 2: Active Profile
	if profileName != "" {
		profile, err := LoadProfile(profileName)
		if err != nil {
			logger.Warning("Failed to load profile '%s' for scope resolution: %v", profileName, err)
		} else if profile.Settings.Global.Scope != nil {
			logger.Debug("Using scope from active profile '%s'", profileName)
			return profile.Settings.Global.Scope, nil
		}
	}

	// Priority 3: Project .gitsense-map
	// Note: LoadGitSenseMap is defined in gitsense_map.go
	projectMap, err := LoadGitSenseMap()
	if err != nil {
		logger.Debug("No .gitsense-map found or error loading: %v", err)
	} else if projectMap != nil {
		logger.Debug("Using scope from .gitsense-map")
		return projectMap, nil
	}

	// Priority 4: Default (All files)
	logger.Debug("Using default scope (all tracked files)")
	return nil, nil
}
