/**
 * Component: Database Resolver
 * Block-UUID: b249e58d-0270-4f48-a689-68cc3633a320
 * Parent-UUID: 4865ec7d-fe54-4375-817c-a8a8ab367c72
 * Version: 1.1.0
 * Description: Provides logic to resolve user-provided database names or display names to their canonical physical DatabaseName.
 * Language: Go
 * Created-at: 2026-02-05T07:25:33.531Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package registry

import (
	"fmt"
	"strings"
)

// ResolveDatabase takes a user-provided string (either the display Name or the
// physical DatabaseName) and returns the canonical DatabaseName used for file paths.
// It prioritizes exact matches for performance, then falls back to case-insensitive
// matches on the display name for better user experience.
func ResolveDatabase(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("database name cannot be empty")
	}

	reg, err := LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("failed to load registry for resolution: %w", err)
	}

	// Sanitize input: strip common extensions and whitespace
	// This allows users to type "mydb.db" or "mydb.sqlite" and it will resolve correctly
	cleanInput := sanitizeInput(input)

	// 1. Try exact match on DatabaseName (the slug) first for performance/scripts
	for _, db := range reg.Databases {
		if db.DatabaseName == cleanInput {
			return db.DatabaseName, nil
		}
	}

	// 2. Try case-insensitive match on Name (the display label)
	for _, db := range reg.Databases {
		if strings.EqualFold(db.Name, cleanInput) {
			return db.DatabaseName, nil
		}
	}

	// 3. Try case-insensitive match on DatabaseName as a final fallback
	for _, db := range reg.Databases {
		if strings.EqualFold(db.DatabaseName, cleanInput) {
			return db.DatabaseName, nil
		}
	}

	return "", fmt.Errorf("database '%s' not found in registry", input)
}

// sanitizeInput removes common database file extensions and trims whitespace.
// It ensures that inputs like "mydb.db" or "mydb.sqlite" are treated as "mydb".
func sanitizeInput(input string) string {
	// Trim whitespace
	clean := strings.TrimSpace(input)

	// List of extensions to strip (case-insensitive)
	extensions := []string{".db", ".sqlite", ".sqlite3"}

	lowerClean := strings.ToLower(clean)
	for _, ext := range extensions {
		if strings.HasSuffix(lowerClean, ext) {
			return clean[:len(clean)-len(ext)]
		}
	}

	return clean
}
