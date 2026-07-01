/**
 * Component: Pi Sessions Data Models
 * Block-UUID: 5f2b9c84-7d31-4e0a-9b6c-1a8e3f4d2c70
 * Parent-UUID: f8de6294-a1da-472a-9aca-48aa22f5022e
 * Version: 1.6.0
 * Description: Defines phase-one sync and query data structures for the Pi sessions mirror; adds message-preview models for the resume picker's split-pane view, a FirstUserText field on ListResult for resume-picker row titles, and SessionUsage/TouchedFile models that back the HUD sidebar's token gauge and touched-file tree. ContextTokens now includes output tokens to match Pi's context window calculation.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v1.1.0, v1.2.0), claude-opus-4-8 (v1.3.0, v1.4.0, v1.5.0)
 */

package sessions

type SyncOptions struct {
	SessionsDir string
	DBPath      string
	Logger      SyncLogger
}

// SyncLogger provides logging for sync operations.
type SyncLogger interface {
	LogError(msg string)
	LogInfo(msg string)
	LogDebug(msg string)
}

type SyncResult struct {
	SessionsDir       string      `json:"sessions_dir"`
	DBPath            string      `json:"db_path"`
	FilesScanned      int         `json:"files_scanned"`
	SessionsImported  int         `json:"sessions_imported"`
	MessagesImported  int         `json:"messages_imported"`
	ToolCallsImported int         `json:"tool_calls_imported"`
	FileRefsImported  int         `json:"file_refs_imported"`
	Errors            []SyncError `json:"errors,omitempty"`
}

type SyncError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type QueryOptions struct {
	DBPath            string
	File              string
	AbsFile           string
	Repo              string
	SessionID         string
	Tool              string
	Op                string
	Text              string
	CommandStartsWith string
	CommandContains   string
	OutputContains    string
	ToolArgsContains  string
	CaseInsensitive   bool
	Since             string
	Until             string
	Provider          string
	Model             string
	Type              string // deprecated alias for EntryType
	Role              string
	EntryID           string
	View              string // "events" (default) or "sessions"
	EntryType         string
	SessionName       string
	SessionNamePrefix string
	Sort              string // "recent" (default), "oldest", "match-count"
	WithBranches      bool
	Color             string // "auto" (default), "always", "never"
	Limit             int
}

