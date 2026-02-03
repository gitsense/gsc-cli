/*
 * Component: Manifest Querier
 * Block-UUID: 67fed41a-c28b-4ea4-b111-9d92728edab0
 * Parent-UUID: 0217de9f-52e5-44af-83a8-07e9c196de14
 * Version: 1.2.0
 * Description: Logic to query the manifest registry and list available databases. Updated to query the actual file count from the database instead of returning 0.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package manifest

import (
	"context"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/db"
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

		// Query the database to get the actual file count
		var count int
		database, err := db.OpenDB(dbPath)
		if err != nil {
			logger.Warning("Failed to open database '%s' for counting: %v", entry.Name, err)
			count = 0
		} else {
			defer db.CloseDB(database)
			row := database.QueryRow("SELECT COUNT(*) FROM files")
			if err := row.Scan(&count); err != nil {
				logger.Warning("Failed to count files in database '%s': %v", entry.Name, err)
				count = 0
			}
		}

		databases = append(databases, DatabaseInfo{
			Name:        entry.Name,
			Description: entry.Description,
			Tags:        entry.Tags,
			DBPath:      dbPath,
			EntryCount:  count,
		})
	}

	logger.Info("Found %d databases in registry", len(databases))
	return databases, nil
}
