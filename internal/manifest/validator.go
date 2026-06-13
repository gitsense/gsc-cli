/**
 * Component: Manifest Validator
 * Block-UUID: dcaf816d-179b-4673-a630-ff2a0845b7fa
 * Parent-UUID: 8a7f9c12-3d45-4e8a-9b10-1c2d3e4f5a6b
 * Version: 1.2.0
 * Description: Validates the structure and content of a loaded ManifestFile.
 * Language: Go
 * Created-at: 2026-04-01T23:42:56.454Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), claude-haiku-4-5-20251001 (v1.2.0)
 */

package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

type ValidationResult struct {
	Valid         bool
	JSONValid     bool
	SchemaVersion string
	Errors        []ValidationError
	Warnings      []string
	Summary       ValidationSummary
}

type ValidationError struct {
	Field   string
	Message string
}

type ValidationSummary struct {
	FileCount     int
	FieldCount    int
	AnalyzerCount int
	RepoCount     int
	BranchCount   int
}

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

func ValidateManifestFile(path string, quiet bool) ValidationResult {
	result := ValidationResult{Valid: true}
	addError := func(field, message string) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{Field: field, Message: message})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		addError("", fmt.Sprintf("cannot read file: %v", err))
		return result
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		addError("", fmt.Sprintf("invalid JSON: %v", err))
		return result
	}
	result.JSONValid = true

	schemaVersion, ok := raw["schema_version"].(string)
	if !ok || schemaVersion == "" {
		addError("schema_version", "missing required field: schema_version")
	} else {
		result.SchemaVersion = schemaVersion
	}

	generatedAt, ok := raw["generated_at"].(string)
	if !ok || generatedAt == "" {
		addError("generated_at", "missing or invalid generated_at (expected ISO 8601)")
	} else if _, err := time.Parse(time.RFC3339, generatedAt); err != nil {
		addError("generated_at", "missing or invalid generated_at (expected ISO 8601)")
	}

	manifestObj, ok := raw["manifest"].(map[string]interface{})
	if !ok {
		addError("manifest", "missing required field: manifest")
	}
	if ok {
		name, _ := manifestObj["name"].(string)
		if name == "" {
			addError("manifest.name", "missing required field: manifest.name")
		}
	}

	repositories, repositoriesOK := raw["repositories"].([]interface{})
	if !repositoriesOK {
		addError("repositories", "repositories must be an array")
	}
	branches, branchesOK := raw["branches"].([]interface{})
	if !branchesOK {
		addError("branches", "branches must be an array")
	}
	analyzers, analyzersOK := raw["analyzers"].([]interface{})
	if !analyzersOK || len(analyzers) == 0 {
		addError("analyzers", "analyzers must be a non-empty array")
	}
	fields, fieldsOK := raw["fields"].([]interface{})
	if !fieldsOK || len(fields) == 0 {
		addError("fields", "fields must be a non-empty array")
	}
	records, recordsOK := raw["data"].([]interface{})
	if !recordsOK {
		addError("data", "data must be an array")
	}

	result.Summary = ValidationSummary{
		FileCount:     len(records),
		FieldCount:    len(fields),
		AnalyzerCount: len(analyzers),
		RepoCount:     len(repositories),
		BranchCount:   len(branches),
	}

	if !repositoriesOK || !branchesOK || !analyzersOK || !fieldsOK || !recordsOK {
		return addManifestWarnings(result, manifestObj, records, quiet)
	}

	repoRefs := collectRefs(repositories, "repositories", "repository", addError)
	branchRefs := collectRefs(branches, "branches", "branch", addError)
	analyzerRefs := collectAnalyzerRefs(analyzers, addError)
	fieldRefs := collectFieldRefs(fields, analyzerRefs, addError)
	chatIDs := make(map[int]bool)

	for i, item := range records {
		record, ok := item.(map[string]interface{})
		if !ok {
			addError(fmt.Sprintf("data[%d]", i), "data entry must be an object")
			continue
		}

		filePath, _ := record["file_path"].(string)
		if filePath == "" {
			addError(fmt.Sprintf("data[%d].file_path", i), "data entry file_path is empty")
		}

		chatID, ok := record["chat_id"].(float64)
		if !ok || chatID == 0 || chatID != float64(int(chatID)) {
			addError(fmt.Sprintf("data[%d].chat_id", i), "chat_id must be a unique non-zero integer")
		} else if chatIDs[int(chatID)] {
			addError(fmt.Sprintf("data[%d].chat_id", i), fmt.Sprintf("duplicate chat_id: %d", int(chatID)))
		} else {
			chatIDs[int(chatID)] = true
		}

		repoRef, _ := record["repo_ref"].(string)
		if repoRef != "" && !repoRefs[repoRef] {
			addError(fmt.Sprintf("data[%d].repo_ref", i), fmt.Sprintf("%q not found in repositories", repoRef))
		}

		branchRef, _ := record["branch_ref"].(string)
		if branchRef != "" && !branchRefs[branchRef] {
			addError(fmt.Sprintf("data[%d].branch_ref", i), fmt.Sprintf("%q not found in branches", branchRef))
		}

		recordFields, ok := record["fields"].(map[string]interface{})
		if !ok {
			addError(fmt.Sprintf("data[%d].fields", i), "fields must be an object")
			continue
		}
		for fieldRef := range recordFields {
			if !fieldRefs[fieldRef] {
				addError(fmt.Sprintf("data[%d].fields", i), fmt.Sprintf("key %q not found in fields", fieldRef))
			}
		}
	}

	return addManifestWarnings(result, manifestObj, records, quiet)
}

