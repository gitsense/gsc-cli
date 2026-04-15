/**
 * Component: Discovery Result Parser
 * Block-UUID: a5dcfd69-2db2-4dbd-9c98-2a4346f73ff7
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parses JSON results from Claude discovery turns into generic TurnResults.
 * Language: Go
 * Created-at: 2026-04-15T16:10:04.265Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// discoveryResult represents the JSON structure expected from a discovery turn
type discoveryResult struct {
	Candidates   []Candidate    `json:"candidates"`
	Duration     *int64         `json:"duration,omitempty"`
	Cost         *float64       `json:"cost,omitempty"`
	Usage        *Usage         `json:"usage,omitempty"`
	TotalFound   int            `json:"total_found"`
	Coverage     string         `json:"coverage"`
	DiscoveryLog *DiscoveryLog  `json:"discovery_log"`
}

// ParseDiscoveryResult attempts to parse a JSON string as a discovery result.
// It handles markdown code fences and returns a populated TurnResults struct.
func ParseDiscoveryResult(jsonContent string) (*TurnResults, error) {
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
	var result discoveryResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse discovery result: %w", err)
	}

	// Build TurnResults
	turnResults := &TurnResults{
		Candidates:   result.Candidates,
		DiscoveryLog: result.DiscoveryLog,
		Coverage:     result.Coverage,
		Duration:     result.Duration,
		Cost:         result.Cost,
		Usage:        result.Usage,
	}

	return turnResults, nil
}
