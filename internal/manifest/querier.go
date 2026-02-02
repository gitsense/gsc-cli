/*
 * Component: Manifest Querier
 * Block-UUID: 0217de9f-52e5-44af-83a8-07e9c196de14
 * Parent-UUID: 5a0f9391-feaa-4047-9e69-215d5972c442
 * Version: 1.1.0
 * Description: Logic to query the manifest registry and list available databases.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0)
 */


package manifest

import (
	"context"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/git"
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
	root, err := git.FindProjectRoot()
	if err != nil {
		logger.Error("Failed to find project root: %v", err)
		return nil, err
	}

	// 2. Load the registry file
	reg, err := registry.LoadRegistry()
	if err != nil {
		logger.Error("Failed to load registry: %v", err)
		return nil, err
	}

	// 3. Convert registry entries to DatabaseInfo structs
	var databases []DatabaseInfo
	for _, entry := range reg.Databases {
		dbPath := filepath.Join(root, ".gitsense", entry.Name+".db")

		databases = append(databases, DatabaseInfo{
			Name:        entry.Name,
			Description: entry.Description,
			Tags:        entry.Tags,
			DBPath:      dbPath,
			EntryCount:  0, // TODO: Query the database to get actual entry count
		})
	}

	logger.Info("Found %d databases in registry", len(databases))
	return databases, nil
}
