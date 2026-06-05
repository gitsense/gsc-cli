/**
 * Component: Scout Setup and Configuration Validator
 * Block-UUID: 1ea48bdc-1711-45db-a996-3c3d025756c3
 * Parent-UUID: 2fb4fb22-0e35-4e41-9d03-47b35cb607c5
 * Version: 2.1.0
 * Description: Validates scout session prerequisites (working directories). Simplified to only require git repository validation. Removed brain database validation, RequiredFields, ValidateBrainDatabase, and getBrainFields functions as part of hybrid discovery strategy.
 * Language: Go
 * Created-at: 2026-05-02T16:27:32.076Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), claude-haiku-4-5-20251001 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
 */


package scout

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/gitsense/gsc-cli/internal/git"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Type    string // "invalid_workdir", "invalid_reference"
	Message string
	Details string
}

// ValidateSetup checks all prerequisites for a scout session
func ValidateSetup(workdirs []intent_workflow.WorkingDirectory, refFilesContext []intent_workflow.ReferenceFileContext) ([]ValidationError, error) {
	var errors []ValidationError

	// Validate working directories
	for _, wd := range workdirs {
		if errs, _ := ValidateWorkdir(wd); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return errors, nil
}

// ValidateWorkdir checks that a working directory is a valid git repository
func ValidateWorkdir(wd intent_workflow.WorkingDirectory) ([]ValidationError, error) {
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

	// Check if it's a git repository using internal/git package
	_, err = git.FindGitRootFrom(wd.Path)

	if err != nil {
		return []ValidationError{
			{
				Type:    "invalid_workdir",
				Message: fmt.Sprintf("Working directory is not a git repository: %s", wd.Name),
				Details: err.Error(),
			},
		}, nil
	}

	return errors, nil
}
