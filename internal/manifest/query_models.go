/*
 * Component: Query Models
 * Block-UUID: 854ea0c7-91a2-44dc-b3ab-bfd0f3469775
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the Go structs for query operations, configuration, and list results for the Phase 3 query command.
 * Language: Go
 * Created-at: 2026-02-02T18:45:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

// SimpleQuery represents a basic query request to find files by metadata value.
type SimpleQuery struct {
	Database   string `json:"database"`   // The database to query
	MatchField string `json:"match_field"` // The field to match against
	MatchValue string `json:"match_value"` // The value to match (comma-separated for OR logic)
}

// QueryResult represents a single file result from a query.
type QueryResult struct {
	FilePath string `json:"file_path"` // The path to the file
	ChatID   int    `json:"chat_id"`   // The GitSense Chat ID for the file
}

// ListResult represents the result of a --list operation.
// It can represent a list of databases, fields within a database, or values within a field.
type ListResult struct {
	Level string     `json:"level"` // "database", "field", or "value"
	Items []ListItem `json:"items"`
}

// ListItem represents a single item in a list result.
type ListItem struct {
	Name        string `json:"name"`                  // The name of the item (db, field, or value)
	Description string `json:"description,omitempty"` // Optional description
	Type        string `json:"type,omitempty"`        // Optional type (for fields)
	Count       int    `json:"count,omitempty"`       // Optional count (for values)
}

// QueryConfig represents the configuration stored in .gitsense/config.json.
// This file manages shared defaults and aliases for the query and rg commands.
type QueryConfig struct {
	Query struct {
		DefaultDatabase string                 `json:"default_database"` // Default database for queries
		DefaultField    string                 `json:"default_field"`    // Default field for queries
		DefaultFormat   string                 `json:"default_format"`   // Default output format
		Aliases         map[string]QueryAlias  `json:"aliases"`         // Saved query aliases
		History         []string               `json:"history"`         // Recent query history
	} `json:"query"`
	RG struct {
		DefaultDatabase string `json:"default_database"` // Default database for ripgrep enrichment
		DefaultFormat   string `json:"default_format"`   // Default output format for ripgrep
		DefaultContext  int    `json:"default_context"`  // Default lines of context
	} `json:"rg"`
}

// QueryAlias represents a saved query alias.
type QueryAlias struct {
	Database string `json:"database"` // The database to query
	Field    string `json:"field"`    // The field to match
	Value    string `json:"value"`    // The value to match
}
