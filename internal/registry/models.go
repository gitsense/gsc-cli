/**
 * Component: Registry Models
 * Block-UUID: 52d1e0b1-eda0-422b-8b1f-2cb700862252
 * Parent-UUID: b3278380-41a1-4ed6-a24c-f88422c54be5
 * Version: 1.6.0
 * Description: Defines the data structures for the GitSense registry file (.gitsense/manifest.json). Updated method signatures and logic to explicitly use 'databaseLabel' instead of 'name' to align with the manifest schema and clarify the distinction between the human-readable label and the physical database name.
 * Language: Go
 * Created-at: 2026-02-11T03:11:09.766Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), GLM-4.7 (v1.6.0)
 */


package registry

import "time"

// Registry represents the top-level structure of the .gitsense/manifest.json file.
// It acts as a central index for all manifest databases in the repository.
type Registry struct {
	Version   string          `json:"version"`   // Schema version of the registry file
	Databases []RegistryEntry `json:"databases"` // List of registered databases
}

// RegistryEntry represents a single manifest database entry in the registry.
// It provides metadata about the database so agents and users can discover and select the right one.
type RegistryEntry struct {
	ManifestName  string    `json:"name"`           // The human-readable display name for the manifest, typically 2-3 capitalized words (e.g., "Secure Payments Architecture").
	DatabaseName  string    `json:"database_name"`  // The physical filename of the database (e.g., "secure-payments")
	Description   string    `json:"description"`    // Human-readable description of the database's purpose
	Tags          []string  `json:"tags"`           // Keywords for categorization (e.g., ["security", "javascript"])
	Version       string    `json:"version"`        // Version of the manifest data
	CreatedAt     time.Time `json:"created_at"`     // Timestamp when the database was created
	UpdatedAt     time.Time `json:"updated_at"`     // Timestamp when the database was last updated
	SourceFile    string    `json:"source_file"`    // The original JSON file used to import this database
}

// NewRegistry creates a new, empty Registry with the current schema version.
func NewRegistry() *Registry {
	return &Registry{
		Version:   "1.0",
		Databases: []RegistryEntry{},
	}
}

// AddEntry adds a new database entry to the registry.
// Deprecated: Use UpsertEntry for idempotent updates.
func (r *Registry) AddEntry(entry RegistryEntry) {
	r.Databases = append(r.Databases, entry)
}

// UpsertEntry updates an existing database entry by DatabaseName or appends it as new.
// This ensures the registry acts as a source of truth and prevents duplicate entries.
func (r *Registry) UpsertEntry(entry RegistryEntry) {
	for i, existing := range r.Databases {
		if existing.DatabaseName == entry.DatabaseName {
			// Replace entire entry (Source of Truth from manifest)
			r.Databases[i] = entry
			return
		}
	}
	// Not found, append as new
	r.Databases = append(r.Databases, entry)
}

// FindEntry searches for a database entry by human-readable manifest name.
// Returns the entry and true if found, nil and false otherwise.
func (r *Registry) FindEntry(manifestName string) (*RegistryEntry, bool) {
	for i := range r.Databases {
		if r.Databases[i].ManifestName == manifestName {
			return &r.Databases[i], true
		}
	}
	return nil, false
}

// FindEntryByDBName searches for a database entry by its physical database name (slug).
// Returns the entry and true if found, nil and false otherwise.
func (r *Registry) FindEntryByDBName(dbName string) (*RegistryEntry, bool) {
	for i := range r.Databases {
		if r.Databases[i].DatabaseName == dbName {
			return &r.Databases[i], true
		}
	}
	return nil, false
}

// RemoveEntry removes a database entry by human-readable manifest name.
// Returns true if an entry was removed, false if it wasn't found.
func (r *Registry) RemoveEntry(manifestName string) bool {
	for i, entry := range r.Databases {
		if entry.ManifestName == manifestName {
			r.Databases = append(r.Databases[:i], r.Databases[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveEntryByDBName removes a database entry by its physical database name (slug).
// Returns true if an entry was removed, false if it wasn't found.
func (r *Registry) RemoveEntryByDBName(dbName string) bool {
	for i, entry := range r.Databases {
		if entry.DatabaseName == dbName {
			r.Databases = append(r.Databases[:i], r.Databases[i+1:]...)
			return true
		}
	}
	return false
}
