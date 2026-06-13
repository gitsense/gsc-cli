/**
 * Component: Analysis Get Command
 * Block-UUID: 4c0df673-b29a-4444-bd19-795c134770af
 * Parent-UUID: 5d830d66-72a0-4d52-8a04-cfa383a01efd
 * Version: 1.1.0
 * Description: Implements the 'gsc app analysis get' command for retrieving analysis metadata for files. Supports single file lookup, list mode, field filtering, and multi-analyzer queries with JSON, JSONL, and table output formats. v1.1.0: Added --owner, --repo, and --branch flags to support explicit branch context resolution, matching the pattern used in dump, load, and copy commands.
 * Language: Go
 * Created-at: 2026-06-10T01:45:55.178Z
 * Authors: MiMo-v2.5-Pro (v1.0.0), MiMo-v2.5-Pro (v1.0.1), GLM-4.7 (v1.1.0)
 */


package analysis

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	importgit "github.com/gitsense/gsc-cli/internal/cli/app/import/git"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	flagGetAnalyzer  string
	flagGetFile      string
	flagGetList      bool
	flagGetFields    string
	flagGetFormat    string
	flagGetRefChatID int64
	flagGetOwner     string
	flagGetRepo      string
	flagGetBranch    string
)

// GetCmd represents the 'gsc app analysis get' command.
var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Retrieve analysis metadata for files",
	Long: `Retrieves extracted_metadata fields for one or more files from a specific analyzer.

Supports single file lookup, list mode for all files, field filtering, and
multi-analyzer queries. Output can be formatted as JSON, JSONL, or a human-readable table.

Examples:
  # Get all fields for a single file
  gsc app analysis get --analyzer code-intent --file src/main.go

  # Get specific fields only
  gsc app analysis get --analyzer code-intent --file src/main.go --fields purpose,keywords

  # List all files for an analyzer
  gsc app analysis get --analyzer code-intent --list

  # List with specific fields in table format
  gsc app analysis get --analyzer code-intent --list --fields purpose,mechanism --format table

  # Query multiple analyzers for one file
  gsc app analysis get --analyzer code-intent,agent-file-triage --file src/main.go`,
	RunE: runGet,
}

func init() {
	GetCmd.Flags().StringVar(&flagGetAnalyzer, "analyzer", "", "Analyzer name (e.g., code-intent). Supports comma-separated list.")
	_ = GetCmd.MarkFlagRequired("analyzer")

	GetCmd.Flags().StringVar(&flagGetFile, "file", "", "Single file path (relative to repo root)")
	GetCmd.Flags().BoolVar(&flagGetList, "list", false, "List all files with analysis for this analyzer")
	GetCmd.Flags().StringVar(&flagGetFields, "fields", "", "Comma-separated field names to return (default: all)")
	GetCmd.Flags().StringVar(&flagGetFormat, "format", "json", "Output format: json, jsonl, table")
	GetCmd.Flags().Int64Var(&flagGetRefChatID, "ref-chat-id", 0, "Branch ref chat ID for disambiguation")
	GetCmd.Flags().StringVar(&flagGetOwner, "owner", "", "Repository owner (overrides auto-detection)")
	GetCmd.Flags().StringVar(&flagGetRepo, "repo", "", "Repository name (overrides auto-detection)")
	GetCmd.Flags().StringVar(&flagGetBranch, "branch", "", "Branch name (overrides auto-detection)")
}

// runGet is the main entry point for the get command.
func runGet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// 1. Validate: one of --file or --list is required
	if flagGetFile == "" && !flagGetList {
		return fmt.Errorf("either --file or --list is required")
	}
	if flagGetFile != "" && flagGetList {
		return fmt.Errorf("cannot use both --file and --list")
	}

	// 2. Parse analyzers
	analyzers := parseAnalyzers(flagGetAnalyzer)
	if len(analyzers) == 0 {
		return fmt.Errorf("--analyzer is required")
	}

	// 3. Parse optional fields filter
	selectFields := parseFieldsList(flagGetFields)

	// 4. Open database
	dbConn, err := openChatDB()
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// 5. Resolve branch context (refChatID)
	refChatID, err := resolveRefChatID(dbConn, flagGetRefChatID, flagGetOwner, flagGetRepo, flagGetBranch)
	if err != nil {
		return err
	}

	// 6. Dispatch based on mode
	if flagGetList {
		return runGetList(dbConn, refChatID, analyzers, selectFields)
	}
	return runGetFile(dbConn, refChatID, flagGetFile, analyzers, selectFields)
}

// ---------------------------------------------------------------------------
// Single File Mode
// ---------------------------------------------------------------------------

