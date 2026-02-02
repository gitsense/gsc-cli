/*
 * Component: Manifest Querier
 * Block-UUID: 5a0f9391-feaa-4047-9e69-215d5972c442
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to query the manifest registry and list available databases.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// DatabaseInfo represents summary information about a manifest database
type DatabaseInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	DBPath      string   `json:"db_path"`
	EntryCount  int      `json:"entry_count"`
}

// ListDatabases retrieves all registered databases from the manifest registry
// and returns a summary list.
func ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	// 1. Find the project root to locate the .gitsense directory
	root, err := path_helper.FindProjectRoot()
	if err != nil {
		logger.Error("Failed to find project root: %v", err)
		return nil, err
	}

	// 2. Load the registry file
	reg, err := registry.LoadRegistry(root)
	if err != nil {
		logger.Error("Failed to load registry: %v", err)
		return nil, err
	}

	// 3. Convert registry entries to DatabaseInfo structs
	var databases []DatabaseInfo
	for _, entry := range reg.Entries {
		dbPath := filepath.Join(root, entry.DatabaseName+".db")
		
		databases = append(databases, DatabaseInfo{
			Name:        entry.DatabaseName,
			Description: entry.Description,
			Tags:        entry.Tags,
			DBPath:      dbPath,
			EntryCount:  len(entry.Files), // Assuming registry tracks file count
		})
	}

	logger.Info("Found %d databases in registry", len(databases))
	return databases, nil
}
