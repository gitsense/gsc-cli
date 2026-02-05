/*
 * Component: Manifest Querier
 * Block-UUID: 988bbe7d-ad2b-4be6-98be-2361ab2f7815
 * Parent-UUID: ab279929-5911-4ab5-8c87-45a0383a2516
 * Version: 1.5.0
 * Description: Logic to query the manifest registry and list available databases. Updated to use entry.DatabaseName instead of entry.Name to resolve the correct physical database file. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted routine Info logs to Debug level to enable quiet-by-default behavior.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
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
		// CRITICAL FIX: Use entry.DatabaseName (physical filename) instead of entry.Name (display name)
		// This ensures we connect to the correct database file (e.g., "secure-payments.db")
		// instead of guessing based on the display name (e.g., "Secure Payments Architecture.db").
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
			Name:        entry.Name,
			Description: entry.Description,
			Tags:        entry.Tags,
			DBPath:      dbPath,
			EntryCount:  count,
		})
	}

	logger.Debug("Found databases in registry", "count", len(databases))
	return databases, nil
}
