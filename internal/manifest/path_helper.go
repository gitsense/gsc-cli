/*
 * Component: Manifest Path Helper
 * Block-UUID: 27c603ce-3c45-4e31-8dce-8d62c97da032
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Helper functions to resolve file paths for databases and manifests within the .gitsense directory.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"fmt"
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

// ResolveRegistryPath constructs the absolute path to the manifest.json registry file.
func ResolveRegistryPath() (string, error) {
	return ResolveJSONPath("manifest.json")
}
