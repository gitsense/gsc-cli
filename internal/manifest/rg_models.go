/*
 * Component: Ripgrep Models
 * Block-UUID: b808c94b-3dd8-48ae-b895-dd927251fc7a
 * Parent-UUID: bae19798-1185-46d5-86be-82b4fe5afcdf
 * Version: 1.1.0
 * Description: Defines the Go structs for ripgrep operations, including raw matches, enriched results, and the new FileMetadataResult struct for the YAML/JSON appendix.
 * Language: Go
 * Created-at: 2026-02-02T18:46:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package manifest

// RgMatch represents a raw match result from ripgrep's JSON output.
// This maps directly to the structure returned by `rg --json`.
type RgMatch struct {
	FilePath   string `json:"path"`       // The path to the file
	LineNumber int    `json:"line_number"` // The line number of the match
	LineText   string `json:"line"`       // The full text of the line
	MatchText  string `json:"submatches"` // The actual matched text (simplified)
	// Note: ripgrep JSON output is complex; this struct captures the essential fields.
	// A full parser would handle nested arrays for submatches.
}

// EnrichedMatch represents a ripgrep match that has been enriched with metadata from the database.
// This is the final output structure for the `gsc rg` command.
type EnrichedMatch struct {
	FilePath   string                 `json:"file_path"`   // The path to the file
	ChatID     int                    `json:"chat_id"`     // The GitSense Chat ID
	LineNumber int                    `json:"line_number"` // The line number of the match
	Match      string                 `json:"match"`       // The matched text or line
	Metadata   map[string]interface{} `json:"metadata"`   // Enriched metadata fields from the database
}

// RgOptions represents the configuration options for running ripgrep.
type RgOptions struct {
	Pattern       string `json:"pattern"`        // The search pattern
	Database      string `json:"database"`       // The database to use for enrichment
	ContextLines  int    `json:"context_lines"`  // Number of context lines to show
	CaseSensitive bool   `json:"case_sensitive"` // Whether the search is case-sensitive
	FileType      string `json:"file_type"`      // Optional file type filter (e.g., "js", "py")
}

// FileMetadataResult represents the metadata for a single file found in the search.
// It is used to generate the YAML/JSON appendix.
type FileMetadataResult struct {
	FilePath string                 `yaml:"file_path" json:"file_path"`
	ChatID   int                    `yaml:"chat_id,omitempty" json:"chat_id,omitempty"`
	Status   string                 `yaml:"status,omitempty" json:"status,omitempty"` // "found" or "not_found"
	Fields   map[string]interface{} `yaml:"fields,omitempty" json:"fields,omitempty"`
}
