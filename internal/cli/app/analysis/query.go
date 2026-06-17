/**
 * Component: Analysis Query Command
 * Block-UUID: ac38af0d-556e-4b40-a875-e2c39795c7c3
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app analysis query' command for querying analysis metadata. Supports filter expressions like "field=value", "field~contains", "field>value". Queries the chat database using json_extract() on messages.meta.extracted_metadata.
 * Language: Go
 * Created-at: 2026-06-16T15:00:00.000Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package analysis

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	flagQueryAnalyzer string
	flagQueryFormat   string
	flagQueryQuiet    bool
	flagQueryLimit    int
	flagQueryDebug    bool
	flagQueryRefChatID int64
	flagQueryOwner    string
	flagQueryRepo     string
	flagQueryBranch   string
	flagQueryFilesOnly bool
)

// QueryCmd represents the 'gsc app analysis query' command.
var QueryCmd = &cobra.Command{
	Use:   "query <expression>",
	Short: "Query analysis metadata by field values",
	Long: `Queries the analysis database to find files matching metadata field criteria.

Supports filter expressions with the following operators:
  =       Equals (or numeric range with '..')
  !=      Not equals
  ~       Contains (substring match)
  !~      Not contains
  >       Greater than (numeric)
  <       Less than (numeric)
  >=      Greater than or equal
  <=      Less than or equal
  in      List membership (OR logic)
  exists  Field is not null
  !exists Field is null

Multiple filters can be combined with semicolons (;) for AND logic.

Examples:
  # Find files where has_dynamic_event_emissions is true (auto-detect branch)
  gsc app analysis query --analyzer typescript-event-coupling "has_dynamic_event_emissions=true"

  # Specify repository and branch explicitly
  gsc app analysis query --analyzer typescript-event-coupling --owner gitsense --repo smart-pi --branch main "has_dynamic_event_emissions=true"

  # Find high confidence files
  gsc app analysis query --analyzer typescript-event-coupling "confidence=high"

  # Find files with non-empty emits_events array
  gsc app analysis query --analyzer typescript-event-coupling "emits_events!=[]"

  # Find files where event_emission_apis is null
  gsc app analysis query --analyzer typescript-event-coupling "event_emission_apis=null"

  # Same as above using exists syntax
  gsc app analysis query --analyzer typescript-event-coupling "event_emission_apis!exists"

  # Find files where event_emission_apis is NOT null
  gsc app analysis query --analyzer typescript-event-coupling "event_emission_apis!=null"
  gsc app analysis query --analyzer typescript-event-coupling "event_emission_apis exists"

  # Find TypeScript files with dynamic emissions
  gsc app analysis query --analyzer typescript-event-coupling "language=typescript;has_dynamic_event_emissions=true"

  # Find files that handle specific events (substring match)
  gsc app analysis query --analyzer typescript-event-coupling "handles_events~session_start"

  # Find files with low or medium confidence
  gsc app analysis query --analyzer typescript-event-coupling "confidence in (low,medium)"

  # Output as JSON
  gsc app analysis query --analyzer typescript-event-coupling "confidence=high" --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

func init() {
	QueryCmd.Flags().StringVar(&flagQueryAnalyzer, "analyzer", "", "Analyzer name (required)")
	_ = QueryCmd.MarkFlagRequired("analyzer")

	QueryCmd.Flags().StringVar(&flagQueryFormat, "format", "table", "Output format: json, jsonl, table")
	QueryCmd.Flags().BoolVar(&flagQueryQuiet, "quiet", false, "Suppress headers and hints")
	QueryCmd.Flags().IntVar(&flagQueryLimit, "limit", 0, "Maximum number of results to return (0 = unlimited)")
	QueryCmd.Flags().BoolVar(&flagQueryDebug, "debug", false, "Show SQL query and diagnostic information")
	QueryCmd.Flags().Int64Var(&flagQueryRefChatID, "ref-chat-id", 0, "Branch ref chat ID for disambiguation")
	QueryCmd.Flags().StringVar(&flagQueryOwner, "owner", "", "Repository owner (overrides auto-detection)")
	QueryCmd.Flags().StringVar(&flagQueryRepo, "repo", "", "Repository name (overrides auto-detection)")
	QueryCmd.Flags().StringVar(&flagQueryBranch, "branch", "", "Branch name (overrides auto-detection)")
	QueryCmd.Flags().BoolVar(&flagQueryFilesOnly, "files-only", false, "Output only file paths (one per line)")
}

// FilterOperator represents the type of filter operation.
type FilterOperator string

const (
	OpEquals        FilterOperator = "="
	OpNotEquals     FilterOperator = "!="
	OpContains      FilterOperator = "~"
	OpNotContains   FilterOperator = "!~"
	OpGreater       FilterOperator = ">"
	OpLess          FilterOperator = "<"
	OpGreaterEquals FilterOperator = ">="
	OpLessEquals    FilterOperator = "<="
	OpIn            FilterOperator = "in"
	OpExists        FilterOperator = "exists"
	OpNotExists     FilterOperator = "!exists"
)

// AnalysisFilter represents a parsed filter condition.
type AnalysisFilter struct {
	Field    string
	Operator FilterOperator
	Value    string
	Values   []string // For 'in' operator
}

// QueryResult represents a single query result.
type QueryResult struct {
	FilePath string                 `json:"file_path"`
	ChatID   int64                  `json:"chat_id"`
	Metadata map[string]interface{} `json:"metadata"`
}

// runQuery is the main entry point for the query command.
func runQuery(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	expression := args[0]

	// Parse the filter expression
	filters, err := parseAnalysisFilter(expression)
	if err != nil {
		return fmt.Errorf("invalid filter expression: %w", err)
	}

	// Open database
	dbConn, err := openChatDB()
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// Resolve branch context
	refChatID, err := resolveRefChatID(dbConn, flagQueryRefChatID, flagQueryOwner, flagQueryRepo, flagQueryBranch)
	if err != nil {
		return fmt.Errorf("could not determine branch context: %w\n\nRun 'gsc import' first or use --owner/--repo/--branch flags", err)
	}

	if flagQueryDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG] refChatID: %d\n", refChatID)
		fmt.Fprintf(os.Stderr, "[DEBUG] analyzer: %s\n", flagQueryAnalyzer)
		fmt.Fprintf(os.Stderr, "[DEBUG] expression: %s\n", expression)
		fmt.Fprintf(os.Stderr, "[DEBUG] filters: %+v\n", filters)

		// Count total records for this analyzer (before filtering)
		analyzerPattern := flagQueryAnalyzer
		if !strings.Contains(flagQueryAnalyzer, "::") {
			analyzerPattern = flagQueryAnalyzer + "::%"
		}
		var totalCount int
		countQuery := `SELECT COUNT(*) FROM chats c
			JOIN messages m ON m.chat_id = c.id AND m.type LIKE ? AND m.deleted = 0
			WHERE c.type = 'git-blob' AND c.deleted = 0
			AND json_extract(c.meta, '$.refContext.refChatId') = ?`
		err = dbConn.QueryRow(countQuery, analyzerPattern, refChatID).Scan(&totalCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Count query error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Total records for analyzer '%s' with refChatID %d: %d\n", flagQueryAnalyzer, refChatID, totalCount)
		}
	}

	// Execute query
	results, err := executeAnalysisQuery(dbConn, refChatID, flagQueryAnalyzer, filters, flagQueryLimit)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Output results
	return outputQueryResults(results)
}

// parseAnalysisFilter parses a filter expression string into AnalysisFilter objects.
func parseAnalysisFilter(expression string) ([]AnalysisFilter, error) {
	var filters []AnalysisFilter

	// Split by semicolons for AND logic
	parts := strings.Split(expression, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		filter, err := parseSingleAnalysisFilter(part)
		if err != nil {
			return nil, fmt.Errorf("failed to parse '%s': %w", part, err)
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

// parseSingleAnalysisFilter parses a single "field operator value" string.
func parseSingleAnalysisFilter(expr string) (AnalysisFilter, error) {
	// Define operator patterns (order matters - longer operators first)
	operatorPatterns := []struct {
		op      FilterOperator
		pattern string
	}{
		{OpNotContains, `!~`},
		{OpNotEquals, `!=`},
		{OpGreaterEquals, `>=`},
		{OpLessEquals, `<=`},
		{OpContains, `~`},
		{OpGreater, `>`},
		{OpLess, `<`},
		{OpEquals, `=`},
	}

	// Try each operator pattern
	for _, opPattern := range operatorPatterns {
		idx := strings.Index(expr, string(opPattern.op))
		if idx > 0 {
			field := strings.TrimSpace(expr[:idx])
			value := strings.TrimSpace(expr[idx+len(string(opPattern.op)):])

			// Handle 'in' operator specially
			if opPattern.op == OpEquals && strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
				// Parse as 'in' operator
				inner := strings.TrimPrefix(value, "(")
				inner = strings.TrimSuffix(inner, ")")
				values := strings.Split(inner, ",")
				for i := range values {
					values[i] = strings.TrimSpace(values[i])
				}
				return AnalysisFilter{
					Field:    field,
					Operator: OpIn,
					Values:   values,
				}, nil
			}

			return AnalysisFilter{
				Field:    field,
				Operator: opPattern.op,
				Value:    value,
			}, nil
		}
	}

	// Also check for 'in' operator with spaces
	inRegex := regexp.MustCompile(`^(\w+)\s+in\s+\((.+)\)$`)
	if matches := inRegex.FindStringSubmatch(expr); matches != nil {
		field := strings.TrimSpace(matches[1])
		valuesStr := strings.TrimSpace(matches[2])
		values := strings.Split(valuesStr, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		return AnalysisFilter{
			Field:    field,
			Operator: OpIn,
			Values:   values,
		}, nil
	}

	// Check for 'exists' and '!exists' operators
	existsRegex := regexp.MustCompile(`^(\w+)\s*(!)?exists$`)
	if matches := existsRegex.FindStringSubmatch(expr); matches != nil {
		field := strings.TrimSpace(matches[1])
		op := OpExists
		if matches[2] == "!" {
			op = OpNotExists
		}
		return AnalysisFilter{
			Field:    field,
			Operator: op,
		}, nil
	}

	return AnalysisFilter{}, fmt.Errorf("no valid operator found in expression '%s'", expr)
}

// executeAnalysisQuery executes the query against the chat database.
func executeAnalysisQuery(dbConn *sql.DB, refChatID int64, analyzer string, filters []AnalysisFilter, limit int) ([]QueryResult, error) {
	// Build analyzer pattern
	analyzerPattern := analyzer
	if !strings.Contains(analyzer, "::") {
		analyzerPattern = analyzer + "::%"
	}

	// Build base query
	baseQuery := `
		SELECT
			json_extract(c.meta, '$.path') as file_path,
			c.id as chat_id,
			m.meta
		FROM chats c
		JOIN messages m ON m.chat_id = c.id
			AND m.type LIKE ?
			AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`

	args := []interface{}{analyzerPattern, refChatID}

	// Add filter conditions
	for _, filter := range filters {
		condition, conditionArgs := buildFilterCondition(filter)
		baseQuery += " AND " + condition
		args = append(args, conditionArgs...)
	}

	// Add ordering and limit
	baseQuery += " ORDER BY file_path ASC"
	if limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", limit)
	}

	if flagQueryDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG] SQL Query:\n%s\n", baseQuery)
		fmt.Fprintf(os.Stderr, "[DEBUG] Args: %v\n", args)
	}

	// Execute query
	rows, err := dbConn.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Process results
	var results []QueryResult
	for rows.Next() {
		var filePath string
		var chatID int64
		var metaJSON sql.NullString

		if err := rows.Scan(&filePath, &chatID, &metaJSON); err != nil {
			logger.Warning("Failed to scan query row", "error", err)
			continue
		}

		result := QueryResult{
			FilePath: filePath,
			ChatID:   chatID,
		}

		// Parse metadata
		if metaJSON.Valid {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
				if em, ok := meta["extracted_metadata"].(map[string]interface{}); ok {
					result.Metadata = em
				}
			}
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating query rows: %w", err)
	}

	return results, nil
}

// buildFilterCondition builds a SQL WHERE condition from a filter.
func buildFilterCondition(filter AnalysisFilter) (string, []interface{}) {
	// Use json_extract() on the nested extracted_metadata
	fieldPath := fmt.Sprintf("json_extract(m.meta, '$.extracted_metadata.%s')", filter.Field)

	switch filter.Operator {
	case OpEquals:
		// Handle null check
		if filter.Value == "null" {
			return fmt.Sprintf("%s IS NULL", fieldPath), nil
		}
		// Handle boolean values - SQLite stores JSON booleans as integers (1=true, 0=false)
		if filter.Value == "true" {
			return fmt.Sprintf("%s = ?", fieldPath), []interface{}{1}
		}
		if filter.Value == "false" {
			return fmt.Sprintf("%s = ?", fieldPath), []interface{}{0}
		}
		// Handle numeric values
		if isNumeric(filter.Value) {
			return fmt.Sprintf("CAST(%s AS REAL) = ?", fieldPath), []interface{}{filter.Value}
		}
		// String equality
		return fmt.Sprintf("%s = ?", fieldPath), []interface{}{filter.Value}

	case OpNotEquals:
		// Handle null check
		if filter.Value == "null" {
			return fmt.Sprintf("%s IS NOT NULL", fieldPath), nil
		}
		// Handle boolean values - SQLite stores JSON booleans as integers (1=true, 0=false)
		if filter.Value == "true" {
			return fmt.Sprintf("(%s != ? OR %s IS NULL)", fieldPath, fieldPath), []interface{}{1}
		}
		if filter.Value == "false" {
			return fmt.Sprintf("(%s != ? OR %s IS NULL)", fieldPath, fieldPath), []interface{}{0}
		}
		return fmt.Sprintf("(%s != ? OR %s IS NULL)", fieldPath, fieldPath), []interface{}{filter.Value}

	case OpContains:
		return fmt.Sprintf("%s LIKE ?", fieldPath), []interface{}{"%" + filter.Value + "%"}

	case OpNotContains:
		return fmt.Sprintf("(%s NOT LIKE ? OR %s IS NULL)", fieldPath, fieldPath), []interface{}{"%" + filter.Value + "%"}

	case OpGreater:
		return fmt.Sprintf("CAST(%s AS REAL) > ?", fieldPath), []interface{}{filter.Value}

	case OpLess:
		return fmt.Sprintf("CAST(%s AS REAL) < ?", fieldPath), []interface{}{filter.Value}

	case OpGreaterEquals:
		return fmt.Sprintf("CAST(%s AS REAL) >= ?", fieldPath), []interface{}{filter.Value}

	case OpLessEquals:
		return fmt.Sprintf("CAST(%s AS REAL) <= ?", fieldPath), []interface{}{filter.Value}

	case OpIn:
		placeholders := make([]string, len(filter.Values))
		inArgs := make([]interface{}, len(filter.Values))
		for i, v := range filter.Values {
			placeholders[i] = "?"
			// Handle boolean values in IN clause
			if v == "true" {
				inArgs[i] = 1
			} else if v == "false" {
				inArgs[i] = 0
			} else {
				inArgs[i] = v
			}
		}
		return fmt.Sprintf("%s IN (%s)", fieldPath, strings.Join(placeholders, ",")), inArgs

	case OpExists:
		return fmt.Sprintf("%s IS NOT NULL", fieldPath), nil

	case OpNotExists:
		return fmt.Sprintf("%s IS NULL", fieldPath), nil

	default:
		// Default to equality
		return fmt.Sprintf("%s = ?", fieldPath), []interface{}{filter.Value}
	}
}

// isNumeric checks if a string represents a numeric value.
func isNumeric(s string) bool {
	_, err := fmt.Sscanf(s, "%f", new(float64))
	return err == nil
}

// outputQueryResults outputs the query results in the requested format.
func outputQueryResults(results []QueryResult) error {
	if flagQueryFilesOnly {
		return outputQueryResultsFilesOnly(results)
	}

	switch flagQueryFormat {
	case "json":
		return outputQueryResultsJSON(results)
	case "jsonl":
		return outputQueryResultsJSONL(results)
	default:
		return outputQueryResultsTable(results)
	}
}

// outputQueryResultsJSON outputs results as a JSON array.
func outputQueryResultsJSON(results []QueryResult) error {
	output := map[string]interface{}{
		"total_results": len(results),
		"results":       results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputQueryResultsJSONL outputs results as newline-delimited JSON.
func outputQueryResultsJSONL(results []QueryResult) error {
	encoder := json.NewEncoder(os.Stdout)
	for _, r := range results {
		if err := encoder.Encode(r); err != nil {
			return fmt.Errorf("failed to encode JSONL record: %w", err)
		}
	}
	return nil
}

// outputQueryResultsFilesOnly outputs only file paths, one per line.
func outputQueryResultsFilesOnly(results []QueryResult) error {
	for _, r := range results {
		fmt.Println(r.FilePath)
	}
	return nil
}

// outputQueryResultsTable outputs results as a human-readable table.
func outputQueryResultsTable(results []QueryResult) error {
	if len(results) == 0 {
		if !flagQueryQuiet {
			fmt.Println("No results found.")
		}
		return nil
	}

	if !flagQueryQuiet {
		fmt.Printf("Found %d result(s):\n\n", len(results))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if !flagQueryQuiet {
		fmt.Fprintln(w, "FILE_PATH\tCHAT_ID\tMETADATA")
		fmt.Fprintln(w, "---------\t-------\t--------")
	}

	// Data rows
	for _, r := range results {
		// Format metadata as compact JSON
		metaStr := "{}"
		if r.Metadata != nil {
			metaBytes, err := json.Marshal(r.Metadata)
			if err == nil {
				metaStr = string(metaBytes)
			}
		}

		// Truncate metadata if too long
		if len(metaStr) > 80 {
			metaStr = metaStr[:77] + "..."
		}

		fmt.Fprintf(w, "%s\t%d\t%s\n", r.FilePath, r.ChatID, metaStr)
	}

	return w.Flush()
}
