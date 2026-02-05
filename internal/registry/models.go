/*
 * Component: Registry Models
 * Block-UUID: eb77ea0b-7f02-4764-8b65-64b3c759e163
 * Parent-UUID: e416d612-0822-41bb-a3eb-d4f6e98cd2f2
 * Version: 1.2.0
 * Description: Defines the data structures for the GitSense registry file (.gitsense/manifest.json). Added UpdatedAt field, UpsertEntry method for idempotent updates, and FindEntryByDBName for existence checks.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
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
	Name         string    `json:"name"`          // The human-readable display name for the database, typically 2-3 capitalized words (e.g., "Secure Payments Architecture").
	DatabaseName string    `json:"database_name"` // The physical filename of the database (e.g., "secure-payments")
	Description  string    `json:"description"`   // Human-readable description of the database's purpose
	Tags         []string  `json:"tags"`          // Keywords for categorization (e.g., ["security", "javascript"])
	Version      string    `json:"version"`       // Version of the manifest data
	CreatedAt    time.Time `json:"created_at"`    // Timestamp when the database was created
	UpdatedAt    time.Time `json:"updated_at"`    // Timestamp when the database was last updated
	SourceFile   string    `json:"source_file"`   // The original JSON file used to import this database
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

// FindEntry searches for a database entry by human-readable name.
// Returns the entry and true if found, nil and false otherwise.
func (r *Registry) FindEntry(name string) (*RegistryEntry, bool) {
	for i := range r.Databases {
		if r.Databases[i].Name == name {
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

// RemoveEntry removes a database entry by human-readable name.
// Returns true if an entry was removed, false if it wasn't found.
func (r *Registry) RemoveEntry(name string) bool {
	for i, entry := range r.Databases {
		if entry.Name == name {
			r.Databases = append(r.Databases[:i], r.Databases[i+1:]...)
			return true
		}
	}
	return false
}
