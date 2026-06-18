/**
 * Component: Pi Sessions Data Models
 * Block-UUID: 6be4912a-3e89-42b4-b2de-990863235120
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Defines phase-one sync and query data structures for the Pi sessions mirror.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */

package sessions

type SyncOptions struct {
	SessionsDir string
	DBPath      string
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

	// Branch enrichment fields (populated when WithBranches is set)
	BranchLeafIDs          []string `json:"branch_leaf_ids,omitempty"`
	NearestCompactionID    string   `json:"nearest_compaction_id,omitempty"`
	NearestBranchSummaryID string   `json:"nearest_branch_summary_id,omitempty"`
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
