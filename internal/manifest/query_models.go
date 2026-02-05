/*
 * Component: Query Models
 * Block-UUID: ab34a4e7-fd6b-448f-9ea1-99b421d73a1d
 * Parent-UUID: 854ea0c7-91a2-44dc-b3ab-bfd0f3469775
 * Version: 1.1.0
 * Description: Defines the Go structs for query operations, configuration, and list results. Added CoverageReport and supporting structs to implement the Phase 3 Scout Layer coverage analysis feature.
 * Language: Go
 * Created-at: 2026-02-02T18:45:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package manifest

import "time"

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

// QueryAlias represents a saved query alias.
type QueryAlias struct {
	Database string `json:"database"` // The database to query
	Field    string `json:"field"`    // The field to match
	Value    string `json:"value"`    // The value to match
}

// CoverageReport represents the full results of a coverage analysis.
type CoverageReport struct {
	Timestamp       time.Time                   `json:"timestamp"`
	ActiveProfile   string                      `json:"active_profile"`
	ScopeDefinition *ScopeConfig                `json:"scope_definition"`
	Totals          CoverageTotals              `json:"totals"`
	Percentages     CoveragePercentages         `json:"percentages"`
	ByLanguage      map[string]LanguageCoverage `json:"by_language"`
	BlindSpots      BlindSpots                  `json:"blind_spots"`
	AnalysisStatus  string                      `json:"analysis_status"`
	Recommendations []string                    `json:"recommendations"`
}

// CoverageTotals contains the raw file counts for the report.
type CoverageTotals struct {
	TrackedFiles    int `json:"tracked_files"`
	InScopeFiles    int `json:"in_scope_files"`
	AnalyzedFiles   int `json:"analyzed_files"`
	OutOfScopeFiles int `json:"out_of_scope_files"`
	UntrackedFiles  int `json:"untracked_files"`
}

// CoveragePercentages contains the calculated coverage ratios.
type CoveragePercentages struct {
	FocusCoverage float64 `json:"focus_coverage"`
	TotalCoverage float64 `json:"total_coverage"`
}

// LanguageCoverage contains coverage stats for a specific programming language.
type LanguageCoverage struct {
	Total    int     `json:"total"`
	Analyzed int     `json:"analyzed"`
	Percent  float64 `json:"percent"`
}

// BlindSpots identifies areas of the codebase with low analysis coverage.
type BlindSpots struct {
	Directories []DirectoryBlindSpot `json:"directories"`
	Files       []string             `json:"files,omitempty"`
}

// DirectoryBlindSpot represents a directory with unanalyzed files.
type DirectoryBlindSpot struct {
	Path          string  `json:"path"`
	TotalFiles    int     `json:"total_files"`
	AnalyzedFiles int     `json:"analyzed_files"`
	Percent       float64 `json:"percent"`
}
