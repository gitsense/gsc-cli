/**
 * Component: Search Intelligence Models
 * Block-UUID: 3e1cbfe0-bd9c-4265-a7e8-83894cb9dcfa
 * Parent-UUID: 37e27d9f-c464-4d09-8668-7d6318c6cd65
 * Version: 2.1.0
 * Description: Defines the structured JSON response for gsc grep. Updated to support grouped file results, tool metadata, system info, truncation signals, and filter structures.
 * Language: Go
 * Created-at: 2026-02-03T19:44:11.581Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.1.0)
 */


package search

import "time"

// GrepResponse is the top-level JSON object returned to the agent.
type GrepResponse struct {
	Context  QueryContext  `json:"context"`
	Summary  GrepSummary   `json:"summary"`
	Files    []FileResult  `json:"files,omitempty"` // Only if NOT --summary
}

// QueryContext provides metadata about the search execution.
type QueryContext struct {
	Pattern     string       `json:"pattern"`
	Database    string       `json:"database"`
	Mode        string       `json:"mode"` // "summary" or "full"
	Tool        ToolInfo     `json:"tool"`
	SearchScope SearchScope  `json:"search_scope"`
	System      SystemInfo   `json:"system"`
	Repository  RepositoryInfo `json:"repository"`
	Timestamp   time.Time    `json:"timestamp"`
	Filters     []string     `json:"filters,omitempty"` // List of filter strings applied
}

// ToolInfo holds details about the search tool used.
type ToolInfo struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Arguments []string `json:"arguments"`
	TotalMs   int      `json:"total_ms"`
}

// SearchScope describes the constraints applied to the search.
type SearchScope struct {
	FileType      string `json:"file_type"`
	ContextLines  int    `json:"context_lines"`
	CaseSensitive bool   `json:"case_sensitive"`
}

// SystemInfo holds details about the execution environment.
type SystemInfo struct {
	ProjectRoot string `json:"project_root"`
	OS          string `json:"os"`
}

// RepositoryInfo holds details about the git repository.
type RepositoryInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Remote string `json:"remote"`
}

// GrepSummary provides quantitative "Hot/Cold" signals and file list.
type GrepSummary struct {
	TotalMatches    int                       `json:"total_matches"`
	TotalFiles      int                       `json:"total_files"`
	AnalyzedFiles   int                       `json:"analyzed_files"`
	IsTruncated     bool                      `json:"is_truncated"`
	FieldDistribution map[string]map[string]int `json:"field_distribution"`
	Files           []FileSummary             `json:"files"`
}

// FileSummary represents a file in the summary list.
type FileSummary struct {
	FilePath   string                 `json:"file_path"`
	ChatID     *int                   `json:"chat_id,omitempty"`     // Omitted if not analyzed
	Analyzed   bool                   `json:"analyzed"`
	MatchCount int                    `json:"match_count"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // Omitted if not analyzed
}

// FileResult represents a file with full match details.
type FileResult struct {
	FilePath   string                 `json:"file_path"`
	ChatID     *int                   `json:"chat_id,omitempty"`     // Omitted if not analyzed
	Analyzed   bool                   `json:"analyzed"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // Omitted if not analyzed
	Matches    []MatchDetail          `json:"matches"`
}

// MatchDetail represents a single code match with context.
type MatchDetail struct {
	LineNumber    int      `json:"line_number"`
	LineText      string   `json:"line_text"`
	ContextBefore []string `json:"context_before"`
	ContextAfter  []string `json:"context_after"`
}

// MatchResult represents an intermediate enriched match before grouping by file.
// It is used internally by the enricher and aggregator.
type MatchResult struct {
	FilePath      string                 `json:"file_path"`
	LineNumber    int                    `json:"line_number"`
	LineText      string                 `json:"line_text"`
	ContextBefore []string               `json:"context_before"`
	ContextAfter  []string               `json:"context_after"`
	ChatID        int                    `json:"chat_id"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// FilterCondition represents a single parsed filter condition.
// Example: Field="topic", Operator="=", Value="security"
type FilterCondition struct {
	Field    string
	Operator string
	Value    string
}

// SearchRecord represents the data stored in the search history database.
type SearchRecord struct {
	Timestamp      time.Time
	Pattern        string
	ToolName       string
	ToolVersion    string
	DurationMs     int
	TotalMatches   int
	TotalFiles     int
	AnalyzedFiles  int
	FiltersUsed    string // JSON string of filters applied
	DatabaseName   string
	CaseSensitive  bool
	FileFilters    string // JSON string of file path patterns
	AnalyzedFilter string // "true", "false", or "all"
}
