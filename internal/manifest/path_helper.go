/*
 * Component: Manifest Path Helper
 * Block-UUID: 497c60a5-854b-483c-b619-2aa807e762ed
 * Parent-UUID: 59c7d1db-e10b-4cfc-87b9-a71ad8acd1e6
 * Version: 1.4.0
 * Description: Helper functions to resolve file paths for databases and manifests. Added ResolveLockPath to support file-based locking for concurrent import prevention.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
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
