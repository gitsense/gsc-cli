/**
 * Component: Pi Session Error Extractor
 * Block-UUID: d4e5f6a7-b8c9-0123-defa-456789012345
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Extracts failed tool results from Pi session JSONL active branch.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// ToolError represents a failed tool result.
type ToolError struct {
	ToolCallID   string          `json:"toolCallId"`
	ToolName     string          `json:"toolName"`
	Arguments    json.RawMessage `json:"arguments,omitempty"`
	ResultEntryID string         `json:"resultEntryId"`
	ResultText   string          `json:"resultText"`
	Timestamp    string          `json:"timestamp"`
}

// ErrorsResult represents failed tool results from the active branch.
type ErrorsResult struct {
	Session SessionInfo `json:"session"`
	Leaf    string      `json:"leaf"`
	Errors  []ToolError `json:"errors"`
}

// ErrorsOptions configures error extraction.
type ErrorsOptions struct {
	Tool     string // Filter by tool name
	Contains string // Filter by substring in result text
}

// ExtractErrors reads a session JSONL file and returns failed tool results from the active branch.
func ExtractErrors(sessionPath string, leafID string, opts ErrorsOptions) (*ErrorsResult, error) {
	// Get tool calls
	toolCalls, err := ExtractToolCalls(sessionPath, leafID)
	if err != nil {
		return nil, err
	}

	var errors []ToolError

	for _, tc := range toolCalls.ToolCalls {
		// Only include errors
		if !tc.IsError {
			continue
		}

		// Apply tool filter
		if opts.Tool != "" && tc.ToolName != opts.Tool {
			continue
		}

		// Apply contains filter
		if opts.Contains != "" && !strings.Contains(tc.ResultText, opts.Contains) {
			continue
		}

		errors = append(errors, ToolError{
			ToolCallID:    tc.ToolCallID,
			ToolName:      tc.ToolName,
			Arguments:     tc.Arguments,
			ResultEntryID: tc.ResultEntryID,
			ResultText:    tc.ResultText,
			Timestamp:     tc.Timestamp,
		})
	}

	absPath, _ := filepath.Abs(sessionPath)

	return &ErrorsResult{
		Session: SessionInfo{
			Path:    absPath,
			ID:      toolCalls.Session.ID,
			Version: toolCalls.Session.Version,
			CWD:     toolCalls.Session.CWD,
		},
		Leaf:   leafID,
		Errors: errors,
	}, nil
}
