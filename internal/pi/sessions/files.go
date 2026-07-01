/**
 * Component: Pi Session File Reference Extractor
 * Block-UUID: c3d4e5f6-a7b8-9012-cdef-345678901234
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Extracts file references from Pi session JSONL active branch.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"path/filepath"
)

// FileRef represents a file reference from the session.
type FileRef struct {
	Path       string `json:"path"`
	Op         string `json:"op"`
	Source     string `json:"source"`
	ToolCallID string `json:"toolCallId,omitempty"`
	EntryID    string `json:"entryId,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
}

// FilesResult represents file references from the active branch.
type FilesResult struct {
	Session SessionInfo `json:"session"`
	Leaf    string      `json:"leaf"`
	Files   []FileRef   `json:"files"`
}

// ExtractFiles reads a session JSONL file and returns file references from the active branch.
func ExtractFiles(sessionPath string, leafID string) (*FilesResult, error) {
	// Get the branch
	branch, err := WalkBranch(sessionPath, leafID)
	if err != nil {
		return nil, err
	}

	filesMap := make(map[string]FileRef) // dedupe by path+op

	for _, entry := range branch.Entries {
		if entry.Message == nil {
			continue
		}

		var msg messagePayload
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// Extract from tool calls
		if msg.Role == "assistant" {
			for _, block := range msg.Content {
				if block.Type == "toolCall" {
					op := toolNameToOp(block.Name)
					if op == "" {
						continue
					}
					path := extractPathFromArgs(block.Arguments)
					if path == "" {
						continue
					}
					key := path + ":" + op
					filesMap[key] = FileRef{
						Path:       path,
						Op:         op,
						Source:     "tool_call",
						ToolCallID: block.ID,
						EntryID:    entry.ID,
						Timestamp:  entry.Timestamp,
					}
				}
			}
		}

		// Extract from compaction/branch summary details in raw entry
		var rawEntry parsedEntry
		if err := json.Unmarshal(entry.Raw, &rawEntry); err == nil {
			if rawEntry.Details.ReadFiles != nil || rawEntry.Details.ModifiedFiles != nil {
				for _, path := range rawEntry.Details.ReadFiles {
					key := path + ":read"
					if _, exists := filesMap[key]; !exists {
						filesMap[key] = FileRef{
							Path:      path,
							Op:        "read",
							Source:    "branch_summary",
							EntryID:   entry.ID,
							Timestamp: entry.Timestamp,
						}
					}
				}
				for _, path := range rawEntry.Details.ModifiedFiles {
					key := path + ":edit"
					if _, exists := filesMap[key]; !exists {
						filesMap[key] = FileRef{
							Path:      path,
							Op:        "edit",
							Source:    "branch_summary",
							EntryID:   entry.ID,
							Timestamp: entry.Timestamp,
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	files := make([]FileRef, 0, len(filesMap))
	for _, f := range filesMap {
		files = append(files, f)
	}

	absPath, _ := filepath.Abs(sessionPath)

	return &FilesResult{
		Session: SessionInfo{
			Path:    absPath,
			ID:      branch.Session.ID,
			Version: branch.Session.Version,
			CWD:     branch.Session.CWD,
		},
		Leaf:  leafID,
		Files: files,
	}, nil
}

// toolNameToOp maps tool names to file operations.
func toolNameToOp(toolName string) string {
	switch toolName {
	case "read":
		return "read"
	case "edit":
		return "edit"
	case "write":
		return "write"
	default:
		return ""
	}
}

// extractPathFromArgs extracts a file path from tool call arguments.
func extractPathFromArgs(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var parsed struct {
		Path     string `json:"path"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return ""
	}
	if parsed.Path != "" {
		return parsed.Path
	}
	return parsed.FilePath
}
