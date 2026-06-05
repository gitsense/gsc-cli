/**
 * Component: Manifest Initializer
 * Block-UUID: 533458b9-8eb2-4153-b43c-4e484f2bbf98
 * Parent-UUID: deded7eb-5738-43c2-b311-12704a3e3c6d
 * Version: 1.7.0
 * Description: Updated to use centralized gitignore service for .gitignore management. Replaced hardcoded .gitignore creation with gitignore.EnsureUpdated() to ensure consistent pattern management across all GitSense features.
 * Language: Go
 * Created-at: 2026-02-05T00:42:23.439Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/gitignore"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// InitializeGitSense Chat creates the .gitsense directory structure and the manifest.json registry file.
// It performs the following steps:
// 1. Locates the project root.
// 2. Creates the .gitsense directory.
// 3. Initializes an empty manifest.json registry.
// 4. Creates a .gitignore file within .gitsense to exclude database files.
func InitializeGitSense() error {
	// 1. Find Project Root
	projectRoot, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("GitSense Chat can only be initialized within a Git repository. Error: %w", err)
	}

	gitsenseDir := filepath.Join(projectRoot, settings.GitSenseDir)
	registryPath := filepath.Join(gitsenseDir, settings.RegistryFileName)

	// 2. Create .gitsense directory
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", gitsenseDir, err)
	}

	// 3. Check if registry already exists
	if _, err := os.Stat(registryPath); err == nil {
		logger.Debug("GitSense Chat workspace already initialized", "path", gitsenseDir)
		return nil
	}

	// 4. Create initial manifest.json registry
	initialRegistry := map[string]interface{}{
		"version":   "1.0",
		"databases": []map[string]interface{}{},
	}

	registryData, err := json.MarshalIndent(initialRegistry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal initial registry: %w", err)
	}

	if err := os.WriteFile(registryPath, registryData, 0644); err != nil {
		return fmt.Errorf("failed to write registry file %s: %w", registryPath, err)
	}

	// 5. Create .gitignore using centralized service
	if err := gitignore.EnsureUpdated(gitsenseDir, gitignore.Registration{
		Source: gitignore.SourceCore,
		Patterns: []string{"*.db", "*.sqlite", "*.sqlite3"},
		WarnFn: func(msg string) {
			logger.Debug("Gitignore updated", "message", msg)
		},
	}); err != nil {
		logger.Warning("Failed to create .gitignore", "dir", gitsenseDir, "error", err)
		// Non-fatal error, continue
	}

	logger.Debug("GitSense Chat workspace initialized successfully", "path", gitsenseDir)
	return nil
}
