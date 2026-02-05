/**
 * Component: Manifest Initializer
 * Block-UUID: ebea8209-f9a0-40b4-beec-fefe41e938ec
 * Parent-UUID: 284a690b-979e-4495-bcdc-3b27a4ca67bc
 * Version: 1.4.1
 * Description: Logic to initialize the .gitsense directory structure and registry file. Reclassified internal state logs to Debug level to support quiet mode by default.
 * Language: Go
 * Created-at: 2026-02-05T00:42:23.439Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1)
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
		logger.Info("GitSense workspace already initialized at", gitsenseDir)
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

	logger.Info("GitSense workspace initialized successfully at", gitsenseDir)
	return nil
}
