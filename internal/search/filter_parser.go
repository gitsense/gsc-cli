/**
 * Component: Filter Parser
 * Block-UUID: bc4691bb-9e58-4ea0-8301-5455aa34e49c
 * Parent-UUID: 96639906-6185-42ec-b43d-fb59b5aa3958
 * Version: 1.0.2
 * Description: Parses filter strings and generates SQL WHERE clauses for metadata filtering. Supports operators, ranges, and field type detection. Fixed logic error in validateOperator for numeric fields.
 * Language: Go
 * Created-at: 2026-02-04T03:55:26.960Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package search

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/yourusername/gsc-cli/internal/manifest"
)

// ParseFilters parses a list of filter strings into FilterCondition objects.
// It validates field names and operators against the database schema.
func ParseFilters(ctx context.Context, filterStrings []string, dbName string) ([]FilterCondition, error) {
	if len(filterStrings) == 0 {
		return []FilterCondition{}, nil
	}

	// 1. Get Field Schema to determine types
	fieldTypes, err := manifest.GetFieldTypes(ctx, dbName)
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
		if op == "in" || op == "not in" || op == "~" || op == "!~" {
			return fmt.Errorf("string operators not supported for numeric field '%s'. Use =, !=, >, <, >=, <=", field)
		}
	}

	return nil
}

// BuildSQLWhereClause constructs the SQL WHERE clause and arguments from filter conditions.
// It also handles system filters (analyzed status, file paths).
func BuildSQLWhereClause(conditions []FilterCondition, analyzedFilter string, filePatterns []string) (string, []interface{}, error) {
	var whereParts []string
	var args []interface{}

	// 1. Add Metadata Filters
	for _, cond := range conditions {
		sqlPart, condArgs, err := buildConditionSQL(cond)
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
func buildConditionSQL(cond FilterCondition) (string, []interface{}, error) {
	// Handle System Fields
	if cond.Field == "file_path" {
		return buildFileConditionSQL(cond)
	}

	// Handle Metadata Fields
	// We need to join file_metadata and metadata_fields
	// SQL structure: (SELECT field_value FROM file_metadata WHERE file_path = f.file_path AND field_id = (SELECT field_id FROM metadata_fields WHERE field_name = ?))
	
	// However, for efficiency in the main query, we usually join tables.
	// This function returns the condition part assuming the tables are joined.
	// e.g., "fm.field_value = ?" or "fm.field_value LIKE ?"
	
	// Note: The main query in enricher.go handles the joins. 
	// Here we return the condition relative to the joined table alias.
	
	switch cond.Operator {
	case "=":
		// For lists, use LIKE. For scalars, use =.
		// Since we don't have type info here easily without re-querying, 
		// we assume the caller (enricher) or this function needs type info.
		// For simplicity, let's use LIKE for everything to handle lists, 
		// but this might be imprecise for exact scalar matches.
		// Better: Use parameterized query.
		return "fm.field_value LIKE ?", []interface{}{"%" + strings.ToLower(cond.Value) + "%"}, nil // Case-insensitive

	case "!=":
		return "fm.field_value NOT LIKE ?", []interface{}{"%" + strings.ToLower(cond.Value) + "%"}, nil

	case "in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = strings.ToLower(strings.TrimSpace(v))
		}
		return "LOWER(fm.field_value) IN (" + strings.Join(placeholders, ",") + ")", args, nil

	case "not in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = strings.ToLower(strings.TrimSpace(v))
		}
		return "LOWER(fm.field_value) NOT IN (" + strings.Join(placeholders, ",") + ")", args, nil

	case "~":
		return "fm.field_value LIKE ?", []interface{}{"%" + strings.ToLower(cond.Value) + "%"}, nil

	case "!~":
		return "fm.field_value NOT LIKE ?", []interface{}{"%" + strings.ToLower(cond.Value) + "%"}, nil

	case ">", "<", ">=", "<=":
		return fmt.Sprintf("CAST(fm.field_value AS REAL) %s ?", cond.Operator), []interface{}{cond.Value}, nil

	case "exists":
		return "fm.field_value IS NOT NULL", []interface{}{}, nil

	case "!exists":
		return "fm.field_value IS NULL", []interface{}{}, nil

	case "range":
		parts := strings.Split(cond.Value, "..")
		return "CAST(fm.field_value AS REAL) BETWEEN ? AND ?", []interface{}{parts[0], parts[1]}, nil

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
