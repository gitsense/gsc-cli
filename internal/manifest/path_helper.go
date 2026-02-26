/*
 * Component: Manifest Path Helper
 * Block-UUID: 28743fe9-d0c3-48d6-a285-de4a3b7dbc17
 * Parent-UUID: b170a058-08e3-41a6-afc2-c90c4032975a
 * Version: 1.6.0
 * Description: Restored all original path resolution functions and added new helpers for Global (~/.gitsense) and Local (.gitsense) path resolution to support the Contract and Provenance systems.
 * Language: Go
 * Created-at: 2026-02-14T05:55:16.869Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0)
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

// ResolveDBPath constructs the absolute path to a database file within the .gitsense directory.
// It finds the project root and appends the database name with a .db extension.
func ResolveDBPath(dbName string) (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	dbPath := filepath.Join(root, settings.GitSenseDir, dbName+".db")
	return dbPath, nil
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

// ValidateDBExists checks if the database file exists on disk before attempting a connection.
// This prevents the SQLite driver from creating an empty file artifact if the database is missing.
func ValidateDBExists(dbName string) error {
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database '%s' not found at %s", dbName, dbPath)
	}
	return nil
}
