/**
 * Component: Query Models
 * Block-UUID: c7e07dd0-70b7-406f-8af3-b63c1f77db51
 * Parent-UUID: d2b3efb1-31c4-4fd0-b884-8ba29765cbb5
 * Version: 1.5.0
 * Description: Defines the Go structs for query operations, configuration, and list results. Added CoverageReport and supporting structs to implement the Phase 3 Scout Layer coverage analysis feature. Added InsightsReport, InsightsContext, FieldInsight, and InsightsSummary to support Phase 2 Scout Layer insights and reporting features.
 * Language: Go
 * Created-at: 2026-02-09T04:19:42.189Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0)
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

// QueryResponse wraps query results with context and coverage metadata.
type QueryResponse struct {
	Query   SimpleQuery    `json:"query"`   // The original query parameters
	Results []QueryResult  `json:"results"` // The matching files
	Summary QuerySummary   `json:"summary"` // Aggregated metadata and coverage
}

// QuerySummary provides high-level stats about the query execution.
type QuerySummary struct {
	TotalResults    int     `json:"total_results"`
	CoveragePercent float64 `json:"coverage_percent"`
	Confidence      string  `json:"confidence"`
	Database        string  `json:"database"`
}

// ListResult represents the result of a --list operation.
// It can represent a list of databases, fields within a database, or values within a field.
type ListResult struct {
	Level          string     `json:"level"` // "discovery", "database", "field", or "value"
	ActiveDatabase string     `json:"active_database,omitempty"`
	Databases      []ListItem `json:"databases,omitempty"`
	Fields         []ListItem `json:"fields,omitempty"`
	Values         []ListItem `json:"values,omitempty"`
	Hints          []string   `json:"hints,omitempty"`
}

// ListItem represents a single item in a list result.
type ListItem struct {
	Name        string `json:"name"`                  // The name of the item (db, field, or value)
	Description string `json:"description,omitempty"` // Optional description
	Source      string `json:"source,omitempty"`      // Optional source (e.g., physical filename)
	Type        string `json:"type,omitempty"`        // Optional type (for fields
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

// InsightsReport represents the full results of an insights analysis.
type InsightsReport struct {
	Context  InsightsContext            `json:"context"`
	Insights map[string][]FieldInsight  `json:"insights"` // Keyed by field name (e.g., "risk_level")
	Summary  InsightsSummary            `json:"summary"`
}

// InsightsContext provides metadata about the insights query execution.
type InsightsContext struct {
	Database        string       `json:"database"`
	Type            string       `json:"type"` // "insights"
	Limit           int          `json:"limit"`
	ScopeApplied    bool         `json:"scope_applied"`
	ScopeDefinition *ScopeConfig `json:"scope_definition,omitempty"`
	Timestamp       time.Time    `json:"timestamp"`
}

// FieldInsight represents a single value distribution for a specific field.
type FieldInsight struct {
	Value      string  `json:"value"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// InsightsSummary provides quantitative totals for the insights query.
type InsightsSummary struct {
	TotalFilesInScope             int            `json:"total_files_in_scope"`
	FilesWithMetadata             map[string]int `json:"files_with_metadata"`             // Keyed by field name
	FilesWithoutRequestedMetadata map[string]int `json:"files_without_requested_metadata"` // Keyed by field name
	NullValueCounts               map[string]int `json:"null_value_counts"`               // Keyed by field name
}
