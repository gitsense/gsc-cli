/**
 * Component: Manifest Initializer
 * Block-UUID: 5db9b412-9a44-4ec7-9e51-1b32dc1caae1
 * Parent-UUID: b7473786-f319-4b8f-b8b5-9771bbacb10e
 * Version: 1.3.0
 * Description: Logic to initialize the .gitsense directory structure and registry file.
 * Language: Go
 * Created-at: 2026-02-02T07:12:37.835Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// InitializeGitSense creates the .gitsense directory structure and the manifest.json registry file.
// It performs the following steps:
// 1. Locates the project root.
// 2. Creates the .gitsense directory.
// 3. Initializes an empty manifest.json registry.
// 4. Creates a .gitignore file within .gitsense to exclude database files.
func InitializeGitSense() error {
	// 1. Find Project Root
	projectRoot, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("GitSense can only be initialized within a Git repository. Error: %w", err)
	}

	gitsenseDir := filepath.Join(projectRoot, settings.GitSenseDir)
	registryPath := filepath.Join(gitsenseDir, settings.RegistryFileName)
	gitignorePath := filepath.Join(gitsenseDir, ".gitignore")

	// 2. Create .gitsense directory
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", gitsenseDir, err)
	}

	// 3. Check if registry already exists
	if _, err := os.Stat(registryPath); err == nil {
		logger.Info("GitSense directory already initialized at %s", gitsenseDir)
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

	// 5. Create .gitignore to ignore .db files
	gitignoreContent := "*.db\n*.sqlite\n*.sqlite3\n"
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		logger.Warning("Failed to create .gitignore in %s: %v", gitsenseDir, err)
		// Non-fatal error, continue
	}

	logger.Success("GitSense initialized successfully at %s", gitsenseDir)
	return nil
}
