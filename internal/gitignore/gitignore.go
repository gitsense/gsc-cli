/*
 * Component: GitIgnore Service
 * Block-UUID: bdb306b2-fe5c-4e20-b50f-e8371b8ab220
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Centralized service for managing .gitsense/.gitignore patterns. Provides EnsureUpdated() for features to register patterns and Regenerate() for CLI command to rebuild the file.
 * Language: Go
 * Created-at: 2026-06-04T12:57:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package gitignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Source represents where a pattern comes from
type Source string

const (
	SourceCore     Source = "core"     // Core database and temp files
	SourceImport   Source = "import"   // Import operation state
	SourceManifest Source = "manifest" // Manifest derived state
	SourceExperts  Source = "experts"  // Expert context generation
	SourceConfig   Source = "config"   // Configuration and profiles
	SourceClaude   Source = "claude"   // Claude change turn state
)

// Registration represents a feature's request to ensure patterns are in .gitignore
type Registration struct {
	Source   Source
	Patterns []string
	WarnFn   func(msg string) // nil = silent
}

// Pattern represents a single .gitignore pattern with metadata
type Pattern struct {
	Pattern string
	Source  Source
	Comment string
}

// GetPatterns returns all patterns for the given sources
func GetPatterns(sources ...Source) []Pattern {
	allPatterns := []Pattern{
		// Core database and temp files
		{Pattern: "*.db", Source: SourceCore, Comment: "SQLite database files"},
		{Pattern: "*.sqlite", Source: SourceCore, Comment: "SQLite database files"},
		{Pattern: "*.sqlite3", Source: SourceCore, Comment: "SQLite database files"},
		{Pattern: "*.tmp", Source: SourceCore, Comment: "Temporary database files during atomic import"},
		
		// Import operation state
		{Pattern: "import-git.json", Source: SourceImport, Comment: "Git import state file"},
		{Pattern: ".import.lock", Source: SourceImport, Comment: "Import concurrency lock file"},
		{Pattern: "backups/", Source: SourceImport, Comment: "Backup directory"},
		
		// Manifest derived state
		{Pattern: "manifest.json", Source: SourceManifest, Comment: "Manifest registry (derived state, can go out of sync if DB deleted)"},
		
		// Expert context generation
		{Pattern: "experts-context.md", Source: SourceExperts, Comment: "Generated expert context file"},
		
		// Configuration and profiles
		{Pattern: "config.json", Source: SourceConfig, Comment: "Query configuration file"},
		{Pattern: "profiles/", Source: SourceConfig, Comment: "Context profile directory"},
		{Pattern: "*.profile.json", Source: SourceConfig, Comment: "Profile configuration files"},
		
		// Claude change turn state
		{Pattern: ".change-meta.json", Source: SourceClaude, Comment: "Per-file change metadata (ephemeral, cleaned up post-turn)"},
		{Pattern: "change-metadata.jsonl", Source: SourceClaude, Comment: "Turn-local metadata aggregation for resumption (ephemeral)"},
	}
	
	// Filter by requested sources
	if len(sources) == 0 {
		return allPatterns
	}
	
	sourceSet := make(map[Source]bool)
	for _, s := range sources {
		sourceSet[s] = true
	}
	
	var filtered []Pattern
	for _, p := range allPatterns {
		if sourceSet[p.Source] {
			filtered = append(filtered, p)
		}
	}
	
	return filtered
}

// GenerateContent generates the .gitignore file content
func GenerateContent(sources ...Source) string {
	patterns := GetPatterns(sources...)
	
	var builder strings.Builder
	builder.WriteString("# This file is programmatically generated. DO NOT EDIT MANUALLY.\n")
	builder.WriteString("# To regenerate, run: gsc gitignore update\n")
	builder.WriteString("#\n")
	builder.WriteString(fmt.Sprintf("# Generated on: %s\n", time.Now().Format(time.RFC3339)))
	builder.WriteString("\n")
	
	currentSource := Source("")
	for _, p := range patterns {
		if p.Source != currentSource {
			if currentSource != "" {
				builder.WriteString("\n")
			}
			builder.WriteString("# ")
			builder.WriteString(string(p.Source))
			builder.WriteString("\n")
			currentSource = p.Source
		}
		if p.Comment != "" {
			builder.WriteString("# ")
			builder.WriteString(p.Comment)
			builder.WriteString("\n")
		}
		builder.WriteString(p.Pattern)
		builder.WriteString("\n")
	}
	
	return builder.String()
}

// WriteGitignore writes the .gitignore file to the .gitsense directory
func WriteGitignore(gitsenseDir string, sources ...Source) error {
	gitignorePath := filepath.Join(gitsenseDir, ".gitignore")
	
	content := GenerateContent(sources...)
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

// EnsureUpdated ensures the .gitignore file exists and includes patterns from the given registration
// It merges new patterns into the existing file without removing patterns from other sources
func EnsureUpdated(gitsenseDir string, reg Registration) error {
	gitignorePath := filepath.Join(gitsenseDir, ".gitignore")
	
	// Check if file exists
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		// File doesn't exist, create it with all patterns
		if err := WriteGitignore(gitsenseDir); err != nil {
			return err
		}
		if reg.WarnFn != nil {
			reg.WarnFn(fmt.Sprintf("Created .gitsense/.gitignore with patterns for %s", reg.Source))
		}
		return nil
	}
	
	// File exists - check if it's managed by gsc
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return err
	}
	
	// If it doesn't have our header, it's manually managed - don't touch it
	if !strings.Contains(string(content), "# This file is programmatically generated") {
		if reg.WarnFn != nil {
			reg.WarnFn(fmt.Sprintf(".gitsense/.gitignore is manually managed. Patterns for %s not added.", reg.Source))
		}
		return nil
	}
	
	// File is managed by gsc - check if patterns for this source are present
	contentStr := string(content)
	
	// Check if any pattern from this source is missing
	missingPatterns := false
	for _, pattern := range reg.Patterns {
		if !strings.Contains(contentStr, pattern) {
			missingPatterns = true
			break
		}
	}
	
	if !missingPatterns {
		// All patterns present, nothing to do
		return nil
	}
	
	// Regenerate with all patterns to ensure consistency
	if err := WriteGitignore(gitsenseDir); err != nil {
		return err
	}
	
	if reg.WarnFn != nil {
		reg.WarnFn(fmt.Sprintf("Updated .gitsense/.gitignore with patterns for %s", reg.Source))
	}
	
	return nil
}

// Regenerate rewrites the entire .gitignore file from scratch
func Regenerate(gitsenseDir string) error {
	return WriteGitignore(gitsenseDir)
}
