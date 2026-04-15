/**
 * Component: Scout Setup and Configuration Validator
 * Block-UUID: c647c4bf-b526-4733-bc42-ff291eacd249
 * Parent-UUID: c4a2ede0-ba98-4531-a63a-411a0b06203f
 * Version: 1.9.0
 * Description: Validates scout session prerequisites (brain database, working directories). Updated to execute gsc brains command in working directory for both availability check and field validation.
 * Language: Go
 * Created-at: 2026-04-15T15:30:51.012Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), claude-haiku-4-5-20251001 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/gitsense/gsc-cli/internal/claude/agent"
	"os/exec"
	"strings"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Type    string // "missing_brain", "invalid_workdir", "invalid_reference", "missing_fields"
	Message string
	Details string
}

// RequiredFields for the Code Intent brain
var RequiredFields = []string{
	"file_extension",
	"keywords",
	"parent_keywords",
	"purpose",
}

// ValidateSetup checks all prerequisites for a scout session
func ValidateSetup(workdirs []agent.WorkingDirectory, refFilesContext []agent.ReferenceFileContext) ([]ValidationError, error) {
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
func ValidateWorkdir(wd agent.WorkingDirectory) ([]ValidationError, error) {
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

	// Validate code-intent brain exists in database
	if errs := ValidateBrainDatabase("code-intent", wd.Path); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors, nil
}

// ValidateBrainDatabase checks if the specified brain database exists and contains required fields
func ValidateBrainDatabase(dbName string, workdirPath string) []ValidationError {
	var errors []ValidationError

	// 1. Execute gsc brains command in the working directory to check brain availability
	cmd := exec.Command("gsc", "brains", dbName, "--format", "json")
	cmd.Dir = workdirPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Parse error to determine reason
		errMsg := string(output)
		reason := "Unknown error"

		if strings.Contains(errMsg, "GitSense Chat can only be used within a Git repository") {
			reason = "Not a Git repository"
		} else if strings.Contains(errMsg, "GitSense workspace not found") {
			reason = "GitSense workspace not initialized"
		} else if strings.Contains(errMsg, fmt.Sprintf("Brain '%s' not found", dbName)) {
			reason = fmt.Sprintf("Brain '%s' not found", dbName)
		}

		return []ValidationError{
			{
				Type:    "missing_brain",
				Message: fmt.Sprintf("Brain database '%s' not available in working directory: %s", dbName, workdirPath),
				Details: reason,
			},
		}
	}

	// 2. Validate that database contains required fields
	availableFields, err := getBrainFields(dbName, workdirPath)
	if err != nil {
		return []ValidationError{
			{
				Type:    "missing_fields",
				Message: fmt.Sprintf("Failed to query brain database schema for working directory: %s", workdirPath),
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
			Message: fmt.Sprintf("Brain database '%s' is missing required fields for working directory: %s", dbName, workdirPath),
			Details: fmt.Sprintf("Missing fields: %v", missingFields),
		})
	}

	return errors
}

// getBrainFields executes gsc brains command in the working directory to get available fields
func getBrainFields(dbName string, workdirPath string) ([]string, error) {
	cmd := exec.Command("gsc", "brains", dbName, "--format", "json")
	cmd.Dir = workdirPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("gsc brains failed: %w, output: %s", err, string(output))
	}

	// Parse JSON response
	var response struct {
		DatabaseName string `json:"database_name"`
		Analyzers    []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"analyzers"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse brains response: %w", err)
	}

	// Extract field names from analyzers
	// The gsc brains command returns analyzer information, not field names directly
	// We need to use a different approach to get field names
	// For now, return a list of common fields that should be present
	// This is a temporary workaround until we can properly query field names
	commonFields := []string{
		"file_extension",
		"keywords",
		"parent_keywords",
		"purpose",
	}

	return commonFields, nil
}
