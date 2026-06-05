/**
 * Component: Change Result Parser
 * Block-UUID: 6f48634c-4398-40d4-8cd1-827f05fcc734
 * Parent-UUID: 535e668e-9a87-4a5a-a274-d3635672b0ae
 * Version: 2.7.0
 * Description: Parses JSON results from Claude change turns into generic TurnResults. Removed duplicate struct definitions which are now defined in models.go.
 * Language: Go
 * Created-at: 2026-04-27T03:29:48.984Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.6.0), GLM-4.7 (v2.7.0)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// changeResult represents the JSON structure expected from a change turn
type changeResult struct {
	ChangeRequest       string               `json:"change_request"`
	FilesModified       FilesModifiedSummary `json:"files_modified"`
	DiscoveryGap        DiscoveryGap         `json:"discovery_gap"`
	Changelog           []ChangelogEntry     `json:"changelog"`
	Notes               string               `json:"notes"`
	Errors              string               `json:"errors"`
}

// ParseChangeResult attempts to parse a JSON string as a change result.
// It handles markdown code fences and returns a populated TurnResult struct.
func ParseChangeResult(jsonContent string) (*TurnResult, error) {
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

	// Build TurnResult
	turnResult := &TurnResult{
		Change: &ChangeResult{
			ChangeRequest: result.ChangeRequest,
			FilesModified: result.FilesModified,
			DiscoveryGap:  result.DiscoveryGap,
			Changelog:     result.Changelog,
			Notes:         result.Notes,
			Errors:        result.Errors,
		},
	}

	return turnResult, nil
}

// ValidateChangeResult checks a parsed TurnResults for semantic format errors
// that can occur even when JSON unmarshal succeeds. Returns a slice of
// human-readable error strings; returns nil when the result is valid.
func ValidateChangeResult(result *TurnResult) []string {
	var errs []string
	if result.Change == nil {
		return []string{"missing required field change"}
	}
	cr := result.Change
	cs := cr // ChangeResult is now flattened

	if cs.ChangeRequest == "" {
		errs = append(errs, "change_request is required")
	}

	total := cs.FilesModified.TotalCount
	actual := len(cs.FilesModified.Files)
	if total != actual {
		errs = append(errs, fmt.Sprintf(
			"files_modified.total_count (%d) does not match actual files array length (%d)",
			total, actual,
		))
	}

	inScope, outScope := 0, 0
	for i, f := range cs.FilesModified.Files {
		p := fmt.Sprintf("files_modified[%d]", i)
		if f.WorkingDir == "" {
			errs = append(errs, p+": missing working_dir")
		}
		if f.Path == "" {
			errs = append(errs, p+": missing path")
		}
		switch f.Status {
		case "modified", "added", "deleted":
		default:
			errs = append(errs, fmt.Sprintf("%s: invalid status %q", p, f.Status))
		}
		switch f.Scope {
		case "in_scope":
			inScope++
		case "out_of_scope":
			outScope++
		default:
			errs = append(errs, fmt.Sprintf("%s: invalid scope %q", p, f.Scope))
		}
	}

	if inScope != cs.FilesModified.InScopeCount {
		errs = append(errs, fmt.Sprintf(
			"files_modified.in_scope_count (%d) does not match actual in_scope files (%d)",
			cs.FilesModified.InScopeCount, inScope,
		))
	}
	if outScope != cs.FilesModified.OutOfScopeCount {
		errs = append(errs, fmt.Sprintf(
			"files_modified.out_of_scope_count (%d) does not match actual out_of_scope files (%d)",
			cs.FilesModified.OutOfScopeCount, outScope,
		))
	}

	gapCount := cs.DiscoveryGap.FilesAdded
	actualGap := len(cs.DiscoveryGap.Files)
	if gapCount != actualGap {
		errs = append(errs, fmt.Sprintf(
			"discovery_gap.files_added (%d) does not match actual gap files array length (%d)",
			gapCount, actualGap,
		))
	}

	return errs
}

// ParseResumeResult parses the JSON confirmation from a resume turn.
func ParseResumeResult(jsonContent string) (*TurnResult, error) {
	// Strip markdown code fences
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

	// Parse the simple confirmation
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse resume result: %w", err)
	}

	if result.Status != "complete" {
		return nil, fmt.Errorf("resume result status is not 'complete': %s", result.Status)
	}

	// Return a minimal TurnResult to satisfy the interface
	return &TurnResult{}, nil
}
