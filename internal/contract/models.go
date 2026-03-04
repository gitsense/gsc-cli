/**
 * Component: Contract Models
 * Block-UUID: ce6ee81c-cebd-4e72-a311-f4f6ec08b959
 * Parent-UUID: 12241c3c-c019-48f0-9726-faf0722ef3aa
 * Version: 1.15.0
 * Description: Added MappedDumpResult, MappedFileEntry, and Provenance structs to support the 'mapped' dump type. These structures define the JSON response format for the CLI and the schema for the provenance.json sidecar files.
 * Language: Go
 * Created-at: 2026-03-03T18:35:47.821Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., Gemini 3 Flash (v1.13.0), Gemini 3 Flash (v1.14.0), Gemini 3 Flash (v1.15.0)
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
	Sort         string `json:"sort,omitempty"` // Sort mode for 'merged' dump type
	DebugPatch   bool   `json:"debug_patch,omitempty"` // Enable patch debugging artifacts
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

// ==========================================
// Mapped Dump Types
// ==========================================

// MappedFileStatus defines the mapping status of a file in the dump.
type MappedFileStatus string

const (
	MappedStatusMapped   MappedFileStatus = "mapped"
	MappedStatusUnmapped MappedFileStatus = "unmapped"
)

// MappedFileEntry represents a single file in the mapped dump result.
type MappedFileEntry struct {
	Path      string           `json:"path"`       // Relative path in the project (or component name)
	Status    MappedFileStatus `json:"status"`     // "mapped" or "unmapped"
	BlockUUID string           `json:"block_uuid"` // The UUID of the code block
	Reason    string           `json:"reason,omitempty"` // Why it was unmapped (e.g., "no_parent_uuid")
}

// MappedDumpStats provides summary statistics for the dump.
type MappedDumpStats struct {
	Mappable   int `json:"mappable"`   // Count of successfully mapped files
	Unmappable int `json:"unmappable"` // Count of unmapped files
}

// MappedDumpResult is the JSON response structure for the 'mapped' dump command.
type MappedDumpResult struct {
	Success  bool             `json:"success"`
	Hash     string           `json:"hash"`      // The message hash (directory name)
	RootDir  string           `json:"root_dir"`  // Absolute path to the dump directory
	Stats    MappedDumpStats  `json:"stats"`
	Files    []MappedFileEntry `json:"files"`
	Error    *DumpError       `json:"error,omitempty"` // Present if Success is false
}

// DumpError represents a structured error for the dump response.
type DumpError struct {
	Code    string `json:"code"`    // e.g., "INVALID_MESSAGE_ID"
	Message string `json:"message"` // Human-readable description
}

// Provenance represents the content of the provenance.json sidecar file.
type Provenance struct {
	FilePath      string   `json:"file_path"`      // Relative path in the project
	BlockUUID     string   `json:"block_uuid"`     // The UUID of the code block
	ParentUUID    string   `json:"parent_uuid"`    // The UUID of the parent block
	Version       string   `json:"version"`        // The version string
	ChatID        int64    `json:"chat_id"`        // The ID of the chat
	MessageID     int64    `json:"message_id"`     // The ID of the message
	ContractUUID  string   `json:"contract_uuid"`  // The contract UUID
	Model         string   `json:"model"`          // The AI model name
	Timestamp     string   `json:"timestamp"`      // ISO 8601 timestamp
	Action        string   `json:"action"`         // e.g., "patch_applied", "full_code"
	Authors       []string `json:"authors"`        // List of authors
}
