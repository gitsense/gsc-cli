/*
Component: Correction Turn Result Parser
Block-UUID: 38f34102-267a-432b-b941-34f1fcbfe43a
Parent-UUID: N/A
Version: 1.0.0
Description: Parses and validates the JSON response produced by a correction turn. Handles success, insufficient_data, and failed statuses, and exposes the corrected discovery JSON for re-parsing.
Language: Go
Created-at: {{UTC-TIME}}
Authors: Gemini 2.5 Flash Lite (v1.0.0)
*/


package intent_workflow

import (
	"encoding/json"
	"fmt"
)

// Correction status values returned by the correction turn AI.
const (
	CorrectionStatusSuccess          = "success"
	CorrectionStatusInsufficientData = "insufficient_data"
	CorrectionStatusFailed           = "failed"
)

// CorrectionResult is the top-level response produced by a correction turn.
//
// On success the AI embeds the full corrected discovery JSON inside
// CorrectedOutput. On failure it omits that field and explains why in
// Reasoning and ErrorsRemaining.
type CorrectionResult struct {
	// Status is one of: "success", "insufficient_data", "failed".
	Status string `json:"status"`

	// CorrectedOutput holds the corrected discovery JSON when Status is
	// "success". Stored as raw JSON so it can be forwarded directly to
	// ParseDiscoveryResult without an extra marshal/unmarshal round-trip.
	CorrectedOutput json.RawMessage `json:"corrected_output,omitempty"`

	// Reasoning is a human-readable explanation of the outcome.
	Reasoning string `json:"reasoning"`

	// ErrorsFixed lists the format errors that were successfully corrected.
	// Populated only when Status is "success".
	ErrorsFixed []string `json:"errors_fixed,omitempty"`

	// ErrorsRemaining lists format errors the AI could not resolve.
	// Populated when Status is "insufficient_data" or "failed".
	ErrorsRemaining []string `json:"errors_remaining,omitempty"`
}

// ParseCorrectionResult unmarshals the raw JSON output from a correction
// turn subprocess and validates that the status field contains a recognised
// value.
//
// Returns an error if the JSON is syntactically invalid, if the status is
// unrecognised, or if the status is "success" but no corrected_output is
// present.
func ParseCorrectionResult(jsonContent string) (*CorrectionResult, error) {
	var result CorrectionResult
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		return nil, fmt.Errorf("correction result: json parse failed: %w", err)
	}

	switch result.Status {
	case CorrectionStatusSuccess, CorrectionStatusInsufficientData, CorrectionStatusFailed:
		// valid
	case "":
		return nil, fmt.Errorf("correction result: missing required field \"status\"")
	default:
		return nil, fmt.Errorf(
			"correction result: invalid status %q (must be one of: success, insufficient_data, failed)",
			result.Status,
		)
	}

	if result.Status == CorrectionStatusSuccess && len(result.CorrectedOutput) == 0 {
		return nil, fmt.Errorf(
			"correction result: status is \"success\" but \"corrected_output\" is absent",
		)
	}

	return &result, nil
}

// GetCorrectedDiscoveryJSON returns the corrected_output field as a plain
// string suitable for passing to ParseDiscoveryResult.
//
// Returns an error if the result status is not "success" or if the field is
// empty.
func (r *CorrectionResult) GetCorrectedDiscoveryJSON() (string, error) {
	if r.Status != CorrectionStatusSuccess {
		return "", fmt.Errorf(
			"cannot extract corrected output: correction status is %q", r.Status,
		)
	}
	if len(r.CorrectedOutput) == 0 {
		return "", fmt.Errorf("corrected_output is empty")
	}
	return string(r.CorrectedOutput), nil
}

// IsTerminal reports whether this correction status should stop the retry
// loop immediately (i.e. without consuming further attempts).
//
// "insufficient_data" is terminal because additional attempts against the
// same malformed input will not produce a better outcome.
func (r *CorrectionResult) IsTerminal() bool {
	return r.Status == CorrectionStatusInsufficientData
}