type QueryResult struct {
	Kind          string `json:"kind"`
	SessionID     string `json:"session_id,omitempty"`
	SessionName   string `json:"session_name,omitempty"`
	CWD           string `json:"cwd,omitempty"`
	RepoRoot      string `json:"repo_root,omitempty"`
	EntryID       string `json:"entry_id,omitempty"`
	ToolCallID    string `json:"tool_call_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	Command       string `json:"command,omitempty"`
	ArgumentsJSON string `json:"arguments_json,omitempty"`
	Op            string `json:"op,omitempty"`
	Source        string `json:"source,omitempty"`
	RawPath       string `json:"raw_path,omitempty"`
	AbsPath       string `json:"abs_path,omitempty"`
	FilePathRel   string `json:"file_path_rel,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Type          string `json:"type,omitempty"`
	Role          string `json:"role,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	Text          string `json:"text,omitempty"`

	// Snippet and match highlighting
	Snippet     string         `json:"snippet,omitempty"`
	MatchRanges []MatchRange   `json:"match_ranges,omitempty"`

	// Branch enrichment fields (populated when WithBranches is set)
	BranchLeafIDs          []string `json:"branch_leaf_ids,omitempty"`
	NearestCompactionID    string   `json:"nearest_compaction_id,omitempty"`
	NearestBranchSummaryID string   `json:"nearest_branch_summary_id,omitempty"`
}

// MatchRange represents a highlighted region within a snippet.
type MatchRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// SessionQueryResult represents an aggregated session-level query result.
type SessionQueryResult struct {
	SessionID            string   `json:"session_id"`
	Title                string   `json:"title"`
	Name                 string   `json:"name,omitempty"`
	CWD                  string   `json:"cwd"`
	RepoRoot             string   `json:"repo_root,omitempty"`
	Provider             string   `json:"provider,omitempty"`
	Model                string   `json:"model,omitempty"`
	CreatedAt            string   `json:"created_at"`
	LastMessageAt        string   `json:"last_message_at,omitempty"`
	MessageCount         int      `json:"message_count"`
	ToolCallCount        int      `json:"tool_call_count"`
	FileRefCount         int      `json:"file_ref_count"`
	MatchCount           int      `json:"match_count,omitempty"`
	MatchedFileRefCount  int      `json:"matched_file_ref_count,omitempty"`
	MatchedToolCallCount int      `json:"matched_tool_call_count,omitempty"`
	MatchedMessageCount  int      `json:"matched_message_count,omitempty"`
	MatchedPaths         []string `json:"matched_paths,omitempty"`
}

// ListOptions configures session listing.
type ListOptions struct {
	DBPath   string
	Repo     string
	Since    string
	Until    string
	Provider string
	Model    string
	Sort     string // "recent" (default), "oldest", "messages"
	Limit    int
}

// ListResult represents a single session in list output.
type ListResult struct {
	SessionID        string `json:"session_id"`
	Name             string `json:"name,omitempty"`
	CWD              string `json:"cwd"`
	RepoRoot         string `json:"repo_root,omitempty"`
	CreatedAt        string `json:"created_at"`
	LastMessageAt    string `json:"last_message_at,omitempty"`
	MessageCount     int    `json:"message_count"`
	LastDisplayText  string `json:"last_display_text,omitempty"`
	FirstUserText    string `json:"first_user_text,omitempty"`
}

// PreviewOptions configures a first/last-N message preview for a session.
type PreviewOptions struct {
	DBPath    string
	SessionID string
	Limit     int  // number of messages to return (default 3)
	FromEnd   bool // true = last N (default), false = first N
}

// MessagePreview is one message line rendered in the resume picker preview pane.
// No role/type filtering is applied: tool output is included so a session can be
// found by code that was generated or by a command that was run.
type MessagePreview struct {
	Seq       int    `json:"seq"`
	Type      string `json:"type"`
	Role      string `json:"role,omitempty"`
	Text      string `json:"text,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SessionUsage is the latest provider-reported token usage for a session, read
// from the last assistant message in the mirror (pi_messages.raw_line). It backs
// the HUD context gauge. Tool output is irrelevant here; only assistant rows
// carry usage.
type SessionUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read"`
	CacheWrite   int     `json:"cache_write"`
	TotalTokens  int     `json:"total_tokens"`
	CostTotal    float64 `json:"cost_total"`
}

// ContextTokens approximates how full the context window is: the total tokens
// from the last request (input + output + cached reads/writes). This mirrors
// how Pi derives in-process context usage.
func (u SessionUsage) ContextTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheRead + u.CacheWrite
}

// TouchedFile is one file the agent read, edited, or wrote during a session,
// derived from pi_file_refs. RepoRoot is the session's repo; FilePathRel is set
// for in-repo files and empty for files touched outside any repository.
type TouchedFile struct {
	RepoRoot    string `json:"repo_root,omitempty"`
	FilePathRel string `json:"file_path_rel,omitempty"`
	AbsPath     string `json:"abs_path,omitempty"`
	Op          string `json:"op,omitempty"`
}

// ShowOptions configures session detail view.
type ShowOptions struct {
	DBPath    string
	SessionID string
}

// ShowResult represents detailed session information.
type ShowResult struct {
	SessionID     string `json:"session_id"`
	Name          string `json:"name,omitempty"`
	CWD           string `json:"cwd"`
	RepoRoot      string `json:"repo_root,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	CreatedAt     string `json:"created_at"`
	LastMessageAt string `json:"last_message_at,omitempty"`
	MessageCount  int    `json:"message_count"`
	ToolCallCount int    `json:"tool_call_count"`
	FileRefCount  int    `json:"file_ref_count"`
	FirstUserText string `json:"first_user_text,omitempty"`
	LastUserText  string `json:"last_user_text,omitempty"`
	LastText      string `json:"last_text,omitempty"`
}
