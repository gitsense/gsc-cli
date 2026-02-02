/*
 * Component: Manifest Initializer
 * Block-UUID: ea6df520-dec1-4616-bd8b-42ea7c75d7b4
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to initialize the .gitsense directory structure and registry file.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4. (v1.0.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/manifest"
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
	projectRoot, err := path_helper.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	gitsenseDir := filepath.Join(projectRoot, settings.GitSenseDir)
	registryPath := filepath.Join(gitsenseDir, "manifest.json")
	gitignorePath := filepath.Join(gitsenseDir, ".gitignore")

	// 2. Create .gitsense directory
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", gitsenseDir, err)
	}

	// 3. Check if registry already exists
	if _, err := os.Stat(registryPath); err == nil {
		logger.Info(fmt.Sprintf("GitSense directory already initialized at %s", gitsenseDir))
		return nil
	}

	// 4. Create initial manifest.json registry
	// We use a map for the initial structure to avoid importing the registry package here,
	// keeping dependencies minimal.
	initialRegistry := map[string]interface{}{
		"version": "1.0",
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
	gitignoreContent := "*.db\n"
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		logger.Error(fmt.Sprintf("Warning: failed to create .gitignore in %s: %v", gitsenseDir, err))
		// Non-fatal error, continue
	}

	logger.Success(fmt.Sprintf("GitSense initialized successfully at %s", gitsenseDir))
	return nil
}
