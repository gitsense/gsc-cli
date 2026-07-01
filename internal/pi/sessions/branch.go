/**
 * Component: Pi Session Branch Walker
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Walks Pi session JSONL from leaf to root, returning the active branch entries.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// BranchEntry represents a normalized entry on the active branch.
type BranchEntry struct {
	ID        string          `json:"id"`
	ParentID  *string         `json:"parentId"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Role      string          `json:"role,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
}

// BranchResult represents the active branch of a session.
type BranchResult struct {
	Session SessionInfo   `json:"session"`
	Leaf    string        `json:"leaf"`
	Entries []BranchEntry `json:"entries"`
}

// SessionInfo contains session metadata.
type SessionInfo struct {
	Path    string `json:"path"`
	ID      string `json:"id"`
	Version int    `json:"version"`
	CWD     string `json:"cwd"`
}

// GetLatestLeaf returns the ID of the latest (last) entry in a session file.
func GetLatestLeaf(sessionPath string) (string, error) {
	parsed, err := parseSessionFile(sessionPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse session: %w", err)
	}
	if len(parsed.entries) == 0 {
		return "", fmt.Errorf("session has no entries")
	}
	return parsed.entries[len(parsed.entries)-1].ID, nil
}

// WalkBranch reads a session JSONL file and returns the active branch from root to leaf.
func WalkBranch(sessionPath string, leafID string) (*BranchResult, error) {
	// Parse the session file
	parsed, err := parseSessionFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	// Build ID -> entry map
	entryMap := make(map[string]parsedEntry, len(parsed.entries))
	for _, entry := range parsed.entries {
		entryMap[entry.ID] = entry
	}

	// Verify leaf exists
	if _, ok := entryMap[leafID]; !ok {
		return nil, fmt.Errorf("leaf ID %q not found in session", leafID)
	}

	// Walk from leaf to root
	var branch []parsedEntry
	current := leafID
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			return nil, fmt.Errorf("cycle detected at entry %q", current)
		}
		visited[current] = true

		entry, ok := entryMap[current]
		if !ok {
			return nil, fmt.Errorf("broken parent chain: entry %q not found", current)
		}

		branch = append(branch, entry)

		if entry.ParentID == nil {
			break
		}
		current = *entry.ParentID
	}

	// Reverse to get root-to-leaf order
	for i, j := 0, len(branch)-1; i < j; i, j = i+1, j-1 {
		branch[i], branch[j] = branch[j], branch[i]
	}

	// Convert to normalized entries
	entries := make([]BranchEntry, len(branch))
	for i, entry := range branch {
		entries[i] = normalizeEntry(entry)
	}

	absPath, _ := filepath.Abs(sessionPath)

	return &BranchResult{
		Session: SessionInfo{
			Path:    absPath,
			ID:      parsed.header.UUID,
			Version: parsed.header.Version,
			CWD:     parsed.header.CWD,
		},
		Leaf:    leafID,
		Entries: entries,
	}, nil
}

// normalizeEntry converts a parsedEntry to a BranchEntry.
func normalizeEntry(entry parsedEntry) BranchEntry {
	raw, _ := json.Marshal(entry)

	be := BranchEntry{
		ID:        entry.ID,
		ParentID:  entry.ParentID,
		Type:      entry.Type,
		Timestamp: entry.Timestamp,
		Raw:       raw,
	}

	if entry.Message != nil {
		be.Role = entry.Message.Role
		msg, _ := json.Marshal(entry.Message)
		be.Message = msg
	}

	return be
}
