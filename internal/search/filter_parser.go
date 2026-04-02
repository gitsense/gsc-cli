/**
 * Component: Filter Parser
 * Block-UUID: 2bd31fae-dd35-41a7-9cf5-f9b011af8026
 * Parent-UUID: fefe4dcb-68a4-47ee-8dc1-388be74200a1
 * Version: 1.0.18
 * Description: Parses filter strings and generates SQL WHERE clauses for metadata filtering. Supports operators, ranges, and field type detection. Fixed logic error in validateOperator for numeric fields. Fixed array filtering by adding alias 'AS je' to json_each calls and referencing 'je.value' instead of 'json_each.atom' to match insights implementation. Fixed missing 'AS je' alias in the 'in' operator case. Fixed SQL error by correcting table alias for field_name (fm_filter.field_name -> mf_filter.field_name). Fixed wildcard matching for array fields in 'in' operator by using consistent json_each alias pattern matching simple_querier.go and properly handling exact vs wildcard value separation.
 * Language: Go
 * Created-at: 2026-04-02T18:15:12.039Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), claude-haiku-4-5-20251001 (v1.0.3), claude-haiku-4-5-20251001 (v1.0.4), claude-haiku-4-5-20251001 (v1.0.5), GLM-4.7 (v1.0.6), claude-haiku-4-5-20251001 (v1.0.7), claude-haiku-4-5-20251001 (v1.0.8), claude-haiku-4-5-20251001 (v1.0.9), claude-haiku-4-5-20251001 (v1.0.10), claude-haiku-4-5-20251001 (v1.0.11), GLM-4.7 (v1.0.12), claude-haiku-4-5-20251001 (v1.0.13), claude-haiku-4-5-20251001 (v1.0.14), GLM-4.7 (v1.0.15), GLM-4.7 (v1.0.16), GLM-4.7 (v1.0.17), claude-haiku-4-5-20251001 (v1.0.18)
 */


package search

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
)

// ParseFilters parses a list of filter strings into FilterCondition objects.
// It validates field names and operators against the database schema.
func ParseFilters(ctx context.Context, filterStrings []string, dbName string) ([]FilterCondition, error) {
	if len(filterStrings) == 0 {
		return []FilterCondition{}, nil
	}

	// 1. Get Field Schema to determine types
	fieldTypes, err := db.GetFieldTypes(ctx, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get field schema: %w", err)
	}

	var conditions []FilterCondition

	// 2. Parse each filter string
	// Filters can be combined with semicolons (AND logic) or passed as separate flags
	for _, filterStr := range filterStrings {
		// Split by semicolon for AND logic within a single string
		parts := strings.Split(filterStr, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			cond, err := parseSingleFilter(part, fieldTypes)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, cond)
		}
	}

	return conditions, nil
}

// parseSingleFilter parses a single "field operator value" string.
func parseSingleFilter(filterStr string, fieldTypes map[string]string) (FilterCondition, error) {
	// Detect range syntax: "field=0..10"
	if strings.Contains(filterStr, "=") && strings.Contains(filterStr, "..") {
		return parseRangeFilter(filterStr, fieldTypes)
	}

	// Detect standard operators
	// Order matters: check longer operators first
	operators := []string{"!=", ">=", "<=", "!~", "~", "in", "not in", "=", ">", "<", "exists", "!exists"}
	
	var op string
	var field, value string

	for _, candidateOp := range operators {
		if strings.Contains(filterStr, candidateOp) {
			op = candidateOp
			parts := strings.SplitN(filterStr, candidateOp, 2)
			if len(parts) != 2 {
				// Handle exists/!exists which might not have a value
				if op == "exists" || op == "!exists" {
					field = strings.TrimSpace(parts[0])
					value = ""
				} else {
					return FilterCondition{}, fmt.Errorf("invalid filter syntax: %s", filterStr)
				}
			} else {
				field = strings.TrimSpace(parts[0])
				value = strings.TrimSpace(parts[1])
			}
			break
		}
	}

	if op == "" {
		return FilterCondition{}, fmt.Errorf("unknown operator in filter: %s", filterStr)
	}

	// Validate Field Name
	if _, exists := fieldTypes[field]; !exists {
		// Special case: file_path is a system field, not in metadata_fields
		if field != "file_path" {
			return FilterCondition{}, fmt.Errorf("unknown field '%s'. Available fields: %s", field, getAvailableFields(fieldTypes))
		}
	}

	// Validate Operator vs Field Type
	if err := validateOperator(field, op, value, fieldTypes); err != nil {
		return FilterCondition{}, err
	}

	// Strip parentheses from 'in' and 'not in' operators (e.g., "in (a,b)" → "a,b")
	if op == "in" || op == "not in" {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
			value = value[1 : len(value)-1]
		}
	}

	return FilterCondition{
		Field:    field,
		Operator: op,
		Value:    value,
	}, nil
}

