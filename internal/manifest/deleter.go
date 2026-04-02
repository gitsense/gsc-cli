/**
 * Component: Manifest Deleter
 * Block-UUID: 4432f900-aa3c-4bc4-85cc-084266f9951d
 * Parent-UUID: 3b9c1d2e-5f6a-4b7c-8d9e-0a1b2c3d4e5f
 * Version: 1.2.0
 * Description: Logic to delete a manifest database file and remove its entry from the registry.
 * Language: Go
 * Created-at: 2026-04-01T23:37:34.352Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), claude-haiku-4-5-20251001 (v1.2.0)
 */


package manifest

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// DeleteManifest removes the database file and the registry entry for the given database name.
// It performs the following steps:
// 1. Loads the registry to verify the database exists.
// 2. Resolves the physical path of the database file.
// 3. Deletes the physical .db file from the filesystem.
// 4. Removes the entry from the registry.
// 5. Saves the updated registry.
func DeleteManifest(dbName string) error {
	// 1. Load Registry
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// 2. Check if entry exists
	entry, exists := reg.FindEntryByDBName(dbName)
	if !exists {
		return fmt.Errorf("database '%s' not found in registry", dbName)
	}

	// 3. Resolve DB Path
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	// 4. Delete Physical File
	if err := os.Remove(dbPath); err != nil {
		// Check if it's a "no such file" error - maybe it was already deleted manually?
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete database file: %w", err)
		}
		logger.Warning("Database file not found on disk, removing from registry only", "path", dbPath)
	}

	// 5. Remove from Registry
	if !reg.RemoveEntryByDBName(dbName) {
		// This should theoretically not happen since we checked existence above,
		// but good to be defensive.
		return fmt.Errorf("failed to remove entry from registry (logic error)")
	}

	// 6. Save Registry
	if err := registry.SaveRegistry(reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	logger.Success("Successfully deleted manifest", "manifest", entry.ManifestName, "db", dbName)
	return nil
}
