/**
 * Component: Change Result Parser
 * Block-UUID: f2debffe-28ab-477d-b1dc-a2c2c9f1c826
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parses JSON results from Claude change turns into generic TurnResults.
 * Language: Go
 * Created-at: 2026-04-15T16:10:56.123Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// changeResult represents the JSON structure expected from a change turn
type changeResult struct {
	ChangeSummary ChangeSummary      `json:"change_summary"`
	GitDiff       map[string]string `json:"git_diff"`
	Notes         string             `json:"notes"`
	Errors        string             `json:"errors"`
}

// ParseChangeResult attempts to parse a JSON string as a change result.
// It handles markdown code fences and returns a populated TurnResults struct.
func ParseChangeResult(jsonContent string) (*TurnResults, error) {
	// Strip markdown code fences if present
	content := strings.TrimSpace(jsonContent)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// Parse JSON
	var result changeResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse change result: %w", err)
	}

	// Build TurnResults
	turnResults := &TurnResults{
		ChangeResults: &ChangeResults{
			ChangeSummary: result.ChangeSummary,
			GitDiff:       result.GitDiff,
			Notes:         result.Notes,
			Errors:        result.Errors,
		},
	}

	return turnResults, nil
}