// parseRangeFilter handles "field=0..10" syntax.
func parseRangeFilter(filterStr string, fieldTypes map[string]string) (FilterCondition, error) {
	parts := strings.SplitN(filterStr, "=", 2)
	if len(parts) != 2 {
		return FilterCondition{}, fmt.Errorf("invalid range syntax: %s", filterStr)
	}

	field := strings.TrimSpace(parts[0])
	rangeStr := strings.TrimSpace(parts[1])

	// Validate field
	if _, exists := fieldTypes[field]; !exists && field != "file_path" {
		return FilterCondition{}, fmt.Errorf("unknown field '%s'", field)
	}

	// Validate range format
	rangeParts := strings.Split(rangeStr, "..")
	if len(rangeParts) != 2 {
		return FilterCondition{}, fmt.Errorf("invalid range format, expected 'min..max': %s", rangeStr)
	}

	min := strings.TrimSpace(rangeParts[0])
	max := strings.TrimSpace(rangeParts[1])

	// Validate numeric values
	if _, err := strconv.ParseFloat(min, 64); err != nil {
		return FilterCondition{}, fmt.Errorf("range min must be numeric: %s", min)
	}
	if _, err := strconv.ParseFloat(max, 64); err != nil {
		return FilterCondition{}, fmt.Errorf("range max must be numeric: %s", max)
	}

	// Store as a special operator with value "min..max"
	return FilterCondition{
		Field:    field,
		Operator: "range",
		Value:    fmt.Sprintf("%s..%s", min, max),
	}, nil
}

// validateOperator checks if the operator is valid for the field type.
func validateOperator(field, op, value string, fieldTypes map[string]string) error {
	// System fields
	if field == "file_path" {
		if op != "=" && op != "!=" && op != "~" && op != "!~" {
			return fmt.Errorf("operator '%s' not supported for file_path", op)
		}
		return nil
	}

	fieldType, exists := fieldTypes[field]
	if !exists {
		return nil // Already checked in parseSingleFilter
	}

	// List fields (e.g., topics, keywords)
	// Stored as JSON arrays or comma-separated strings
	if fieldType == "list" {
		if op == ">" || op == "<" || op == ">=" || op == "<=" {
			return fmt.Errorf("numeric operators not supported for list field '%s'. Use 'in', '=', or '!='", field)
		}
	}

	// Numeric fields
	if fieldType == "number" {
		if op == "in" || op == "not in" || op == "~" || op != "!~" {
			return fmt.Errorf("string operators not supported for numeric field '%s'. Use =, !=, >, <, >=, <=", field)
		}
	}

	return nil
}

// BuildSQLWhereClause constructs the SQL WHERE clause and arguments from filter conditions.
// It also handles system filters (analyzed status, file paths).
func BuildSQLWhereClause(conditions []FilterCondition, analyzedFilter string, filePatterns []string) (string, []interface{}, error) {
	// Note: This function requires the database context to be available.
	// If called without database context, use BuildSQLWhereClauseWithTypes directly.
	// For now, we pass nil and let buildConditionSQL handle missing field types.
	return BuildSQLWhereClauseWithTypes(conditions, analyzedFilter, filePatterns, nil)
}

// BuildSQLWhereClauseWithDB retrieves field types from the database and builds the WHERE clause.
// This is the recommended approach for use in CLI commands where database context is available.
func BuildSQLWhereClauseWithDB(ctx context.Context, conditions []FilterCondition, analyzedFilter string, filePatterns []string, dbName string) (string, []interface{}, error) {
	// Retrieve field types from the database
	fieldTypes, err := db.GetFieldTypes(ctx, dbName)
	if err != nil {
		// If we can't get field types, fall back to the original behavior
		// (this ensures backward compatibility and doesn't break queries)
		fieldTypes = make(map[string]string)
	}
	
	// Build WHERE clause with field type information
	return BuildSQLWhereClauseWithTypes(conditions, analyzedFilter, filePatterns, fieldTypes)
}

