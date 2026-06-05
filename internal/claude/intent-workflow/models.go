/**
 * Component: Intent Workflow Models
 * Block-UUID: 5d15475e-413c-41f1-9ff9-f16e2c393b9f
 * Parent-UUID: 16d320cd-938d-41ed-a2f5-4edb12c4506e
 * Version: 2.29.0
 * Description: Core data structures and types for intent workflow sessions including session state, turn management, candidates, and event models. Added SuccinctNaturalLanguageResponse to DiscoveryResult to support AI-generated natural language summaries in discovery responses. Added EnableCodeProvenance to Session struct to persist provenance state across turns. Added OldVersion and NewVersion to GSCFileData and ChangelogEntry to support semantic version tracking in the provenance ledger and enriched metadata files. Added GitContext, Environment, and OtherChanges to ChangeResult for comprehensive audit trail and attribution. Added "skipped" status to TurnState to support skipping discovery turns when using --skip-discovery flag. Added BrainEntry, BrainEffectiveness structs and DiscoveryMode to DiscoveryResult for hybrid discovery strategy. Added DisableExperts to Session struct to support --disable-experts flag for forcing generic discovery mode.
 * Language: Go
 * Created-at: 2026-04-30T12:34:10.812Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.6), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0), GLM-4.7 (v1.17.0), GLM-4.7 (v1.18.0), GLM-4.7 (v1.19.0), GLM-4.7 (v2.20.0), Gemini 3 Flash (v2.21.0), Gemini 3 Flash (v2.22.0), GLM-4.7 (v2.23.0), GLM-4.7 (v2.24.0), GLM-4.7 (v2.25.0), GLM-4.7 (v2.26.0), GLM-4.7 (v2.27.0), GLM-4.7 (v2.28.0), GLM-4.7 (v2.29.0)
 */


package intent_workflow

import (
	"time"
)

// ErrorDetails provides structured error information with file-level details
type ErrorDetails struct {
	ErrorCode  string      `json:"error_code"`  // Machine-readable error code
	Message    string      `json:"message"`     // Human-readable error message
	ErrorFiles []ErrorFile `json:"error_files"` // File-level error details
}

// ErrorFile represents a file-level error with context
type ErrorFile struct {
	FilePath     string `json:"file_path"`     // Relative path to the file
	WorkingDir   string `json:"working_dir"`   // Working directory containing the file
	Reason       string `json:"reason"`        // Human-readable reason for the error
	ExpectedPath string `json:"expected_path"` // Expected path (e.g., metadata file location)
	Resumable    bool   `json:"resumable"`     // Can this error be fixed with resume?
}

// Session represents an agent discovery/validation/change/etc. session
type Session struct {
	SessionDir            string                 `json:"session_dir"`
	SessionID             string                 `json:"session_id"`
	Intent                string                 `json:"intent"`
	Model                 string                 `json:"model"`
	WorkingDirectories    []WorkingDirectory     `json:"working_directories"`
	ReferenceFilesContext []ReferenceFileContext `json:"reference_files_context"`
	AutoReview            bool                   `json:"auto_review"`
	EnableCodeProvenance  bool                   `json:"enable_code_provenance"` // Persists provenance state
	DisableExperts        bool                   `json:"disable_experts,omitempty"` // Force generic mode; skip experts context even if file exists
	Status                string                 `json:"status"` // "discovery", "discovery_complete", "validation", "validation_complete", "change", "change_post_processing", "change_complete", "stopped", "error", "failed_metadata"
	StartedAt             time.Time              `json:"started_at"`
	CompletedAt           *time.Time             `json:"completed_at,omitempty"`
	Error                 *string                `json:"error,omitempty"`
	ErrorDetails          *ErrorDetails          `json:"error_details,omitempty"`
	WatcherPID            *int 					 `json:"watcher_pid,omitempty"`
	Stopped               bool                   `json:"stopped,omitempty"`
	Turns                 []TurnState            `json:"turns"`
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
	LocalPath    string // Path in intent workflow session directory
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
	CodeValidation *CodeValidation     `json:"code_validation,omitempty"` // Code inspection results
}

