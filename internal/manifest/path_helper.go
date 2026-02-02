/*
 * Component: Manifest Path Helper
 * Block-UUID: e47b972f-549c-4f0f-a93f-6317b066be88
 * Parent-UUID: 23f54385-c17d-472e-b02b-a9967cc5f88e
 * Version: 1.2.0
 * Description: Helper functions to resolve file paths for databases and manifests within the .gitsense directory. Added ValidateDBExists to prevent creating empty database artifacts.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/settings"
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
