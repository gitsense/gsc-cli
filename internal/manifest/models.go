/*
 * Component: Manifest Models
 * Block-UUID: 95223007-1aff-47a7-b0a0-ac778a02a151
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the Go structs that map to the JSON manifest schema.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import "time"

// ManifestFile represents the root structure of the downloaded JSON manifest.
type ManifestFile struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Manifest      ManifestInfo  `json:"manifest"`
	Repositories  []Repository  `json:"repositories"`
	Branches      []Branch      `json:"branches"`
	Analyzers     []Analyzer    `json:"analyzers"`
	Fields        []Field       `json:"fields"`
	Data          []DataEntry   `json:"data"`
}

// ManifestInfo contains metadata about the manifest itself.
type ManifestInfo struct {
	Name         string   `json:"name"`
	DatabaseName string   `json:"database_name"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
}

// Repository represents a source repository reference.
type Repository struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// Branch represents a git branch reference.
type Branch struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// Analyzer represents an analyzer that extracted the metadata.
type Analyzer struct {
	Ref         string `json:"ref"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// Field represents a metadata field definition.
type Field struct {
	Ref          string `json:"ref"`
	AnalyzerRef  string `json:"analyzer_ref"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
}

// DataEntry represents a single file's metadata.
type DataEntry struct {
	RepoRef   string                 `json:"repo_ref"`
	BranchRef string                 `json:"branch_ref"`
	FilePath  string                 `json:"file_path"`
	Language  string                 `json:"language"`
	ChatID    int                    `json:"chat_id"`
	Fields    map[string]interface{} `json:"fields"`
}
