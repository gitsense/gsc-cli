/**
 * Component: Manifest Querier
 * Block-UUID: 9b8c7d6e-5f4a-4b3c-9d2e-1f3a4b5c6d7e
 * Parent-UUID: 25a66526-34d1-401d-83e7-ff4469ee538c
 * Version: 1.7.0
 * Description: Logic to query the manifest registry and list available databases. Updated DatabaseInfo struct to use DatabaseName and DatabaseLabel. Updated mapping logic to populate these fields correctly from the registry entry.
 * Language: Go
 * Created-at: 2026-02-11T03:18:10.918Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0)
 */


package manifest

import (
	"context"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// DatabaseInfo represents summary information about a manifest database
type DatabaseInfo struct {
	DatabaseName  string   `json:"database_name"`  // The physical slug/ID
	ManifestName  string   `json:"name"`           // The human-readable manifest name
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	DBPath        string   `json:"db_path"`
	EntryCount    int      `json:"entry_count"`
}

// ListDatabases retrieves all registered databases from the manifest registry
// and returns a summary list.
func ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	// 1. Find the project root to locate the .gitsense directory
	root, err := git.FindProjectRoot()
	if err != nil {
		logger.Error("Failed to find project root", "error", err)
		return nil, err
	}

	// 2. Load the registry file
	reg, err := registry.LoadRegistry()
	if err != nil {
		logger.Error("Failed to load registry", "error", err)
		return nil, err
	}

	// 3. Convert registry entries to DatabaseInfo structs
	var databases []DatabaseInfo
	for _, entry := range reg.Databases {
		// Use entry.DatabaseName (physical filename) to construct the path
		dbPath := filepath.Join(root, ".gitsense", entry.DatabaseName+".db")

		// Query the database to get the actual file count
		var count int
		database, err := db.OpenDB(dbPath)
		if err != nil {
			logger.Warning("Failed to open database for counting", "db", entry.DatabaseName, "error", err)
			count = 0
		} else {
			defer db.CloseDB(database)
			row := database.QueryRow("SELECT COUNT(*) FROM files")
			if err := row.Scan(&count); err != nil {
				logger.Warning("Failed to count files in database", "db", entry.DatabaseName, "error", err)
				count = 0
			}
		}

		databases = append(databases, DatabaseInfo{
			DatabaseName:  entry.DatabaseName,
			ManifestName:  entry.ManifestName,
			Description:   entry.Description,
			Tags:          entry.Tags,
			DBPath:        dbPath,
			EntryCount:    count,
		})
	}

	logger.Debug("Found databases in registry", "count", len(databases))
	return databases, nil
}