// BuildSQLWhereClauseWithTypes constructs the SQL WHERE clause with field type information.
// This is the main implementation that handles both scalar and array fields.
func BuildSQLWhereClauseWithTypes(conditions []FilterCondition, analyzedFilter string, filePatterns []string, fieldTypes map[string]string) (string, []interface{}, error) {
	var whereParts []string
	var args []interface{}

	// 1. Add Metadata Filters
	for _, cond := range conditions {
		sqlPart, condArgs, err := buildConditionSQL(cond, fieldTypes)
		if err != nil {
			return "", nil, err
		}
		whereParts = append(whereParts, sqlPart)
		args = append(args, condArgs...)
	}

	// 2. Add Analyzed Filter (System Filter)
	if analyzedFilter != "" && analyzedFilter != "all" {
		if analyzedFilter == "true" {
			whereParts = append(whereParts, "f.chat_id IS NOT NULL")
		} else if analyzedFilter == "false" {
			whereParts = append(whereParts, "f.chat_id IS NULL")
		}
	}

	// 3. Add File Path Filters (System Filter - OR logic)
	if len(filePatterns) > 0 {
		var fileParts []string
		for _, pattern := range filePatterns {
			// Convert glob pattern to SQL LIKE
			// "internal/*" -> "internal/%"
			// "pkg/auth/*" -> "pkg/auth/%"
			sqlPattern := strings.ReplaceAll(pattern, "*", "%")
			fileParts = append(fileParts, "f.file_path LIKE ?")
			args = append(args, sqlPattern)
		}
		if len(fileParts) > 0 {
			whereParts = append(whereParts, "("+strings.Join(fileParts, " OR ")+")")
		}
	}

	// Combine all parts
	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = "WHERE " + strings.Join(whereParts, " AND ")
	}

	return whereClause, args, nil
}

// buildConditionSQL generates SQL for a single FilterCondition.
func buildConditionSQL(cond FilterCondition, fieldTypes map[string]string) (string, []interface{}, error) {
	// Handle System Fields
	if cond.Field == "file_path" {
		return buildFileConditionSQL(cond)
	}

	// Get field type
	fieldType, exists := fieldTypes[cond.Field]
	if !exists {
		// Default to scalar if not found
		fieldType = "string"
	}

	// Check if this is an array field
	isArray := (fieldType == "array" || fieldType == "list")

	// For array fields, wrap in EXISTS clause
	if isArray {
		return buildArrayConditionSQL(cond)
	}

	// For scalar fields, use existing logic
	return buildScalarConditionSQL(cond)
}

// buildArrayConditionSQL generates SQL for array/list fields using json_each.
func buildArrayConditionSQL(cond FilterCondition) (string, []interface{}, error) {
	switch cond.Operator {
	case "=":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je WHERE LOWER(je.value) LIKE ?))", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "!=":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je WHERE LOWER(je.value) NOT LIKE ?))", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "in":
		values := strings.Split(cond.Value, ",")
		var exactValues []string
		var wildcardValues []string
		var args []interface{}
		args = append(args, cond.Field)

		for _, v := range values {
			v = strings.ToLower(strings.TrimSpace(v))

			if strings.Contains(v, "*") {
				// Wildcard match: convert * to %, append to args immediately
				pattern := strings.ReplaceAll(v, "*", "%")
				wildcardValues = append(wildcardValues, "LOWER(je_filter.value) LIKE ?")
				args = append(args, pattern)
			} else {
				// Exact match: collect placeholder and append to args immediately
				exactValues = append(exactValues, "?")
				args = append(args, v)
			}
		}

		var conditions []string
		if len(exactValues) > 0 {
			conditions = append(conditions, "LOWER(je_filter.value) IN ("+strings.Join(exactValues, ",")+")")
		}
		if len(wildcardValues) > 0 {
			conditions = append(conditions, strings.Join(wildcardValues, " OR "))
		}

		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je_filter WHERE " + strings.Join(conditions, " OR ") + "))", args, nil

	case "not in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = strings.ToLower(strings.TrimSpace(v))
		}
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je WHERE LOWER(je.value) NOT IN (" + strings.Join(placeholders, ", ") + ")))", append([]interface{}{cond.Field}, args...), nil

	case "~":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je WHERE LOWER(je.value) LIKE ?))", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "!~":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je WHERE LOWER(je.value) NOT LIKE ?))", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "exists":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je))", []interface{}{cond.Field}, nil

	case "!exists":
		return "NOT EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND json_valid(fm_filter.field_value) AND EXISTS (SELECT 1 FROM json_each(fm_filter.field_value) AS je))", []interface{}{cond.Field}, nil

	default:
		return "", nil, fmt.Errorf("operator '%s' not supported for array fields", cond.Operator)
	}
}

