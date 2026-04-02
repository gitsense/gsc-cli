/**
 * Component: Manifest Path Helper
 * Block-UUID: f72fd8c9-8341-493f-9be2-81483eb5f541
 * Parent-UUID: 28743fe9-d0c3-48d6-a285-de4a3b7dbc17
 * Version: 1.7.0
 * Description: Restored all original path resolution functions and added new helpers for Global (~/.gitsense) and Local (.gitsense) path resolution to support the Contract and Provenance systems.
 * Language: Go
 * Created-at: 2026-04-01T23:02:04.436Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0), claude-haiku-4-5-20251001 (v1.7.0)
 */


package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ResolveGlobalDir returns the absolute path to the global .gitsense directory.
// It respects the GSC_HOME environment variable if set.
func ResolveGlobalDir() (string, error) {
	return settings.GetGSCHome(false)
}

// ResolveGlobalContractDir returns the path to the global contracts store in GSC_HOME.
func ResolveGlobalContractDir() (string, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", err
	}
	contractDir := filepath.Join(gscHome, settings.ContractsRelPath)
	if err := os.MkdirAll(contractDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create global contracts directory: %w", err)
	}
	return contractDir, nil
}

// ResolveLocalProvenanceLog returns the path to the project-local provenance log.
func ResolveLocalProvenanceLog() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, settings.GitSenseDir, settings.ProvenanceFileName), nil
}

// ResolveTempDBPath constructs the absolute path to a temporary database file.
// This is used during atomic imports to build the database before swapping it into place.
func ResolveTempDBPath(dbName string) (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	tempPath := filepath.Join(root, settings.GitSenseDir, dbName+settings.TempDBSuffix)
	return tempPath, nil
}

// ResolveBackupDir constructs the absolute path to the backups directory.
// It ensures the directory exists before returning the path.
func ResolveBackupDir() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	backupDir := filepath.Join(root, settings.GitSenseDir, settings.BackupsDir)
	
	// Ensure the directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.Error("Failed to create backup directory", "dir", backupDir, "error", err)
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	return backupDir, nil
}

// ResolveLockPath constructs the absolute path to the import lock file.
// This file is used to prevent concurrent import operations.
func ResolveLockPath() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	lockPath := filepath.Join(root, settings.GitSenseDir, ".import.lock")
	return lockPath, nil
}

// ResolveJSONPath constructs the absolute path to a JSON manifest file within the .gitsense directory.
// It finds the project root and appends the provided filename.
func ResolveJSONPath(filename string) (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	jsonPath := filepath.Join(root, settings.GitSenseDir, filename)
	return jsonPath, nil
}

// ValidateWorkspace checks if the .gitsense directory and the manifest registry file exist.
// This ensures the workspace is properly initialized before performing operations like import.
func ValidateWorkspace() error {
	root, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("GitSense Chat can only be used within a Git repository: %w", err)
	}

	gitsenseDir := filepath.Join(root, settings.GitSenseDir)
	if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
		return fmt.Errorf("GitSense workspace not found at %s. Please run 'gsc manifest init' first to initialize the workspace", gitsenseDir)
	}

	registryPath := filepath.Join(gitsenseDir, settings.RegistryFileName)
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return fmt.Errorf("GitSense registry not found at %s. The workspace may be corrupted. Please run 'gsc manifest init' to repair it", registryPath)
	}

	return nil
}
