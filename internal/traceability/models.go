/*
 * Component: Traceability Models
 * Block-UUID: 96c84fa7-3042-43dd-b8ed-2a2f394493c0
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the CodeMetadata struct for parsing traceability headers from code blocks.
 * Language: Go
 * Created-at: 2026-02-26T05:15:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package traceability

import "time"

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
