/*
 * Component: Contract Models
 * Block-UUID: 35bfab3b-5c8c-4e96-aeeb-a6c32972084b
 * Parent-UUID: 4b6ba222-01b0-4c96-acf4-3b12b84b9830
 * Version: 1.12.0
 * Description: Added ContractDone status to the ContractStatus enum to support marking projects as finished while preserving their history.
 * Language: Go
 * Created-at: 2026-03-03T02:23:17.722Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., Gemini 3 Flash (v1.11.0), Gemini 3 Flash (v1.12.0)
 */


package contract

import "time"

// ContractStatus defines the lifecycle state of a contract.
type ContractStatus string

const (
	ContractActive    ContractStatus = "active"
	ContractCancelled ContractStatus = "cancelled"
	ContractExpired   ContractStatus = "expired"
	ContractDone      ContractStatus = "done"
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
	PreferredReview   string `json:"preferred_review"`   // e.g., "vimdiff", "zed --diff"
}

// LaunchRequest represents the data contract from the Web UI to the CLI for the 'launch' command.
type LaunchRequest struct {
	ContractUUID string `json:"contract_uuid"`
	Authcode     string `json:"authcode"`
	Alias        string `json:"alias"` // "review", "terminal", "editor", "exec", "dump"
	BlockUUID    string `json:"block_uuid,omitempty"`
	ParentUUID   string `json:"parent_uuid,omitempty"`
	Action       string `json:"action,omitempty"` // "source", "patch" (review) or "tree", "text", "mapped" (dump)
	AppOverride  string `json:"app_override,omitempty"` // Overrides preferred app (e.g., "zed", "iterm2")
	Cmd          string `json:"cmd,omitempty"`
}

// LaunchResult represents the response from the CLI to the Web UI for the 'launch' command.
type LaunchResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Alias      string `json:"alias"`
	Workdir    string `json:"workdir"`
	StagedPath string `json:"staged_path,omitempty"`
	Command    string `json:"command,omitempty"`
}

// LaunchCapabilities represents the response for 'gsc contract launch --list'.
type LaunchCapabilities struct {
	Aliases  []AliasDefinition `json:"aliases"`
	Apps     AppDefinitions    `json:"apps"`
	Commands []string          `json:"commands"`
}

// AliasDefinition defines a workflow alias.
type AliasDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AppDefinitions groups available apps by category.
type AppDefinitions struct {
	Editors   []string `json:"editors"`
	Terminals []string `json:"terminals"`
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
	PreferredReview   string    `json:"preferred_review"`
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
