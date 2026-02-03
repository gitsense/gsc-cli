/*
 * Component: Search Intelligence Models
 * Block-UUID: 18fa7a4e-db31-41ae-b43c-01b79e5351cc
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the structured JSON response for gsc grep, focusing on AI-consumable summary and insights for steering and cost savings.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import "time"

// GrepResponse is the top-level JSON object returned to the agent.
type GrepResponse struct {
	Context  QueryContext  `json:"context"`
	Summary  *GrepSummary  `json:"summary,omitempty"` // Only if --summary
	Matches  []MatchResult `json:"matches,omitempty"` // Only if NOT --summary
}

// QueryContext provides metadata about the search execution.
type QueryContext struct {
	Pattern    string          `json:"pattern"`
	Database   string          `json:"database"`
	Profile    string          `json:"profile,omitempty"`
	Repository RepositoryInfo  `json:"repository"`
	Timestamp  time.Time       `json:"timestamp"`
}

// RepositoryInfo holds details about the git repository.
type RepositoryInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Remote string `json:"remote"`
}

// GrepSummary provides quantitative "Hot/Cold" signals.
type GrepSummary struct {
	TotalMatches    int                       `json:"total_matches"`
	TotalFiles      int                       `json:"total_files"`
	AnalyzedFiles   int                       `json:"analyzed_files"`
	UnanalyzedFiles int                       `json:"unanalyzed_files"`
	// FieldDistribution maps FieldName -> Value -> Frequency
	// Example: "risk_level" -> {"high": 10, "low": 440}
	FieldDistribution map[string]map[string]int `json:"field_distribution"`
}

// MatchResult represents a single code match with context and metadata.
type MatchResult struct {
	FilePath      string                 `json:"file_path"`
	ChatID        int                    `json:"chat_id"`
	LineNumber    int                    `json:"line_number"`
	LineText      string                 `json:"line_text"`
	ContextBefore []string               `json:"context_before"`
	ContextAfter  []string               `json:"context_after"`
	Metadata      map[string]interface{} `json:"metadata"`
}
