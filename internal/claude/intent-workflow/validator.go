/**
 * Component: Intent Workflow Session Validator
 * Block-UUID: 5cda1c1d-ddc6-411e-9f1a-5253c46fca27
 * Parent-UUID: 7a684113-e9c9-44c4-ba8c-b4a8d2dcddad
 * Version: 1.6.0
 * Description: Generic validation functions for intent workflow sessions including intent validation, status transitions, and turn sequence validation. Updated for Intent Workflow with discovery → change flow. Added support for Discovery → Discovery flow by allowing "discovery_complete" to transition back to "discovery" status. Updated ValidateTurnSequence() to allow multiple discovery turns in sequence.
 * Language: Go
 * Created-at: 2026-05-02T18:51:17.954Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
 */


package intent_workflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Type    string // "missing_brain", "invalid_workdir", "invalid_reference", "missing_fields"
	Message string
	Details string
}

// ValidateIntent checks that the intent string is non-empty and reasonable
func ValidateIntent(intent string) error {
	if intent == "" {
		return fmt.Errorf("intent cannot be empty")
	}

	if len(intent) > 10000 {
		return fmt.Errorf("intent is too long (max 10000 characters)")
	}

	return nil
}

// IsValidSessionStatus checks if a session status is a valid state
func IsValidSessionStatus(status string) bool {
	validStatuses := map[string]bool{
		"discovery":               true,
		"discovery_complete":      true,
		"validation":              true,
		"validation_complete":     true,
		"change":                  true,
		"change_post_processing":  true,
		"change_complete":         true,
		"stopped":                 true,
		"error":                   true,
	}
	return validStatuses[status]
}

// CanTransitionStatus checks if a status transition is allowed
func CanTransitionStatus(from, to string) bool {
	transitions := map[string][]string{
		"discovery":               {"discovery_complete", "stopped", "error"},
		"discovery_complete":      {"change", "stopped", "discovery"},
		"validation":              {"validation_complete", "stopped", "error"},
		"validation_complete":     {"stopped"},
		"change":                  {"change_post_processing", "stopped", "error"},
		"change_post_processing":  {"change_complete", "stopped", "error"},
		"change_complete":         {"stopped"},
		"stopped":                 {},
		"error":                   {"stopped"},
	}

	allowed, exists := transitions[from]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTurnSequence checks if a turn type is valid given the current session state
func ValidateTurnSequence(turnType string, turns []TurnState) error {
	// Validate turn-type value
	if turnType != "discovery" && turnType != "change" {
		return fmt.Errorf("turn-type must be 'discovery' or 'change'")
	}

	// If no turns exist, first turn must be discovery
	if len(turns) == 0 {
		if turnType != "discovery" {
			return fmt.Errorf("first turn must be discovery")
		}
		return nil
	}

	// Get the last turn
	lastTurn := turns[len(turns)-1]

	// Can't start new turn if last turn failed
	if lastTurn.Status == "error" {
		return fmt.Errorf("cannot start new turn: previous turn failed. Please retry the failed turn")
	}

	// Discovery turn: can follow another discovery turn (if complete) or be the first turn
	if turnType == "discovery" {
		// Allow discovery after discovery_complete (for multiple discovery turns)
		if lastTurn.TurnType == "discovery" && lastTurn.Status == "complete" {
			return nil
		}
		// Allow discovery after skipped discovery
		if lastTurn.TurnType == "discovery" && lastTurn.Status == "skipped" {
			return nil
		}
		// If last turn was not discovery, it's invalid
		if lastTurn.TurnType != "discovery" {
			return fmt.Errorf("cannot start discovery turn: last turn was %s (expected discovery)", lastTurn.TurnType)
		}
		// If last turn was discovery but not complete/skipped, it's invalid
		return fmt.Errorf("cannot start discovery turn: previous discovery turn is not complete (status: %s)", lastTurn.Status)
	}

	// Change turn requires discovery_complete status
	if turnType == "change" {
		if lastTurn.TurnType != "discovery" {
			return fmt.Errorf("cannot start change turn: last turn was not discovery")
		}
		if lastTurn.Status != "complete" && lastTurn.Status != "skipped" {
			return fmt.Errorf("cannot start change turn: discovery turn is not complete (status: %s)", lastTurn.Status)
		}
	}

	return nil
}

// ValidateReferenceFilesJSON checks that a reference files JSON file is valid NDJSON format
func ValidateReferenceFilesJSON(filePath string) *ValidationError {
	if filePath == "" {
		return nil // Reference files are optional
	}

	if _, err := os.Stat(filePath); err != nil {
		return &ValidationError{
			Type:    "invalid_reference",
			Message: fmt.Sprintf("Reference files JSON not found: %s", filePath),
			Details: err.Error(),
		}
	}

	// Verify it's readable and valid NDJSON
	file, err := os.Open(filePath)
	if err != nil {
		return &ValidationError{
			Type:    "invalid_reference",
			Message: fmt.Sprintf("Cannot read reference files JSON: %s", filePath),
			Details: err.Error(),
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var ref map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &ref); err != nil {
			return &ValidationError{
				Type:    "invalid_reference",
				Message: fmt.Sprintf("Invalid JSON at line %d: %v", lineNum, err),
				Details: err.Error(),
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &ValidationError{
			Type:    "invalid_reference",
			Message: "Error reading reference files JSON",
			Details: err.Error(),
		}
	}

	return nil
}

// ValidateSetup checks all prerequisites for an intent workflow session
func ValidateSetup(workdirs []WorkingDirectory, refFilesContext []ReferenceFileContext) ([]ValidationError, error) {
	var errors []ValidationError

	// Validate working directories
	for _, wd := range workdirs {
		if errs, _ := ValidateWorkdir(wd); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return errors, nil
}

// ValidateWorkdir checks that a working directory has required files
func ValidateWorkdir(wd WorkingDirectory) ([]ValidationError, error) {
	var errors []ValidationError

	// Check directory exists
	info, err := os.Stat(wd.Path)
	if err != nil {
		return []ValidationError{
			{
				Type:    "invalid_workdir",
				Message: fmt.Sprintf("Working directory does not exist: %s", wd.Name),
				Details: err.Error(),
			},
		}, nil
	}

	if !info.IsDir() {
		return []ValidationError{
			{
				Type:    "invalid_workdir",
				Message: fmt.Sprintf("Path is not a directory: %s", wd.Name),
				Details: wd.Path,
			},
		}, nil
	}

	return errors, nil
}