// runGetFile handles single-file analysis retrieval for one or more analyzers.
func runGetFile(dbConn *sql.DB, refChatID int64, filePath string, analyzers []string, selectFields []string) error {
	if len(analyzers) == 1 {
		// Single analyzer: return a flat map with file_path, chat_id, and fields
		result, err := db.GetAnalysisForFile(dbConn, refChatID, filePath, analyzers[0], selectFields)
		if err != nil {
			if errors.Is(err, db.ErrFileNotFound) {
				return outputFileNotFoundError(filePath)
			}
			return err
		}
		if result == nil {
			return outputNoAnalysisFound(filePath, analyzers[0])
		}
		return outputJSON(result)
	}

	// Multiple analyzers: return a nested map keyed by analyzer name
	output := map[string]interface{}{
		"file_path": filePath,
	}

	var chatID int64
	for _, analyzer := range analyzers {
		result, err := db.GetAnalysisForFile(dbConn, refChatID, filePath, analyzer, nil)
		if err != nil {
			if errors.Is(err, db.ErrFileNotFound) {
				return outputFileNotFoundError(filePath)
			}
			fmt.Fprintf(os.Stderr, "warning: failed to query analyzer '%s': %v\n", analyzer, err)
			continue
		}

		if result == nil {
			output[analyzer] = map[string]interface{}{
				"error":   "no_analysis_found",
				"message": fmt.Sprintf("No analysis of type '%s' found for this file.", analyzer),
			}
			continue
		}

		// Extract chat_id from the first successful result
		if chatID == 0 {
			if id, ok := result["chat_id"]; ok {
				chatID = toInt64(id)
			}
			output["chat_id"] = chatID
		}

		// Remove shared keys from the nested analyzer result
		delete(result, "file_path")
		delete(result, "chat_id")
		output[analyzer] = result
	}

	return outputJSON(output)
}

// ---------------------------------------------------------------------------
// List Mode
// ---------------------------------------------------------------------------

// runGetList handles list-mode analysis retrieval for a single analyzer.
func runGetList(dbConn *sql.DB, refChatID int64, analyzers []string, selectFields []string) error {
	if len(analyzers) > 1 {
		return fmt.Errorf("--list mode only supports a single analyzer")
	}

	results, err := db.ListAnalysisForAnalyzer(dbConn, refChatID, analyzers[0], selectFields)
	if err != nil {
		return err
	}

	switch flagGetFormat {
	case "jsonl":
		return outputJSONL(results)
	case "table":
		return outputTable(results, selectFields)
	default: // json
		output := map[string]interface{}{
			"analyzer":    analyzers[0],
			"total_files": len(results),
			"files":       results,
		}
		return outputJSON(output)
	}
}

// ---------------------------------------------------------------------------
// Branch Resolution
// ---------------------------------------------------------------------------

// resolveRefChatID resolves the branch context chat ID using the following priority:
//  1. Explicit --ref-chat-id flag
//  2. Explicit --owner, --repo, --branch flags (if all three are provided)
//  3. Auto-resolution from import state file + current git HEAD branch
func resolveRefChatID(dbConn *sql.DB, explicit int64, owner, repo, branch string) (int64, error) {
	if explicit != 0 {
		return explicit, nil
	}

	// If all three context flags are provided, use them directly
	if owner != "" && repo != "" && branch != "" {
		groupID, err := db.GetGroupID(dbConn, owner, repo)
		if err != nil {
			return 0, fmt.Errorf("could not resolve repository '%s/%s': %w", owner, repo, err)
		}

		refChatID, err := db.GetRefChatID(dbConn, groupID, branch)
		if err != nil {
			return 0, fmt.Errorf("could not resolve branch '%s': %w", branch, err)
		}

		return refChatID, nil
	}

	// Fallback to auto-resolution
	gitPath, err := git.FindGitRoot()
	if err != nil {
		return 0, fmt.Errorf("could not determine branch: not in a git repository. Use --ref-chat-id or --owner/--repo/--branch")
	}

	state, err := importgit.LoadState(gitPath)
	if err != nil {
		return 0, fmt.Errorf("could not determine branch: no import state found. Use --ref-chat-id or --owner/--repo/--branch")
	}

	if state.Owner == "" || state.Repo == "" {
		return 0, fmt.Errorf("could not determine repository from state. Use --ref-chat-id or --owner/--repo/--branch")
	}

	// Get current branch from git HEAD
	cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdBranch.Dir = gitPath
	output, err := cmdBranch.Output()
	if err != nil {
		return 0, fmt.Errorf("could not determine branch from git HEAD. Use --ref-chat-id or --owner/--repo/--branch")
	}
	branch = strings.TrimSpace(string(output))
	if branch == "" {
		return 0, fmt.Errorf("could not determine branch from git HEAD. Use --ref-chat-id or --owner/--repo/--branch")
	}

	// Resolve through the database: owner/repo → groupID → refChatID
	groupID, err := db.GetGroupID(dbConn, state.Owner, state.Repo)
	if err != nil {
		return 0, fmt.Errorf("could not resolve repository '%s/%s': %w. Use --ref-chat-id or --owner/--repo/--branch", state.Owner, state.Repo, err)
	}

	refChatID, err := db.GetRefChatID(dbConn, groupID, branch)
	if err != nil {
		return 0, fmt.Errorf("could not resolve branch '%s': %w. Use --ref-chat-id or --owner/--repo/--branch", branch, err)
	}

	return refChatID, nil
}