func collectRefs(items []interface{}, path, label string, addError func(string, string)) map[string]bool {
	refs := make(map[string]bool)
	for i, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			addError(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("%s entry must be an object", label))
			continue
		}
		ref, _ := obj["ref"].(string)
		if ref == "" {
			addError(fmt.Sprintf("%s[%d].ref", path, i), fmt.Sprintf("%s ref is empty", label))
			continue
		}
		if refs[ref] {
			addError(fmt.Sprintf("%s[%d].ref", path, i), fmt.Sprintf("duplicate %s ref: %s", label, ref))
		}
		refs[ref] = true
	}
	return refs
}

func collectAnalyzerRefs(items []interface{}, addError func(string, string)) map[string]bool {
	refs := make(map[string]bool)
	for i, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			addError(fmt.Sprintf("analyzers[%d]", i), "analyzer entry must be an object")
			continue
		}
		ref, _ := obj["ref"].(string)
		if ref == "" {
			addError(fmt.Sprintf("analyzers[%d].ref", i), "analyzer ref is empty")
		} else if refs[ref] {
			addError(fmt.Sprintf("analyzers[%d].ref", i), fmt.Sprintf("duplicate analyzer ref: %s", ref))
		} else {
			refs[ref] = true
		}
		id, _ := obj["id"].(string)
		if id == "" {
			addError(fmt.Sprintf("analyzers[%d].id", i), "analyzer id is empty")
		}
		name, _ := obj["name"].(string)
		if name == "" {
			addError(fmt.Sprintf("analyzers[%d].name", i), "analyzer name is empty")
		}
	}
	return refs
}

func collectFieldRefs(items []interface{}, analyzerRefs map[string]bool, addError func(string, string)) map[string]bool {
	refs := make(map[string]bool)
	validTypes := map[string]bool{"string": true, "number": true, "boolean": true, "array": true}
	for i, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			addError(fmt.Sprintf("fields[%d]", i), "field entry must be an object")
			continue
		}
		ref, _ := obj["ref"].(string)
		if ref == "" {
			addError(fmt.Sprintf("fields[%d].ref", i), "field ref is empty")
		} else if refs[ref] {
			addError(fmt.Sprintf("fields[%d].ref", i), fmt.Sprintf("duplicate field ref: %s", ref))
		} else {
			refs[ref] = true
		}

		analyzerRef, _ := obj["analyzer_ref"].(string)
		if analyzerRef == "" {
			addError(fmt.Sprintf("fields[%d].analyzer_ref", i), "field analyzer_ref is empty")
		} else if !analyzerRefs[analyzerRef] {
			addError(fmt.Sprintf("fields[%d].analyzer_ref", i), fmt.Sprintf("%q not found in analyzers", analyzerRef))
		}

		name, _ := obj["name"].(string)
		if name == "" {
			addError(fmt.Sprintf("fields[%d].name", i), "field name is empty")
		}
		fieldType, _ := obj["type"].(string)
		if !validTypes[fieldType] {
			addError(fmt.Sprintf("fields[%d].type", i), fmt.Sprintf("type is %q (expected string, number, boolean, or array)", fieldType))
		}
	}
	return refs
}

func addManifestWarnings(result ValidationResult, manifestObj map[string]interface{}, records []interface{}, quiet bool) ValidationResult {
	if quiet {
		return result
	}

	if manifestObj == nil {
		return result
	}
	databaseName, _ := manifestObj["database_name"].(string)
	if databaseName == "" {
		result.Warnings = append(result.Warnings, "database_name not set; import will use filename")
	}
	description, _ := manifestObj["description"].(string)
	if description == "" {
		result.Warnings = append(result.Warnings, "description not set")
	}
	tags, ok := manifestObj["tags"].([]interface{})
	if !ok || len(tags) == 0 {
		result.Warnings = append(result.Warnings, "no tags set")
	}

	missingLanguage := 0
	for _, item := range records {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		language, _ := record["language"].(string)
		if language == "" {
			missingLanguage++
		}
	}
	if missingLanguage > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d data entries have no language specified", missingLanguage))
	}

	return result
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
