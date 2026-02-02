/*
 * Component: Registry Models
 * Block-UUID: 1c5e5a14-8423-4383-ba2f-d926d2637a70
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the data structures for the GitSense registry file (.gitsense/manifest.json), which tracks all available manifest databases.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
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
	Name        string    `json:"name"`        // The database name (e.g., "security", "performance")
	Description string    `json:"description"` // Human-readable description of the database's purpose
	Tags        []string  `json:"tags"`        // Keywords for categorization (e.g., ["security", "javascript"])
	Version     string    `json:"version"`     // Version of the manifest data
	CreatedAt   time.Time `json:"created_at"`  // Timestamp when the database was created
	SourceFile  string    `json:"source_file"` // The original JSON file used to import this database
}

// NewRegistry creates a new, empty Registry with the current schema version.
func NewRegistry() *Registry {
	return &Registry{
		Version:   "1.0",
		Databases: []RegistryEntry{},
	}
}

// AddEntry adds a new database entry to the registry.
func (r *Registry) AddEntry(entry RegistryEntry) {
	r.Databases = append(r.Databases, entry)
}

// FindEntry searches for a database entry by name.
// Returns the entry and true if found, nil and false otherwise.
func (r *Registry) FindEntry(name string) (*RegistryEntry, bool) {
	for i := range r.Databases {
		if r.Databases[i].Name == name {
			return &r.Databases[i], true
		}
	}
	return nil, false
}

// RemoveEntry removes a database entry by name.
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
