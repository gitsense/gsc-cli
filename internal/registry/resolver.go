/*
 * Component: Database Resolver
 * Block-UUID: 4865ec7d-fe54-4375-817c-a8a8ab367c72
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides logic to resolve user-provided database names or display names to their canonical physical DatabaseName.
 * Language: Go
 * Created-at: 2026-02-04T05:11:38.671Z
 * Authors: GLM-4.7 (v1.0.0)
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

	// 1. Try exact match on DatabaseName (the slug) first for performance/scripts
	for _, db := range reg.Databases {
		if db.DatabaseName == input {
			return db.DatabaseName, nil
		}
	}

	// 2. Try case-insensitive match on Name (the display label)
	for _, db := range reg.Databases {
		if strings.EqualFold(db.Name, input) {
			return db.DatabaseName, nil
		}
	}

	// 3. Try case-insensitive match on DatabaseName as a final fallback
	for _, db := range reg.Databases {
		if strings.EqualFold(db.DatabaseName, input) {
			return db.DatabaseName, nil
		}
	}

	return "", fmt.Errorf("database '%s' not found in registry", input)
}
