/**
 * Component: Manifest Validator
 * Block-UUID: 8a7f9c12-3d45-4e8a-9b10-1c2d3e4f5a6b
 * Parent-UUID: 9533e908-8615-4ae9-aab6-72c7da54561a
 * Version: 1.1.0
 * Description: Validates the structure and content of a loaded ManifestFile.
 * Language: Go
 * Created-at: 2026-02-11T00:46:23.231Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package manifest

import (
	"fmt"
	"reflect"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

// ValidateManifest checks if the manifest file contains all required fields
// and has valid data structures.
func ValidateManifest(m *ManifestFile) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	// Check Schema Version
	if m.SchemaVersion == "" {
		return fmt.Errorf("schema_version is required")
	}

	// Check Manifest Info
	if m.Manifest.ManifestName == "" {
		return fmt.Errorf("manifest.name is required")
	}
	if m.Manifest.DatabaseName == "" {
		return fmt.Errorf("manifest.database_name is required")
	}

	// Check Repositories
	if len(m.Repositories) == 0 {
		return fmt.Errorf("at least one repository is required")
	}
	for i, repo := range m.Repositories {
		if repo.Ref == "" {
			return fmt.Errorf("repository[%d].ref is required", i)
		}
		if repo.Name == "" {
			return fmt.Errorf("repository[%d].name is required", i)
		}
	}

	// Check Branches
	if len(m.Branches) == 0 {
		return fmt.Errorf("at least one branch is required")
	}
	for i, branch := range m.Branches {
		if branch.Ref == "" {
			return fmt.Errorf("branch[%d].ref is required", i)
		}
		if branch.Name == "" {
			return fmt.Errorf("branch[%d].name is required", i)
		}
	}

	// Check Analyzers
	if len(m.Analyzers) == 0 {
		return fmt.Errorf("at least one analyzer is required")
	}
	for i, analyzer := range m.Analyzers {
		if analyzer.Ref == "" {
			return fmt.Errorf("analyzer[%d].ref is required", i)
		}
		if analyzer.ID == "" {
			return fmt.Errorf("analyzer[%d].id is required", i)
		}
	}

	// Check Fields
	if len(m.Fields) == 0 {
		return fmt.Errorf("at least one field is required")
	}
	for i, field := range m.Fields {
		if field.Ref == "" {
			return fmt.Errorf("field[%d].ref is required", i)
		}
		if field.Name == "" {
			return fmt.Errorf("field[%d].name is required", i)
		}
		if field.Type == "" {
			return fmt.Errorf("field[%d].type is required", i)
		}
	}

	// Check Data
	if len(m.Data) == 0 {
		logger.Info("Warning: manifest contains no data entries")
	}

	// Validate Data Entries
	for i, entry := range m.Data {
		if entry.FilePath == "" {
			return fmt.Errorf("data[%d].file_path is required", i)
		}
		if entry.ChatID == 0 {
			return fmt.Errorf("data[%d].chat_id is required", i)
		}
		if entry.Fields == nil {
			return fmt.Errorf("data[%d].fields is required", i)
		}
		
		// Ensure all fields in data entry map to defined field refs
		for fieldRef := range entry.Fields {
			found := false
			for _, definedField := range m.Fields {
				if definedField.Ref == fieldRef {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("data[%d].fields contains undefined ref: %s", i, fieldRef)
			}
		}
	}

	logger.Success("Manifest validation passed")
	return nil
}

// ValidateType checks if a value matches the expected type string.
func ValidateType(value interface{}, expectedType string) bool {
	if value == nil {
		return true // Nil is valid for any type (nullable)
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		// In JSON, numbers can be float64
		_, ok := value.(float64)
		return ok
	case "integer":
		// JSON numbers are float64, check if it's a whole number
		f, ok := value.(float64)
		return ok && f == float64(int(f))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		kind := reflect.TypeOf(value).Kind()
		return kind == reflect.Slice || kind == reflect.Array
	default:
		return false
	}
}
