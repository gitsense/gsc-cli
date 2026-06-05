/**
 * Component: Experts Models
 * Block-UUID: 56273ae2-b831-489e-8724-1a3550ced6ca
 * Parent-UUID: d2f01451-8279-4724-b35e-89836639b991
 * Version: 1.3.0
 * Description: Core data structures for the gsc experts command. Renamed AnalyzerSummary to BrainSummary and ExpertsContext.Analyzers to ExpertsContext.Brains to align with Brain terminology.
 * Language: Go
 * Created-at: 2026-05-01T16:25:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package experts

import "time"

// ExpertsConfig defines the configuration for loading expert context.
type ExpertsConfig struct {
	Databases []string `json:"databases"`  // Specific brain databases to include. Empty = all active.
	RepoPath  string   `json:"repo_path"`  // The root path of the repository.
	UserLevel string   `json:"user_level"` // Persona: "new" (Guide), "author" (Specialist), "user" (Consultant).
}

// FieldSummary represents a metadata field definition within a brain.
type FieldSummary struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// BrainSummary represents a summary of a single Brain database.
type BrainSummary struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	EntryCount  int            `json:"entry_count"`
	Fields      []FieldSummary `json:"fields"`
}

// ExpertsContext represents the complete context generated for the AI agent.
type ExpertsContext struct {
	GeneratedAt time.Time       `json:"generated_at"`
	RepoPath    string          `json:"repo_path"`
	UserLevel   string          `json:"user_level"`
	Brains      []BrainSummary  `json:"brains"`
}