// Usage represents token usage metrics from Claude
type Usage struct {
	InputTokens        int  `json:"input_tokens"`
	OutputTokens       int  `json:"output_tokens"`
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

// CodeValidation contains detailed code analysis for a candidate
type CodeValidation struct {
	ConfirmedPatterns     []string `json:"confirmed_patterns,omitempty"`
	ImplementationDetails string   `json:"implementation_details,omitempty"`
}

// QuickCandidate represents a lightweight candidate for quick status display
type QuickCandidate struct {
	WorkdirID   int     `json:"workdir_id"`
	WorkdirName string  `json:"workdir_name"`
	FilePath    string  `json:"file_path"`
	Score       float64 `json:"score"`
}

// StatusData represents the complete status of an intent workflow session
type StatusData struct {
	SessionID            string               `json:"session_id"`
	Status               string               `json:"status"` // "in_progress", "discovery_complete", "validation_complete", "stopped", "error", "failed_metadata"
	Phase                string               `json:"phase"`  // "discovery", "validation"
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
	ErrorDetails         *ErrorDetails       `json:"error_details,omitempty"`
	CurrentLogPath       string               `json:"current_log_path,omitempty"`
	Turns                []TurnState          `json:"turns"`
	CurrentTurn          int                  `json:"current_turn"`
	CorrectionAttempts   int                  `json:"correction_attempts,omitempty"`
	CorrectionModel      string               `json:"correction_model,omitempty"`
	CorrectionStatus     string               `json:"correction_status,omitempty"`
	CorrectionCost       *float64             `json:"correction_cost,omitempty"`
	TotalCost            *float64             `json:"total_cost,omitempty"`
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
	Type      string      `json:"type"` // "init", "status", "candidates", "validated", "done", "error"
	Data      interface{} `json:"data"`
}

// InitEvent is the first event written when agent starts
type InitEvent struct {
	SessionID              string                 `json:"session_id"`
	Intent                 string                 `json:"intent"`
	WorkingDirectories     []WorkingDirectory     `json:"working_directories"`
	ReferenceFilesContext  []ReferenceFileContext `json:"reference_files_context"`
	Options                InitOptions            `json:"options"`
}

// InitOptions contains options passed to agent
type InitOptions struct {
	AutoReview bool   `json:"auto_review"`
	Turn       int    `json:"turn"`
	Model      string `json:"model"`
}

// StatusEvent indicates progress during discovery/validation
type StatusEvent struct {
	Phase   string        `json:"phase"`
	Message string        `json:"message"`
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

// ValidatedEvent is emitted when candidates are re-scored after validation
type ValidatedEvent struct {
	Phase              string             `json:"phase"`
	TotalValidated      int               `json:"total_validated"`
	UpdatedCandidates  []ValidationUpdate `json:"updated_candidates"`
}

// ValidationUpdate represents a candidate after re-validation (simple format for backward compatibility)
type ValidationUpdate struct {
	FilePath       string   `json:"file_path"`
	WorkdirID      int      `json:"workdir_id"`
	OriginalScore  float64  `json:"original_score"`
	ValidatedScore  float64 `json:"validated_score"`
	Reason         string   `json:"reason"`
}

// DoneEvent indicates completion
type DoneEvent struct {
	Status           string            `json:"status"` // "success", "stopped", "error"
	TotalCandidates  int               `json:"total_candidates"`
	PhaseCompleted   string            `json:"phase_completed"`
	Summary          CompletionSummary `json:"summary"`
	Usage            *Usage            `json:"usage,omitempty"`
	Cost             *float64          `json:"cost,omitempty"`
	Duration         *int64            `json:"duration,omitempty"`
	ClaudeSessionID  *string           `json:"claude_session_id,omitempty"`
}

// CompletionSummary provides statistics about the completed intent workflow session
type CompletionSummary struct {
	FilesFound                 int    `json:"files_found"`
	Coverage                   string `json:"coverage"`
	WorkingDirectoriesSearched int    `json:"working_directories_searched"`
	ReferenceFilesUsed         int    `json:"reference_files_used"`
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
}

// TurnState represents the state of a single turn in an Intent workflow session
type TurnState struct {
	TurnNumber           int                 `json:"turn_number"`
	TurnType             string              `json:"turn_type"` // "discovery" or "validation"
	Status               string              `json:"status"` // "pending", "running", "complete", "error", "failed_metadata", "skipped"
	StartedAt            time.Time           `json:"started_at"`
	CompletedAt          *time.Time          `json:"completed_at,omitempty"`
	LogPath              string              `json:"log_path"`
	ProcessInfo          ProcessInfo         `json:"process_info"`
	Usage                *Usage              `json:"usage,omitempty"`
	Result               *TurnResult         `json:"result,omitempty"`
	Cost                 *float64            `json:"cost,omitempty"`
	Duration             *int64              `json:"duration,omitempty"`
	ClaudeSessionID      *string             `json:"claude_session_id,omitempty"`
	Error                *string             `json:"error,omitempty"`
	CorrectionAttempts   int                 `json:"correction_attempts,omitempty"`
	CorrectionModel      string              `json:"correction_model,omitempty"`
	CorrectionStatus     string              `json:"correction_status,omitempty"`
	CorrectionCost       *float64            `json:"correction_cost,omitempty"`
	TotalCost            *float64            `json:"total_cost,omitempty"`
	ErrorDetails         *ErrorDetails       `json:"error_details,omitempty"`
}

// TurnResult wraps the specific result type for a turn.
// Only one of Discovery or Change will be populated.
type TurnResult struct {
	RawJSON    string           `json:"raw_json,omitempty"`
	ParseError string           `json:"parse_error,omitempty"`
	Discovery  *DiscoveryResult `json:"discovery,omitempty"`
	Change     *ChangeResult    `json:"change,omitempty"`
}

// BrainEntry represents effectiveness data for a single brain database
type BrainEntry struct {
	Name          string   `json:"name"`           // brain database name
	Score         float64  `json:"score"`          // 0.0-1.0 effectiveness score
	FieldsUsed    []string `json:"fields_used"`    // fields that were useful
	FieldsMissing []string `json:"fields_missing"` // fields that were missing
	Feedback      string   `json:"feedback"`       // qualitative feedback
}

// BrainEffectiveness represents overall brain effectiveness for a discovery turn
type BrainEffectiveness struct {
	OverallScore float64      `json:"overall_score"` // 0.0-1.0 overall effectiveness
	Brains       []BrainEntry `json:"brains"`       // per-brain effectiveness data
}

// DiscoveryResult contains the output of a discovery or validation turn.
type DiscoveryResult struct {
	Candidates                   []Candidate         `json:"candidates"`
	TotalFound                   int                 `json:"total_found"`
	MissingFiles                 []MissingFile       `json:"missing_files,omitempty"`
	KeywordAssessment            *KeywordAssessment  `json:"keyword_assessment,omitempty"`
	DiscoveryLog                 *DiscoveryLog       `json:"discovery_log,omitempty"`
	ValidationSummary            *ValidationSummary  `json:"validation_summary,omitempty"`
	Coverage                     string              `json:"coverage,omitempty"`
	DiscoveryMode                string              `json:"discovery_mode"`     // "experts" or "generic"
	BrainEffectiveness           *BrainEffectiveness `json:"brain_effectiveness,omitempty"` // nil when DiscoveryMode == "generic"
	SuccinctNaturalLanguageResponse string           `json:"succinct_natural_language_response,omitempty"` // Optional natural language summary
}

// ChangeResult contains the output of a change turn.
type ChangeResult struct {
	ChangeRequest       string               `json:"change_request"`
	FilesModified       FilesModifiedSummary `json:"files_modified"`
	OtherChanges        FilesModifiedSummary `json:"other_changes"`        // Files changed by user or other processes
	DiscoveryGap        DiscoveryGap         `json:"discovery_gap"`
	Changelog           []ChangelogEntry     `json:"changelog"` // Aggregated changelog from .change-meta.json files
	Notes               string               `json:"notes"`
	Errors              string               `json:"errors"`
	GitContexts         map[string]GitContext `json:"git_contexts"`       // Per-workdir git context (keyed by absolute path)
	Environment         Environment          `json:"environment"`         // Environment metadata
}

// GitContext represents git repository state at the time of a change turn.
type GitContext struct {
	BranchName    string `json:"branch_name"`     // Current branch name (or "HEAD" if detached)
	HeadSHA       string `json:"head_sha"`        // SHA of the HEAD commit
	HeadMessage   string `json:"head_message"`    // First line of HEAD commit message
	HeadAuthor    string `json:"head_author"`     // Author of HEAD commit
	HeadTimestamp string `json:"head_timestamp"`  // ISO 8601 timestamp of HEAD commit
	IsDetached    bool   `json:"is_detached"`    // True if in detached HEAD state
	IsDirty       bool   `json:"is_dirty"`        // True if working directory has uncommitted changes
	RemoteURL     string `json:"remote_url,omitempty"` // URL of remote origin (if configured)
}

// Environment represents environment metadata for a change turn.
type Environment struct {
	GSCVersion string `json:"gsc_version"` // Version of gsc-cli
	User       string `json:"user"`        // OS user who ran the command
	Timestamp  string `json:"timestamp"`   // ISO 8601 timestamp when turn started
}

// DiscoveryLog contains the discovery methodology and pivot checks
type DiscoveryLog struct {
	IntentKeywords       []string            `json:"intent_keywords"`
	PivotChecks          []string            `json:"pivot_checks"`
	Methodology          string              `json:"methodology"`
	TotalCandidatesFound int                 `json:"total_candidates_found"`
	TopCandidatesReturned int                 `json:"top_candidates_returned"`
	ValidationMethod     string              `json:"validation_method"` // How validation was performed
}

// MissingFile represents a file that was missed by discovery but found during validation
type MissingFile struct {
	FilePath        string          `json:"file_path"`
	Score           float64         `json:"score"`
	Reasoning       string          `json:"reasoning"`
	CodeValidation  *CodeValidation `json:"code_validation,omitempty"`
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
	Rating      string   `json:"rating"`     // High/Medium/Low
	Explanation string   `json:"explanation"`
	Matches     []string `json:"matches"`
}

// ValidationLog contains the validation methodology and findings
type ValidationLog struct {
	DiscoveryReviewed      []string            `json:"discovery_reviewed"`       // Files from discovery that were reviewed
	CriticalFindings       []string            `json:"critical_findings"`        // Key discoveries (e.g., missed files)
	MissingFilesIdentified []MissingFile       `json:"missing_files_identified"` // Files discovery missed
	KeywordAssessment      KeywordAssessment   `json:"keyword_assessment"`       // Effectiveness of discovery keywords
	ValidationMethod       string              `json:"validation_method"`      // How validation was performed
	TotalValidated         int                 `json:"total_validated"`
	Confidence             string              `json:"confidence"`               // Confidence level in results
}

// ValidationSummary contains validation phase statistics (rich format)
type ValidationSummary struct {
	SessionIntent            string                    `json:"session_intent"`
	TurnNumber               int                       `json:"turn_number"`
	TotalCandidatesReviewed  int                       `json:"total_candidates_reviewed"`
	ValidatedCandidatesCount int                       `json:"validated_candidates_count"`
	CriticalFinding          string                    `json:"critical_finding"`
	TotalValidated           int                       `json:"total_validated"`
	CandidatesPromoted       int                       `json:"candidates_promoted"`
	CandidatesDemoted        int                       `json:"candidates_demoted"`
	CandidatesRemoved        int                       `json:"candidates_removed"`
	AverageValidatedScore    float64                   `json:"average_validated_score"`
	TopCandidatesCount       int                       `json:"top_candidates_count"`
	Duration                 *int64                    `json:"duration,omitempty"`
	Cost                     *float64                  `json:"cost,omitempty"`
	Usage                    *Usage                    `json:"usage,omitempty"`
	ValidationLog            *ValidationLog            `json:"validation_log,omitempty"`
}

// RichValidatedCandidate represents a validated candidate with detailed analysis
type RichValidatedCandidate struct {
	FilePath         string          `json:"file_path"`
	OriginalScore    float64         `json:"original_score"`
	ValidatedScore    float64        `json:"validated_score"`
	Relevance        string          `json:"relevance"`
	Reasoning        string          `json:"reasoning"`
	CodeValidation CodeValidation    `json:"code_validation"`
	ActionRequired    string         `json:"action_required"`
}

// CriticalMissingCandidate represents a file that discovery missed
type CriticalMissingCandidate struct {
	FilePath         string            `json:"file_path"`
	Score            float64           `json:"score"`
	Relevance        string            `json:"relevance"`
	Reasoning        string            `json:"reasoning"`
	CodeValidation MissingFileCodeValidation `json:"code_validation"`
	ActionRequired    string            `json:"action_required"`
}

// MissingFileCodeValidation contains code validation for missing files
type MissingFileCodeValidation struct {
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
	FutureDiscoveryStrategy string  `json:"future_discovery_strategy"`
}

// SummaryAndRecommendations contains overall assessment and actionable recommendations
type SummaryAndRecommendations struct {
	FilesToModify              []FileModification `json:"files_to_modify"`
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

// FileModified represents a file that was modified during a change turn
type FileModified struct {
	WorkingDir string `json:"working_dir"`
	Path       string `json:"path"`
	Status     string `json:"status"` // "modified", "added", "deleted"
	// Provenance Metadata
	OldBlobSHA   string `json:"old_blob_sha,omitempty"` // SHA before change
	NewBlobSHA   string `json:"new_blob_sha"`          // SHA after change
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
	Scope        string `json:"scope"`  // "in_scope", "out_of_scope"
	Reason       string `json:"reason,omitempty"` // for out_of_scope or user_directed
}

// FilesModifiedSummary contains summary information about modified files
type FilesModifiedSummary struct {
	TotalCount      int            `json:"total_count"`
	InScopeCount    int            `json:"in_scope_count"`
	OutOfScopeCount int            `json:"out_of_scope_count"`
	Files           []FileModified `json:"files"`
}

// DiscoveryGapEntry represents a file that was missed by discovery
type DiscoveryGapEntry struct {
	WorkingDir        string   `json:"working_dir"`
	Path              string   `json:"path"`
	Reason            string   `json:"reason"`
	SuggestedKeywords []string `json:"suggested_keywords"`
}

// DiscoveryGap represents files that were missed by discovery
type DiscoveryGap struct {
	FilesAdded int                 `json:"files_added"`
	Files      []DiscoveryGapEntry `json:"files"`
}

// GSCFileData represents the content of a .change-meta.json file
// This struct serves a dual purpose in the change turn lifecycle:
// 1. **AI Output:** The AI writes a minimal JSON containing only absolute_path, description, and version.
// 2. **CLI Enrichment:** The CLI reads the AI's output, adds technical metadata (old_version, blob SHAs, change_type, language), and writes it back to disk.
// Note: JSON tags match the AI's output format (e.g., `json:"version"`), while Go field names reflect the CLI's internal perspective (e.g., `NewVersion`).
type GSCFileData struct {
    AbsolutePath string `json:"absolute_path"`
    Description  string `json:"description"`
    NewVersion   string `json:"version,omitempty"`        // AI-declared version
    OldVersion   string `json:"old_version,omitempty"`    // CLI-extracted version (from existing header)
    OldBlobSHA   string `json:"old_blob_sha,omitempty"`   // Filled by CLI (SHA before change)
    NewBlobSHA   string `json:"new_blob_sha,omitempty"`   // Filled by CLI (SHA after change)
    ChangeType   string `json:"change_type,omitempty"`    // Filled by CLI: "modified", "added", "deleted"
    Language     string `json:"language,omitempty"`       // Filled by CLI
}

// ChangelogEntry represents a single entry in the aggregated changelog
type ChangelogEntry struct {
	File        string `json:"file"`
	ChangeType  string `json:"change_type"`
	Description string `json:"description"`
	NewVersion  string `json:"new_version,omitempty"`  // Semantic version after change
	OldVersion  string `json:"old_version,omitempty"`  // Semantic version before change
	OldBlobSHA  string `json:"old_blob_sha,omitempty"` // SHA before change
	NewBlobSHA  string `json:"new_blob_sha"`           // SHA after change
	Language    string `json:"language,omitempty"`     // Programming language
}