// ---------------------------------------------------------------------------
// Database Connection
// ---------------------------------------------------------------------------

// openChatDB opens the GitSense Chat database connection.
// This is shared across get and set commands in this package.
func openChatDB() (*sql.DB, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}
	dbPath := settings.GetChatDatabasePath(gscHome)
	if err := db.ValidateDBExists(dbPath); err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}
	return db.OpenDB(dbPath)
}

// ---------------------------------------------------------------------------
// Output Helpers
// ---------------------------------------------------------------------------

// outputJSON writes data as formatted JSON to stdout.
func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// outputJSONL writes results as newline-delimited JSON to stdout.
func outputJSONL(results []map[string]interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	for _, r := range results {
		if err := encoder.Encode(r); err != nil {
			return fmt.Errorf("failed to encode JSONL record: %w", err)
		}
	}
	return nil
}

// outputTable writes results as a human-readable table to stdout.
func outputTable(results []map[string]interface{}, selectFields []string) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Determine column order
	columns := []string{"file_path", "chat_id"}
	if len(selectFields) > 0 {
		columns = append(columns, selectFields...)
	} else {
		// Auto-detect columns from the first result, preserving insertion order
		for k := range results[0] {
			if k != "file_path" && k != "chat_id" {
				columns = append(columns, k)
			}
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header row
	fmt.Fprintln(w, strings.Join(columns, "\t"))

	// Separator row
	separators := make([]string, len(columns))
	for i, col := range columns {
		dashLen := len(col)
		if dashLen < 3 {
			dashLen = 3
		}
		separators[i] = strings.Repeat("-", dashLen)
	}
	fmt.Fprintln(w, strings.Join(separators, "\t"))

	// Data rows
	for _, result := range results {
		values := make([]string, len(columns))
		for i, col := range columns {
			if val, ok := result[col]; ok {
				values[i] = formatTableValue(val)
			} else {
				values[i] = ""
			}
		}
		fmt.Fprintln(w, strings.Join(values, "\t"))
	}

	return w.Flush()
}

// outputFileNotFoundError writes a file_not_found JSON error to stdout and
// returns a non-nil error so cobra exits with code 1.
func outputFileNotFoundError(filePath string) error {
	_ = outputJSON(map[string]interface{}{
		"error":   "file_not_found",
		"message": fmt.Sprintf("File '%s' not found in the database.", filePath),
	})
	return fmt.Errorf("file not found: %s", filePath)
}

// outputNoAnalysisFound writes a no_analysis_found JSON response to stdout.
// This is not an error - the file exists but has no analysis of the requested type.
// Exit code is 0.
func outputNoAnalysisFound(filePath, analyzer string) error {
	return outputJSON(map[string]interface{}{
		"file_path": filePath,
		"error":     "no_analysis_found",
		"message":   fmt.Sprintf("No analysis of type '%s' found for this file.", analyzer),
	})
}

// ---------------------------------------------------------------------------
// Utility Functions (shared across get.go and set.go)
// ---------------------------------------------------------------------------

// parseAnalyzers splits a comma-separated analyzer string and trims whitespace.
func parseAnalyzers(analyzerStr string) []string {
	if analyzerStr == "" {
		return nil
	}
	parts := strings.Split(analyzerStr, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseFieldsList splits a comma-separated fields string and trims whitespace.
func parseFieldsList(fieldsStr string) []string {
	if fieldsStr == "" {
		return nil
	}
	parts := strings.Split(fieldsStr, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// formatTableValue formats a value for display in a table cell.
// Long strings and arrays are truncated to 50 characters.
func formatTableValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return truncateString(val, 50)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatTableValue(item)
		}
		return truncateString(strings.Join(parts, ", "), 50)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return truncateString(fmt.Sprintf("%v", val), 50)
		}
		return truncateString(string(data), 50)
	}
}

// truncateString truncates s to maxLen characters, appending "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// toInt64 converts a numeric interface{} value to int64.
// Handles float64 (from JSON), int64, int, and json.Number.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case json.Number:
		n, _ := val.Int64()
		return n
	default:
		return 0
	}
}
