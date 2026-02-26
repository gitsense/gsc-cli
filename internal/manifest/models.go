/**
 * Component: Manifest Models
 * Block-UUID: 4e3987a8-5a5b-4641-a9b0-fe810e4b6a45
 * Parent-UUID: 3c8f1bc5-d234-4560-a188-e7ebc216974d
 * Version: 1.3.1
 * Description: Added CodeMetadata, ContractMetadata, and ProvenanceEntry structs to support the traceability contract system.
 * Language: Go
 * Created-at: 2026-02-26T04:18:29.696Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.3.1)
 */


package manifest

import "time"

// ManifestFile represents the root structure of the downloaded JSON manifest.
type ManifestFile struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Manifest      ManifestInfo  `json:"manifest"`
	Repositories  []Repository  `json:"repositories"`
	Branches      []Branch      `json:"branches"`
	Analyzers     []Analyzer    `json:"analyzers"`
	Fields        []Field       `json:"fields"`
	Data          []DataEntry   `json:"data"`
}

// ManifestInfo contains metadata about the manifest itself.
type ManifestInfo struct {
	ManifestName  string  `json:"name"`
	DatabaseName string   `json:"database_name"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
}

// Repository represents a source repository reference.
type Repository struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// Branch represents a git branch reference.
type Branch struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// Analyzer represents an analyzer that extracted the metadata.
type Analyzer struct {
	Ref         string `json:"ref"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// Field represents a metadata field definition.
type Field struct {
	Ref          string `json:"ref"`
	AnalyzerRef  string `json:"analyzer_ref"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
}

// DataEntry represents a single file's metadata.
type DataEntry struct {
	RepoRef   string                 `json:"repo_ref"`
	BranchRef string                 `json:"branch_ref"`
	FilePath  string                 `json:"file_path"`
	Language  string                 `json:"language"`
	ChatID    int                    `json:"chat_id"`
	Fields    map[string]interface{} `json:"fields"`
}

// CodeMetadata represents the parsed header from a traceable code block.
type CodeMetadata struct {
	Component    string    `json:"component"`
	BlockUUID    string    `json:"block_uuid"`
	ParentUUID   string    `json:"parent_uuid"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Language     string    `json:"language"`
	CreatedAt    time.Time `json:"created_at"`
	Authors      string    `json:"authors"`
}

// ContractStatus defines the lifecycle state of a contract.
type ContractStatus string

const (
	ContractActive    ContractStatus = "active"
	ContractCancelled ContractStatus = "cancelled"
	ContractExpired   ContractStatus = "expired"
)

// ContractMetadata represents a traceability contract stored in ~/.gitsense/contracts.
type ContractMetadata struct {
	UUID        string         `json:"uuid"`
	Description string         `json:"description"`
	Workdir     string         `json:"workdir"`
	ChatID      int64          `json:"chat_id"`
	ChatUUID    string         `json:"chat_uuid"`
	ContractMessageID int64    `json:"contract_message_id"`
	Status      ContractStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
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

