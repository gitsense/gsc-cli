/*
 * Component: Scout Models
 * Block-UUID: 66d666f6-ab04-4c1e-ae7b-2c6c62de4e2e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Data structures for Scout feature (candidate discovery and verification)
 * Language: Go
 * Created-at: 2026-03-27T04:20:00.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0)
 */


package scout

import (
	"time"
)

// Session represents a Scout discovery/verification session
type Session struct {
	SessionID            string
	Intent               string
	WorkingDirectories   []WorkingDirectory
	ReferenceFiles       []ReferenceFile
	AutoReview           bool
	Status               string // "discovery", "discovery_complete", "verification", "verification_complete", "stopped", "error"
	StartedAt            time.Time
	CompletedAt          *time.Time
	Error                *string
}

// WorkingDirectory represents a directory in the contract being searched
type WorkingDirectory struct {
	ID   int
	Name string
	Path string
}

// ReferenceFile represents a user-provided reference file
type ReferenceFile struct {
	OriginalPath string
	LocalPath    string // Path in scout session directory
}

// Candidate represents a discovered file that may be relevant to the intent
type Candidate struct {
	WorkdirID      int                 `json:"workdir_id"`
	WorkdirName    string              `json:"workdir_name"`
	FilePath       string              `json:"file_path"`       // Relative to workdir
	AbsolutePath   string              `json:"absolute_path"`   // Computed for Turn 2
	Score          float64             `json:"score"`
	Reasoning      string              `json:"reasoning"`
	BrainMetadata  CandidateMetadata   `json:"metadata"`
	MatchedKeyword string              `json:"matched_keyword,omitempty"` // From gsc grep
}

// CandidateMetadata represents metadata from the Tiny Overview brain
type CandidateMetadata struct {
	Purpose        string   `json:"purpose"`
	FileExtension  string   `json:"file_extension"`
	Keywords       []string `json:"keywords"`
	ParentKeywords []string `json:"parent_keywords"`
}

// StatusData represents the complete status of a scout session
type StatusData struct {
	SessionID            string               `json:"session_id"`
	Status               string               `json:"status"` // "in_progress", "discovery_complete", "verification_complete", "stopped", "error"
	Phase                string               `json:"phase"`  // "discovery", "verification"
	StartedAt            time.Time            `json:"started_at"`
	CompletedAt          *time.Time           `json:"completed_at,omitempty"`
	ElapsedSeconds       int64                `json:"elapsed_seconds"`
	EstimatedRemaining   *int64               `json:"estimated_remaining,omitempty"`
	WorkingDirectories   []WorkingDirectory   `json:"working_directories"`
	Candidates           []Candidate          `json:"candidates"`
	TotalFound           int                  `json:"total_found"`
	ProcessInfo          ProcessInfo          `json:"process"`
	NextAction           *NextAction          `json:"next_action,omitempty"`
	Error                *string              `json:"error,omitempty"`
}

// ProcessInfo contains process-level information
type ProcessInfo struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Running bool   `json:"running"`
}

// NextAction indicates what should happen next (e.g., require user selection)
type NextAction struct {
	Type             string `json:"type"` // "require_user_selection", "auto_review_starting", "complete"
	Message          string `json:"message"`
	MaxSelectable    int    `json:"max_selectable,omitempty"`
	MinSelectable    int    `json:"min_selectable,omitempty"`
	RecommendedCount int    `json:"recommended_count,omitempty"`
}

// StreamEvent represents a JSONL event written to the log file
type StreamEvent struct {
	Timestamp string      `json:"timestamp"`
	Type      string      `json:"type"` // "init", "status", "candidates", "verified", "done", "error"
	Data      interface{} `json:"data"`
}

// InitEvent is the first event written when scout starts
type InitEvent struct {
	SessionID              string             `json:"session_id"`
	Intent                 string             `json:"intent"`
	WorkingDirectories     []WorkingDirectory `json:"working_directories"`
	ReferenceFiles         []ReferenceFile    `json:"reference_files"`
	Options                InitOptions        `json:"options"`
}

// InitOptions contains options passed to scout
type InitOptions struct {
	AutoReview bool `json:"auto_review"`
	Turn       int  `json:"turn"`
}

// StatusEvent indicates progress during discovery/verification
type StatusEvent struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Progress ProgressInfo `json:"progress"`
}

// ProgressInfo tracks progress within a phase
type ProgressInfo struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// CandidatesEvent is emitted when candidates are discovered
type CandidatesEvent struct {
	Phase      string      `json:"phase"`
	TotalFound int         `json:"total_found"`
	Candidates []Candidate `json:"candidates"`
}

// VerifiedEvent is emitted when candidates are re-scored after verification
type VerifiedEvent struct {
	Phase              string                    `json:"phase"`
	TotalVerified      int                       `json:"total_verified"`
	UpdatedCandidates  []VerificationUpdate      `json:"updated_candidates"`
}

// VerificationUpdate represents a candidate after re-verification
type VerificationUpdate struct {
	FilePath       string  `json:"file_path"`
	WorkdirID      int     `json:"workdir_id"`
	OriginalScore  float64 `json:"original_score"`
	VerifiedScore  float64 `json:"verified_score"`
	Reason         string  `json:"reason"`
}

// DoneEvent indicates completion
type DoneEvent struct {
	Status           string                    `json:"status"` // "success", "stopped", "error"
	TotalCandidates  int                       `json:"total_candidates"`
	PhaseCompleted   string                    `json:"phase_completed"`
	Summary          CompletionSummary         `json:"summary"`
}

// CompletionSummary provides statistics about the completed scout session
type CompletionSummary struct {
	FilesFound                 int `json:"files_found"`
	Coverage                   string `json:"coverage"`
	WorkingDirectoriesSearched int `json:"working_directories_searched"`
	ReferenceFilesUsed         int `json:"reference_files_used"`
}

// ErrorEvent indicates an error occurred
type ErrorEvent struct {
	Phase       string `json:"phase"`
	ErrorCode   string `json:"error_code"`
	Message     string `json:"message"`
	Details     string `json:"details"`
}

// SelectedCandidates represents user-selected candidates for manual review
type SelectedCandidates struct {
	Selected []SelectedCandidate `json:"selected"`
}

// SelectedCandidate is a candidate selected for review
type SelectedCandidate struct {
	FilePath    string  `json:"file_path"`
	WorkdirID   int     `json:"workdir_id"`
	OriginalScore float64 `json:"original_score"`
}
