/*
 * Component: Import Git NDJSON Parser
 * Block-UUID: 1e3e65e9-b04e-4cc9-a6ee-d6ff9bb97e51
 * Parent-UUID: 1e874428-5669-4ae6-981d-aaaf4ad5a762
 * Version: 1.2.0
 * Description: Defines the data structures for NDJSON events emitted by gscb-cli and provides the parsing logic. Version bump to align with Phase 1 finalization.
 * Language: Go
 * Created-at: 2026-05-13T18:20:15.789Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package importgit

import (
	"encoding/json"
	"fmt"
)

// NDJSONEvent represents a single line of the structured output stream
type NDJSONEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// DataInit is the payload for the 'init' event
type DataInit struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
	RefType string `json:"refType"`
}

// DataScanComplete is the payload for the 'scan_complete' event
type DataScanComplete struct {
	TotalFiles int    `json:"total_files"`
	Mode       string `json:"mode"`
}

// DataResumeDetected is the payload for the 'resume_detected' event
type DataResumeDetected struct {
	AlreadyImported int `json:"already_imported"`
}

// DataFileStart is the payload for the 'file_start' event
type DataFileStart struct {
	Path string `json:"path"`
}

// DataFileSkip is the payload for the 'file_skip' event
type DataFileSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// DataFileDone is the payload for the 'file_done' event
type DataFileDone struct {
	Path   string `json:"path"`
	Tokens int    `json:"tokens"`
}

// DataComplete is the payload for the 'complete' event
type DataComplete struct {
	RefChatID  int64 `json:"ref_chat_id"`
	DurationMs int   `json:"duration_ms"`
}

// DataError is the payload for the 'error' event
type DataError struct {
	Message string `json:"message"`
}

// ParseLine parses a single line of NDJSON into an NDJSONEvent struct
func ParseLine(line string) (*NDJSONEvent, error) {
	var event NDJSONEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON: %w", err)
	}
	return &event, nil
}