// buildScalarConditionSQL generates SQL for scalar fields (original logic).
func buildScalarConditionSQL(cond FilterCondition) (string, []interface{}, error) {
	switch cond.Operator {
	case "=":
		// For scalar fields, match by field name AND value to avoid cross-field pollution
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND LOWER(fm_filter.field_value) LIKE ?)", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "!=":
		return "NOT EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND LOWER(fm_filter.field_value) LIKE ?)", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "in":
		values := strings.Split(cond.Value, ",")
		var exactValues []string
		var wildcardValues []string
		args := make([]interface{}, len(values))
		argCount := 0

		for _, v := range values {
			v = strings.ToLower(strings.TrimSpace(v))

			if strings.Contains(v, "*") {
				// Wildcard match: convert * to %
				pattern := strings.ReplaceAll(v, "*", "%")
				wildcardValues = append(wildcardValues, "LOWER(fm_filter.field_value) LIKE ?")
				args[argCount] = pattern
				argCount++
			} else {
				// Exact match
				exactValues = append(exactValues, "?")
				args[argCount] = v
				argCount++
			}
		}

		// Trim args to actual count
		args = append([]interface{}{cond.Field}, args[:argCount]...)

		var conditions []string
		if len(exactValues) > 0 {
			conditions = append(conditions, "LOWER(fm_filter.field_value) IN ("+strings.Join(exactValues, ",")+")")
		}
		if len(wildcardValues) > 0 {
			conditions = append(conditions, strings.Join(wildcardValues, " OR "))
		}

		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND (" + strings.Join(conditions, " OR ") + "))", args, nil

	case "not in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = strings.ToLower(strings.TrimSpace(v))
		}
		return "NOT EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND LOWER(fm_filter.field_value) IN (" + strings.Join(placeholders, ",") + "))", append([]interface{}{cond.Field}, args...), nil

	case "~":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND LOWER(fm_filter.field_value) LIKE ?)", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case "!~":
		return "NOT EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND LOWER(fm_filter.field_value) LIKE ?)", []interface{}{cond.Field, "%" + strings.ToLower(cond.Value) + "%"}, nil

	case ">", "<", ">=", "<=":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND CAST(fm_filter.field_value AS REAL) " + cond.Operator + " ?)", []interface{}{cond.Field, cond.Value}, nil

	case "exists":
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND fm_filter.field_value IS NOT NULL)", []interface{}{cond.Field}, nil

	case "!exists":
		return "NOT EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND fm_filter.field_value IS NOT NULL)", []interface{}{cond.Field}, nil

	case "range":
		parts := strings.Split(cond.Value, "..")
		return "EXISTS (SELECT 1 FROM file_metadata fm_filter JOIN metadata_fields mf_filter ON fm_filter.field_id = mf_filter.field_id WHERE fm_filter.file_path = f.file_path AND mf_filter.field_name = ? AND CAST(fm_filter.field_value AS REAL) BETWEEN ? AND ?)", []interface{}{cond.Field, parts[0], parts[1]}, nil

	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", cond.Operator)
	}
}

// buildFileConditionSQL generates SQL for file_path filtering.
func buildFileConditionSQL(cond FilterCondition) (string, []interface{}, error) {
	switch cond.Operator {
	case "=":
		return "f.file_path = ?", []interface{}{cond.Value}, nil
	case "!=":
		return "f.file_path != ?", []interface{}{cond.Value}, nil
	case "~":
		return "f.file_path LIKE ?", []interface{}{"%" + cond.Value + "%"}, nil
	case "!~":
		return "f.file_path NOT LIKE ?", []interface{}{"%" + cond.Value + "%"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator for file_path: %s", cond.Operator)
	}
}

// getAvailableFields returns a comma-separated string of available field names.
func getAvailableFields(fieldTypes map[string]string) string {
	fields := make([]string, 0, len(fieldTypes))
	for f := range fieldTypes {
		fields = append(fields, f)
	}
	return strings.Join(fields, ", ")
}
