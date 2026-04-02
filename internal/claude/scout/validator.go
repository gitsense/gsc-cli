/**
 * Component: Scout Setup and Configuration Validator
 * Block-UUID: fde264bb-f146-4055-b30c-b463d25ad0f4
 * Parent-UUID: 76f5d39b-3ce2-471f-9faa-4a5aa83b65e0
 * Version: 1.3.1
 * Description: Validates scout session prerequisites (brain database, working directories). Updated to check for tiny-overview brain in database registry instead of on disk.
 * Language: Go
 * Created-at: 2026-04-01T02:18:32.774Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), claude-haiku-4-5-20251001 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1)
 */


package scout

import (
	"context"
	"encoding/json"
	"bufio"
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/registry"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Type    string // "missing_brain", "invalid_workdir", "invalid_reference", "missing_fields"
	Message string
	Details string
}

// RequiredFields for the Tiny Overview brain
var RequiredFields = []string{
	"file_extension",
	"keywords",
	"parent_keywords",
	"purpose",
}

// ValidateSetup checks all prerequisites for a scout session
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

	// Validate tiny-overview brain exists in database
	if errs := ValidateBrainDatabase("tiny-overview"); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors, nil
}

// ValidateBrainDatabase checks if the specified brain database exists and contains required fields
func ValidateBrainDatabase(dbName string) []ValidationError {
	var errors []ValidationError

	// 1. Check if database exists in registry
	reg, err := registry.LoadRegistry()
	if err != nil {
		return []ValidationError{
			{
				Type:    "missing_brain",
				Message: fmt.Sprintf("Failed to load registry: %v", err),
				Details: "Cannot verify brain database without registry access",
			},
		}
	}

	_, exists := reg.FindEntryByDBName(dbName)
	if !exists {
		return []ValidationError{
			{
				Type:    "missing_brain",
				Message: fmt.Sprintf("Brain database '%s' not found in registry", dbName),
				Details: "Please import the brain database using 'gsc manifest import'",
			},
		}
	}

	// 2. Check if database file exists on disk
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err := db.ValidateDBExists(dbPath); err != nil {
		return []ValidationError{
			{
				Type:    "missing_brain",
				Message: fmt.Sprintf("Brain database file not found: %s", dbName),
				Details: err.Error(),
			},
		}
	}

	// 3. Validate that database contains required fields
	ctx := context.Background()
	availableFields, err := manifest.ListFieldNames(ctx, dbName)
	if err != nil {
		return []ValidationError{
			{
				Type:    "missing_brain",
				Message: fmt.Sprintf("Failed to query brain database schema: %s", dbName),
				Details: err.Error(),
			},
		}
	}

	// Check for missing required fields
	availableFieldMap := make(map[string]bool)
	for _, field := range availableFields {
		availableFieldMap[field] = true
	}

	var missingFields []string
	for _, requiredField := range RequiredFields {
		if !availableFieldMap[requiredField] {
			missingFields = append(missingFields, requiredField)
		}
	}

	if len(missingFields) > 0 {
		errors = append(errors, ValidationError{
			Type:    "missing_fields",
			Message: fmt.Sprintf("Brain database '%s' is missing required fields", dbName),
			Details: fmt.Sprintf("Missing fields: %v", missingFields),
		})
	}

	return errors
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
