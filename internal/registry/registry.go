/*
 * Component: Registry File I/O
 * Block-UUID: 3b44d021-8dd1-4927-9008-8e8e9ab9a1f0
 * Parent-UUID: 13c3751e-ec29-4169-8515-8628f699c736
 * Version: 1.5.0
 * Description: Handles loading and saving the registry file (.gitsense/manifest.json). Updated AddEntry to use UpsertEntry logic to prevent duplicate registry entries and ensure the manifest acts as the source of truth. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted routine Info and Success logs to Debug level to enable quiet-by-default behavior.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// resolveRegistryPath constructs the absolute path to the manifest.json registry file.
// This is a private function to avoid circular imports.
func resolveRegistryPath() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		logger.Error("Failed to resolve registry path", "error", err)
		return "", err
	}

	registryPath := filepath.Join(root, settings.GitSenseDir, settings.RegistryFileName)
	return registryPath, nil
}

// LoadRegistry loads the registry from the .gitsense directory.
// If the registry file does not exist, it returns a new, empty registry.
func LoadRegistry() (*Registry, error) {
	registryPath, err := resolveRegistryPath()
	if err != nil {
		logger.Error("Failed to resolve registry path", "error", err)
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		logger.Debug("Registry file not found, creating new registry", "path", registryPath)
		return NewRegistry(), nil
	}

	// Read file
	data, err := os.ReadFile(registryPath)
	if err != nil {
		logger.Error("Failed to read registry file", "path", registryPath, "error", err)
		return nil, err
	}

	// Parse JSON
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		logger.Error("Failed to parse registry JSON", "path", registryPath, "error", err)
		return nil, err
	}

	logger.Debug("Registry loaded successfully", "path", registryPath, "databases", len(registry.Databases))
	return &registry, nil
}

// SaveRegistry saves the registry to the .gitsense directory.
// It creates the directory if it doesn't exist and writes the file with pretty formatting.
func SaveRegistry(registry *Registry) error {
	registryPath, err := resolveRegistryPath()
	if err != nil {
		logger.Error("Failed to resolve registry path", "error", err)
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create registry directory", "dir", dir, "error", err)
		return err
	}

	// Marshal JSON with indentation
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal registry JSON", "error", err)
		return err
	}

	// Write file
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		logger.Error("Failed to write registry file", "path", registryPath, "error", err)
		return err
	}

	logger.Debug("Registry saved successfully", "path", registryPath)
	return nil
}

// AddEntry adds a new database entry to the registry or updates an existing one.
// It uses UpsertEntry logic to prevent duplicate entries based on DatabaseName.
func AddEntry(entry RegistryEntry) error {
	registry, err := LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Use UpsertEntry to update existing or append new
	registry.UpsertEntry(entry)

	if err := SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}
