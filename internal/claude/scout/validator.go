/**
 * Component: Scout Setup and Configuration Validator
 * Block-UUID: ee3df130-8a93-4725-ac0e-f51c00efd257
 * Parent-UUID: 2239f8a8-f0ff-4946-a84c-33def25f5158
 * Version: 1.0.2
 * Description: Validates scout session prerequisites (contract, brain, working directories)
 * Language: Go
 * Created-at: 2026-03-27T19:09:14.734Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Type    string // "missing_contract", "missing_brain", "invalid_workdir", "invalid_reference"
	Message string
	Details string
}

// ValidateSetup checks all prerequisites for a scout session
func ValidateSetup(workdirs []WorkingDirectory, refFiles []ReferenceFile) ([]ValidationError, error) {
	var errors []ValidationError

	// Validate working directories
	for _, wd := range workdirs {
		if errs, _ := ValidateWorkdir(wd); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	// Validate reference files
	for _, rf := range refFiles {
		if err := ValidateReferenceFile(rf); err != nil {
			errors = append(errors, *err)
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

	// Check contract.json exists
	contractPath := filepath.Join(wd.Path, "contract.json")
	if _, err := os.Stat(contractPath); err != nil {
		errors = append(errors, ValidationError{
			Type:    "missing_contract",
			Message: fmt.Sprintf("contract.json not found in %s", wd.Name),
			Details: contractPath,
		})
	}

	// Check tiny-overview brain exists
	brainPath := filepath.Join(wd.Path, ".gsc", "brain", "tiny-overview.json")
	if _, err := os.Stat(brainPath); err != nil {
		errors = append(errors, ValidationError{
			Type:    "missing_brain",
			Message: fmt.Sprintf("Tiny Overview brain not found in %s", wd.Name),
			Details: brainPath,
		})
	}

	return errors, nil
}

// ValidateReferenceFile checks that a reference file is readable
func ValidateReferenceFile(rf ReferenceFile) *ValidationError {
	if _, err := os.Stat(rf.OriginalPath); err != nil {
		return &ValidationError{
			Type:    "invalid_reference",
			Message: fmt.Sprintf("Reference file not found: %s", rf.OriginalPath),
			Details: err.Error(),
		}
	}

	// Verify it's readable
	file, err := os.Open(rf.OriginalPath)
	if err != nil {
		return &ValidationError{
			Type:    "invalid_reference",
			Message: fmt.Sprintf("Cannot read reference file: %s", rf.OriginalPath),
			Details: err.Error(),
		}
	}
	file.Close()

	return nil
}

// CheckContractStructure validates that a contract.json has required structure
func CheckContractStructure(contractPath string) error {
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("failed to read contract: %w", err)
	}

	var contract map[string]interface{}
	if err := json.Unmarshal(data, &contract); err != nil {
		return fmt.Errorf("invalid JSON in contract: %w", err)
	}

	// Minimal validation - just ensure it's valid JSON
	// More detailed validation would depend on the actual contract schema
	return nil
}

// CheckBrainStructure validates that a tiny-overview.json has required structure
func CheckBrainStructure(brainPath string) error {
	data, err := os.ReadFile(brainPath)
	if err != nil {
		return fmt.Errorf("failed to read brain: %w", err)
	}

	var brain map[string]interface{}
	if err := json.Unmarshal(data, &brain); err != nil {
		return fmt.Errorf("invalid JSON in brain: %w", err)
	}

	// Check for expected fields
	if _, ok := brain["insights"]; !ok {
		return fmt.Errorf("missing 'insights' field in brain")
	}

	return nil
}

// IsValidSessionStatus checks if a session status is a valid state
func IsValidSessionStatus(status string) bool {
	validStatuses := map[string]bool{
		"discovery":              true,
		"discovery_complete":     true,
		"verification":           true,
		"verification_complete":  true,
		"stopped":                true,
		"error":                  true,
	}
	return validStatuses[status]
}

// CanTransitionStatus checks if a status transition is allowed
func CanTransitionStatus(from, to string) bool {
	transitions := map[string][]string{
		"discovery": {"discovery_complete", "stopped", "error"},
		"discovery_complete": {"verification", "stopped"},
		"verification": {"verification_complete", "stopped", "error"},
		"verification_complete": {"stopped"},
		"stopped": {},
		"error": {"stopped"},
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
