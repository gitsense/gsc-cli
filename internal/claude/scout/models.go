/**
 * Component: Scout Models
 * Block-UUID: fead5c6c-a005-426e-9b7c-917cb2342cbe
 * Parent-UUID: a4a0f633-d391-4f87-bf94-35d18198472c
 * Version: 2.0.0
 * Description: Data structures for Scout feature (candidate discovery and verification). Updated to support rich verification format with critical missing files, keyword effectiveness assessment, and actionable recommendations.
 * Language: Go
 * Created-at: 2026-04-13T04:40:10.160Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.6), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v2.0.0)
 */


package scout

import (
	"time"
)

// Session represents a Scout discovery/verification session
type Session struct {
	SessionDir            string              `json:"session_dir"`
	SessionID             string              `json:"session_id"`
	Intent                string              `json:"intent"`
	Model                 string              `json:"model"`
	WorkingDirectories    []WorkingDirectory   `json:"working_directories"`
	ReferenceFilesContext []ReferenceFileContext `json:"reference_files_context"`
	AutoReview            bool                `json:"auto_review"`
	Status                string              `json:"status"` // "discovery", "discovery_complete", "verification", "verification_complete", "stopped", "error"
	StartedAt             time.Time           `json:"started_at"`
	CompletedAt           *time.Time          `json:"completed_at,omitempty"`
	Error                 *string             `json:"error,omitempty"`
	WatcherPID            *int `json:"watcher_pid,omitempty"`
	Turns                 []TurnState         `json:"turns"`
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

// ReferenceFileContext represents a reference file from the NDJSON input with chat metadata
type ReferenceFileContext struct {
	ChatID       int    `json:"chat_id"`
	Repository   string `json:"repository"`
	RelativePath string `json:"relative_path"`
	Content      string `json:"content"`
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

// Usage represents token usage metrics from Claude
type Usage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens    int `json:"cache_read_input_tokens"`
}

// CandidateMetadata represents metadata from the Code Intent brain
type CandidateMetadata struct {
	Purpose        string   `json:"purpose"`
	FileExtension  string   `json:"file_extension"`
	Keywords       []string `json:"keywords"`
	ParentKeywords []string `json:"parent_keywords"`
}

// QuickCandidate represents a lightweight candidate for quick status display
type QuickCandidate struct {
	WorkdirID   int     `json:"workdir_id"`
	WorkdirName string  `json:"workdir_name"`
	FilePath    string  `json:"file_path"`
	Score       float64 `json:"score"`
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
	ReferenceFilesContext []ReferenceFileContext `json:"reference_files_context"`
	Candidates           []Candidate          `json:"candidates"`
	TotalFound           int                  `json:"total_found"`
	ProcessInfo          ProcessInfo          `json:"process"`
	NextAction           *NextAction          `json:"next_action,omitempty"`
	Error                *string              `json:"error,omitempty"`
	Usage                *Usage               `json:"usage,omitempty"`
	Cost                 *float64             `json:"cost,omitempty"`
	Duration             *int64               `json:"duration,omitempty"`
	ClaudeSessionID      *string              `json:"claude_session_id,omitempty"`
	ShutdownInitiated    bool                 `json:"shutdown_initiated,omitempty"`
	ShutdownCompleted    bool                 `json:"shutdown_completed,omitempty"`
	WatcherPID           *int                 `json:"watcher_pid,omitempty"`
	SessionDir           string               `json:"session_dir,omitempty"`
	CurrentLogPath       string               `json:"current_log_path,omitempty"`
	Turns                []TurnState          `json:"turns"`
	CurrentTurn          int                  `json:"current_turn"`
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
	ReferenceFilesContext  []ReferenceFileContext `json:"reference_files_context"`
	Options                InitOptions        `json:"options"`
}

// InitOptions contains options passed to scout
type InitOptions struct {
	AutoReview bool `json:"auto_review"`
	Turn       int  `json:"turn"`
	Model      string `json:"model"`
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

// VerificationUpdate represents a candidate after re-verification (simple format for backward compatibility)
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
	Usage            *Usage                    `json:"usage,omitempty"`
	Cost             *float64                  `json:"cost,omitempty"`
	Duration         *int64                    `json:"duration,omitempty"`
	ClaudeSessionID  *string                   `json:"claude_session_id,omitempty"`
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
	ShutdownInitiated bool `json:"shutdown_initiated,omitempty"`
	ShutdownCompleted bool `json:"shutdown_completed,omitempty"`
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

// TurnState represents the state of a single turn in a Scout session
type TurnState struct {
	TurnNumber           int                 `json:"turn_number"`
	TurnType             string              `json:"turn_type"` // "discovery" or "verification"
	Status               string              `json:"status"` // "pending", "running", "complete", "error"
	StartedAt            time.Time           `json:"started_at"`
	CompletedAt          *time.Time          `json:"completed_at,omitempty"`
	LogPath              string              `json:"log_path"`
	ProcessInfo          ProcessInfo         `json:"process_info"`
	Candidates           []QuickCandidate    `json:"candidates,omitempty"`
	TotalFound           int                 `json:"total_found"`
	Usage                *Usage              `json:"usage,omitempty"`
	Results              *TurnResults        `json:"results,omitempty"`
	Cost                 *float64            `json:"cost,omitempty"`
	Duration             *int64              `json:"duration,omitempty"`
	ClaudeSessionID      *string             `json:"claude_session_id,omitempty"`
	Error                *string             `json:"error,omitempty"`
}

// TurnResults contains full results with reasoning and metadata
type TurnResults struct {
	Candidates           []Candidate         `json:"candidates"`
	DiscoveryLog         *DiscoveryLog       `json:"discovery_log,omitempty"`
	VerificationSummary  *VerificationSummary `json:"verification_summary,omitempty"`
	Coverage             string              `json:"coverage,omitempty"`
	Duration             *int64              `json:"duration,omitempty"`
	Cost                 *float64            `json:"cost,omitempty"`
	Usage                *Usage              `json:"usage,omitempty"`
}

// DiscoveryLog contains the discovery methodology and pivot checks
type DiscoveryLog struct {
	IntentKeywords       []string            `json:"intent_keywords"`
	PivotChecks          []string            `json:"pivot_checks"`
	Methodology          string              `json:"methodology"`
	TotalCandidatesFound int                 `json:"total_candidates_found"`
	TopCandidatesReturned int                 `json:"top_candidates_returned"`
}

// VerificationLog contains the verification methodology and findings
type VerificationLog struct {
	DiscoveryReviewed      []string            `json:"discovery_reviewed"`       // Files from discovery that were reviewed
	CriticalFindings       []string            `json:"critical_findings"`        // Key discoveries (e.g., missed files)
	MissingFilesIdentified []MissingFile       `json:"missing_files_identified"` // Files discovery missed
	KeywordAssessment      KeywordAssessment   `json:"keyword_assessment"`       // Effectiveness of discovery keywords
	VerificationMethod     string              `json:"verification_method"`      // How verification was performed
	TotalVerified          int                 `json:"total_verified"`
	Confidence             string              `json:"confidence"`               // Confidence level in results
}

// MissingFile represents a file that was missed by discovery but found during verification
type MissingFile struct {
	FilePath    string `json:"file_path"`
	Reason      string `json:"reason"`
	Evidence    string `json:"evidence"`
	Relevance   string `json:"relevance"`
}

// KeywordAssessment contains analysis of keyword effectiveness from discovery
type KeywordAssessment struct {
	DiscoveryKeywords []string            `json:"discovery_keywords"`
	Effectiveness     map[string]KeywordEffectiveness `json:"effectiveness"`
	NewKeywords       []string            `json:"new_keywords_discovered"`
	Recommendations   []string            `json:"recommendations"`
}

// KeywordEffectiveness describes how well a keyword performed
type KeywordEffectiveness struct {
	Rating     string   `json:"rating"`     // High/Medium/Low
	Explanation string  `json:"explanation"`
	Matches    []string `json:"matches"`
}

// VerificationSummary contains verification phase statistics (rich format)
type VerificationSummary struct {
	SessionIntent              string                      `json:"session_intent"`
	TurnNumber                 int                         `json:"turn_number"`
	TotalCandidatesReviewed    int                         `json:"total_candidates_reviewed"`
	VerifiedCandidatesCount    int                         `json:"verified_candidates_count"`
	CriticalFinding            string                      `json:"critical_finding"`
	TotalVerified              int                         `json:"total_verified"`
	CandidatesPromoted         int                         `json:"candidates_promoted"`
	CandidatesDemoted          int                         `json:"candidates_demoted"`
	CandidatesRemoved          int                         `json:"candidates_removed"`
	AverageVerifiedScore       float64                     `json:"average_verified_score"`
	TopCandidatesCount         int                         `json:"top_candidates_count"`
	Duration                   *int64                      `json:"duration,omitempty"`
	Cost                       *float64                    `json:"cost,omitempty"`
	Usage                      *Usage                      `json:"usage,omitempty"`
	VerificationLog            *VerificationLog            `json:"verification_log,omitempty"`
}

// RichVerifiedCandidate represents a verified candidate with detailed analysis
type RichVerifiedCandidate struct {
	FilePath         string              `json:"file_path"`
	OriginalScore    float64             `json:"original_score"`
	VerifiedScore    float64             `json:"verified_score"`
	Relevance        string              `json:"relevance"`
	Reasoning        string              `json:"reasoning"`
	CodeVerification CodeVerification    `json:"code_verification"`
	ActionRequired    string              `json:"action_required"`
}

// CodeVerification contains detailed code analysis
type CodeVerification struct {
	ConfirmedPatterns   []string `json:"confirmed_patterns"`
	MissingPatterns     []string `json:"missing_patterns,omitempty"`
	ImplementationDetails string  `json:"implementation_details"`
	Issues              []string `json:"issues,omitempty"`
}

// CriticalMissingCandidate represents a file that discovery missed
type CriticalMissingCandidate struct {
	FilePath         string            `json:"file_path"`
	Score            float64           `json:"score"`
	Relevance        string            `json:"relevance"`
	Reasoning        string            `json:"reasoning"`
	CodeVerification MissingFileCodeVerification `json:"code_verification"`
	ActionRequired    string            `json:"action_required"`
}

// MissingFileCodeVerification contains code verification for missing files
type MissingFileCodeVerification struct {
	ConfirmedPattern string `json:"confirmed_pattern"`
}

// RichKeywordAssessment contains detailed keyword effectiveness analysis
type RichKeywordAssessment struct {
	DiscoveryIntentKeywords []string                        `json:"discovery_intent_keywords"`
	KeywordEffectiveness     map[string]KeywordEffectivenessDetail `json:"keyword_effectiveness"`
	NewKeywordsDiscovered    map[string]NewKeywordDetail    `json:"new_keywords_discovered_in_code"`
	KeywordRecommendations   KeywordRecommendations          `json:"keyword_recommendations"`
}

// KeywordEffectivenessDetail contains detailed effectiveness analysis for a keyword
type KeywordEffectivenessDetail struct {
	Effectiveness    string   `json:"effectiveness"`    // HIGH/MEDIUM/LOW
	Explanation      string   `json:"explanation"`
	MatchedFiles     int      `json:"matched_files"`
	ExampleMatches   []string `json:"example_matches"`
	Issue            string   `json:"issue,omitempty"`
}

// NewKeywordDetail contains details about a newly discovered keyword
type NewKeywordDetail struct {
	FoundIn   string `json:"found_in"`
	Pattern   string `json:"pattern"`
	Relevance string `json:"relevance"`
}

// KeywordRecommendations contains recommendations for improving keyword strategy
type KeywordRecommendations struct {
	ShouldAdd              []string `json:"should_add"`
	ShouldRefine           []string `json:"should_refine"`
	FutureDiscoveryStrategy string   `json:"future_discovery_strategy"`
}

// SummaryAndRecommendations contains overall assessment and actionable recommendations
type SummaryAndRecommendations struct {
	FilesToModify              []FileModification `json:"files_to_modify_to_change_default_expiration"`
	FalsePositivesIdentified   []string           `json:"false_positives_identified"`
	DiscoveryQualityAssessment string             `json:"discovery_quality_assessment"`
	Verdict                    string             `json:"verdict"`
}

// FileModification represents a file that needs to be modified
type FileModification struct {
	Priority string `json:"priority"` // PRIMARY/SECONDARY/TERTIARY
	File     string `json:"file"`
	Line     int    `json:"line"`
	Change   string `json:"change"`
	Reason   string `json:"reason"`
}
