/**
 * Component: Contract Models
 * Block-UUID: 9ac45e3c-7340-45e8-8b75-94847ab87846
 * Parent-UUID: d14c8dd9-6409-4bb8-b827-60c8d4d5e38d
 * Version: 1.6.0
 * Description: Added PreferredEditor and PreferredTerminal to ContractMetadata. Added ReviewRequest and ReviewResult structs to support the AI code review data contract between the Web UI and CLI.
 * Language: Go
 * Created-at: 2026-02-27T16:52:47.050Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
 */


package contract

import "time"

// ContractStatus defines the lifecycle state of a contract.
type ContractStatus string

const (
	ContractActive    ContractStatus = "active"
	ContractCancelled ContractStatus = "cancelled"
	ContractExpired   ContractStatus = "expired"
)

// ContractMetadata represents a traceability contract stored in ~/.gitsense/contracts.
type ContractMetadata struct {
	UUID             string         `json:"uuid"`
	Authcode         string         `json:"authcode"`
	Description      string         `json:"description"`
	Workdir          string         `json:"workdir"`
	ChatID           int64          `json:"chat_id"`
	ChatUUID         string         `json:"chat_uuid"`
	ContractMessageID int64         `json:"contract_message_id"`
	Status           ContractStatus `json:"status"`
	CreatedAt        time.Time      `json:"created_at"`
	ExpiresAt        time.Time      `json:"expires_at"`
	
	// Execution Security Fields
	Whitelist []string `json:"whitelist"`
	NoWhitelist bool      `json:"no_whitelist"`
	ExecTimeout int       `json:"exec_timeout"`

	// Workspace Preferences
	PreferredEditor   string `json:"preferred_editor"`   // e.g., "zed", "vscode", "vim-iterm2"
	PreferredTerminal string `json:"preferred_terminal"` // e.g., "iterm2", "terminal.app"
}

// ReviewRequest represents the data contract from the Web UI to the CLI.
type ReviewRequest struct {
	ContractUUID     string `json:"contract_uuid"`
	Authcode         string `json:"authcode"`
	Intent           string `json:"intent"` // "review", "terminal", "editor", "exec"
	BlockUUID        string `json:"block_uuid,omitempty"`
	ParentUUID       string `json:"parent_uuid,omitempty"`
	Action           string `json:"action,omitempty"` // "source", "patch"
	EditorOverride   string `json:"editor_override,omitempty"`
	TerminalOverride string `json:"terminal_override,omitempty"`
	Cmd              string `json:"cmd,omitempty"`
}

// ReviewResult represents the response from the CLI to the Web UI.
type ReviewResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	StagedPath string `json:"staged_path,omitempty"`
	Command    string `json:"command,omitempty"`
}

// ProvenanceStatus defines the state of a provenance log entry.
type ProvenanceStatus string

const (
	ProvenanceSaved     ProvenanceStatus = "saved"
	ProvenanceAttempted ProvenanceStatus = "attempted"
	ProvenanceFailed    ProvenanceStatus = "failed"
)

// ProvenanceEntry represents a single "receipt" in the project-local provenance.log.
type ProvenanceEntry struct {
	Timestamp     time.Time        `json:"timestamp"`
	Status        ProvenanceStatus `json:"status"`
	Action        string           `json:"action"`          // "update-file" or "create-file"
	FilePath      string           `json:"file_path"`       // Relative to repo root
	BlockUUID     string           `json:"block_uuid"`      // The New Block-UUID
	ParentUUID    string           `json:"parent_uuid"`     // The UUID of the code being replaced
	SourceVersion string           `json:"source_version"`  // The version being replaced
	TargetVersion string           `json:"target_version"`  // The version of the new code
	Author        string           `json:"author"`          // The LLM that generated the code
	ContractUUID  string           `json:"contract_uuid"`   // Link to the global contract
	Error         string           `json:"error,omitempty"` // Error message if status is failed
	Description   string           `json:"description"`     // User-defined intent for the change
}

// ContractInfoResult represents the sanitized output of the 'contract info' command.
type ContractInfoResult struct {
	UUID              string    `json:"uuid"`
	Description       string    `json:"description"`
	Workdir           string    `json:"workdir"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	Authcode          string    `json:"authcode"`
	ExecTimeout       int       `json:"exec_timeout"`
	Whitelist         []string  `json:"whitelist"`
	NoWhitelist       bool      `json:"no_whitelist"`
	PreferredEditor   string    `json:"preferred_editor"`
	PreferredTerminal string    `json:"preferred_terminal"`
}

// ContractTestResult represents the output of the 'contract test' command.
type ContractTestResult struct {
	ContractInfoResult // Embedded: Includes UUID, Status, ExpiresAt, etc.

	// Test Specific Fields
	Success      bool   `json:"success"`       // Overall success/failure
	ErrorCode    string `json:"error_code"`    // Specific error code if failed
	Message      string `json:"message"`       // Human-readable status message
	RelativePath string `json:"relative_path"` // Path relative to workdir (if parent found)
	DiffHTML     string `json:"diff_html"`     // HTML fragment of the diff (if parent found)
	DiffUnified  string `json:"diff_unified"`  // Unified diff text (if parent found)
	BlockUUID    string `json:"block_uuid"`    // The UUID from the source file
	ParentUUID   string `json:"parent_uuid"`   // The parent UUID from the source file
	IsUnique     bool   `json:"is_unique"`     // Whether the Block-UUID is unique
}
