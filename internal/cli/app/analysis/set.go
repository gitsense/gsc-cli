/**
 * Component: Analysis Set Command
 * Block-UUID: 911f4636-9f88-4973-aa6f-2f956d68d76e
 * Parent-UUID: 525bbacf-ea29-43f2-834c-0773bb6d51b8
 * Version: 1.2.0
 * Description: Implements the 'gsc app analysis set' command for writing analysis metadata to the chat database. Supports bulk JSONL ingestion and single-file mode. Auto-generates the message content and meta envelope from consumer-provided fields. Routes through insert (new), update (force), or skip (exists) paths via SetAnalysisBulk. v1.1.1: Removed duplicate resolveRefChatID function (now uses the one from get.go). Removed unused imports (exec, git, importgit). v1.2.0: Added deterministic analysis_hash metadata for hash-based deduplication.
 * Language: Go
 * Created-at: 2026-06-09T23:28:48.074Z
 * Authors: MiMo-v2.5-Pro (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1), Codex (v1.2.0)
 */


package analysis

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	flagSetAnalyzer  string
	flagSetBulk      string
	flagSetFile      string
	flagSetJSON      string
	flagSetRefChatID int64
	flagSetDryRun    bool
	flagSetVerbose   bool
	flagSetForce     bool
	flagSetOwner     string
	flagSetRepo      string
	flagSetBranch    string
)

// SetCmd represents the 'gsc app analysis set' command.
var SetCmd = &cobra.Command{
	Use:   "set",
	Short: "Write analysis metadata for files",
	Long: `Writes extracted_metadata values for one or more files to a specific analyzer.
Bulk-first design: the primary input is a JSONL file where each line contains
a file_path and a fields object.

The command auto-generates the message content and metadata envelope.
Consumer programs only need to provide the field data - no knowledge of the
internal message schema is required.

Examples:
  # Bulk set from JSONL file
  gsc app analysis set --analyzer rust-depmap --bulk depmap-output.jsonl

  # Single file set (convenience for testing)
  gsc app analysis set --analyzer rust-depmap --file src/main.go --json '{"role":"defines","coupling_risk":"low"}'

  # Force overwrite existing analysis
  gsc app analysis set --analyzer rust-depmap --bulk depmap-output.jsonl --force

  # Dry run to validate without writing
  gsc app analysis set --analyzer rust-depmap --bulk depmap-output.jsonl --dry-run`,
	RunE: runSet,
}

func init() {
	SetCmd.Flags().StringVar(&flagSetAnalyzer, "analyzer", "", "Analyzer name (e.g., rust-depmap)")
	_ = SetCmd.MarkFlagRequired("analyzer")

	SetCmd.Flags().StringVar(&flagSetBulk, "bulk", "", "Path to JSONL file for bulk import")
	SetCmd.Flags().StringVar(&flagSetFile, "file", "", "Single file path (requires --json)")
	SetCmd.Flags().StringVar(&flagSetJSON, "json", "", "Inline JSON string for single file mode")
	SetCmd.Flags().Int64Var(&flagSetRefChatID, "ref-chat-id", 0, "Branch ref chat ID for disambiguation")
	SetCmd.Flags().BoolVar(&flagSetDryRun, "dry-run", false, "Validate and show what would be inserted without writing")
	SetCmd.Flags().BoolVar(&flagSetVerbose, "verbose", false, "Print each file as it is processed")
	SetCmd.Flags().BoolVar(&flagSetForce, "force", false, "Overwrite existing analysis (archive to history first)")
	SetCmd.Flags().StringVar(&flagSetOwner, "owner", "", "Repository owner (overrides auto-detection)")
	SetCmd.Flags().StringVar(&flagSetRepo, "repo", "", "Repository name (overrides auto-detection)")
	SetCmd.Flags().StringVar(&flagSetBranch, "branch", "", "Branch name (overrides auto-detection)")
}

// ---------------------------------------------------------------------------
// JSONL record structure (matches the plan's JSONL Schema)
// ---------------------------------------------------------------------------

// jsonlRecord represents a single line in the bulk JSONL input file.
type jsonlRecord struct {
	FilePath string                 `json:"file_path"`
	Fields   map[string]interface{} `json:"fields"`
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

// runSet is the main entry point for the set command.
func runSet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	startTime := time.Now()

	// 1. Validate: one of --bulk or --file is required
	if flagSetBulk == "" && flagSetFile == "" {
		return fmt.Errorf("either --bulk or --file is required")
	}
	if flagSetBulk != "" && flagSetFile != "" {
		return fmt.Errorf("cannot use both --bulk and --file")
	}
	if flagSetFile != "" && flagSetJSON == "" {
		return fmt.Errorf("--json is required when --file is used")
	}

	// 2. Build the JSONL records
	var records []jsonlRecord
	if flagSetBulk != "" {
		var err error
		records, err = readBulkJSONL(flagSetBulk)
		if err != nil {
			return err
		}
	} else {
		// Single file mode
		var fields map[string]interface{}
		if err := json.Unmarshal([]byte(flagSetJSON), &fields); err != nil {
			return fmt.Errorf("failed to parse --json: %w", err)
		}
		records = []jsonlRecord{
			{FilePath: flagSetFile, Fields: fields},
		}
	}

	if len(records) == 0 {
		fmt.Fprintln(os.Stderr, "No valid records found.")
		return nil
	}

	// 3. Open database
	dbConn, err := openChatDB()
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// 4. Resolve branch context
	refChatID, err := resolveRefChatID(dbConn, flagSetRefChatID, flagSetOwner, flagSetRepo, flagSetBranch)
	if err != nil {
		return err
	}

	// 5. Build path → chat_id resolution map
	pathToChatID, err := resolveFilePaths(dbConn, refChatID)
	if err != nil {
		return fmt.Errorf("failed to resolve file paths: %w", err)
	}

	// 6. Resolve records into SetItems
	var items []db.SetItem
	var skippedNoFile int

	for _, record := range records {
		chatID, exists := pathToChatID[record.FilePath]
		if !exists {
			skippedNoFile++
			if flagSetVerbose {
				fmt.Fprintf(os.Stderr, "  SKIP %s (file not in branch)\n", record.FilePath)
			}
			continue
		}

		item := db.SetItem{
			ChatID:      chatID,
			Analyzer:    buildMessageType(flagSetAnalyzer),
			Content:     generateMessageContent(flagSetAnalyzer, record.FilePath, record.Fields),
			Meta:        buildMetaEnvelope(record.Fields, "external"),
			SourceModel: "external",
		}
		items = append(items, item)
	}

	// 7. Print pre-flight summary
	fmt.Fprintf(os.Stderr, "\nReady to set analysis:\n")
	fmt.Fprintf(os.Stderr, "  Analyzer:    %s\n", flagSetAnalyzer)
	fmt.Fprintf(os.Stderr, "  Records:     %d in input\n", len(records))
	fmt.Fprintf(os.Stderr, "  Resolved:    %d files matched in branch\n", len(items))
	if skippedNoFile > 0 {
		fmt.Fprintf(os.Stderr, "  Not in branch: %d (will be skipped)\n", skippedNoFile)
	}
	fmt.Fprintf(os.Stderr, "  Force:       %v\n", flagSetForce)
	fmt.Fprintf(os.Stderr, "\n")

	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "No files to process.")
		return nil
	}

	// 8. Dry run: report what would happen and exit
	if flagSetDryRun {
		fmt.Fprintf(os.Stderr, "Dry run complete. %d records would be processed.\n", len(items))
		return nil
	}

	// 9. Execute SetAnalysisBulk
	progressFn := func(n int, chatID int64) {
		if flagSetVerbose {
			fmt.Fprintf(os.Stderr, "  [%d/%d] chat_id: %d\n", n, len(items), chatID)
		}
	}

	setResult, err := db.SetAnalysisBulk(dbConn, items, flagSetForce, 1000, progressFn)
	if err != nil {
		return fmt.Errorf("set operation failed: %w", err)
	}

	duration := time.Since(startTime)

	// 10. Print result summary (JSON to stdout, per plan)
	summary := map[string]interface{}{
		"analyzer":    flagSetAnalyzer,
		"total_lines": len(records),
		"inserted":    setResult.Inserted,
		"updated":     setResult.Updated,
		"skipped":     setResult.Skipped + skippedNoFile,
		"errors":      setResult.Errors,
		"duration_ms": duration.Milliseconds(),
	}

	if err := outputJSON(summary); err != nil {
		return fmt.Errorf("failed to encode summary: %w", err)
	}

	// 11. Determine exit code
	if setResult.Errors > 0 {
		return fmt.Errorf("%d errors occurred during set operation", setResult.Errors)
	}

	return nil
}

// ---------------------------------------------------------------------------
// JSONL Parsing
// ---------------------------------------------------------------------------

// readBulkJSONL reads and validates a JSONL file, returning all valid records.
// Invalid lines produce warnings to stderr but do not halt processing.
func readBulkJSONL(filePath string) ([]jsonlRecord, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer for large JSONL records (up to 10MB per line)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var records []jsonlRecord
	lineNum := 0
	errorCount := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // skip blank lines
		}

		var record jsonlRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			fmt.Fprintf(os.Stderr, "warning: line %d: invalid JSON, skipping: %v\n", lineNum, err)
			errorCount++
			continue
		}

		// Validate required fields
		if record.FilePath == "" {
			fmt.Fprintf(os.Stderr, "warning: line %d: missing required field 'file_path', skipping\n", lineNum)
			errorCount++
			continue
		}
		if record.Fields == nil {
			fmt.Fprintf(os.Stderr, "warning: line %d: missing required field 'fields', skipping\n", lineNum)
			errorCount++
			continue
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL file: %w", err)
	}

	if errorCount > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d lines had errors and were skipped\n", errorCount)
	}

	return records, nil
}

// ---------------------------------------------------------------------------
// Path Resolution
// ---------------------------------------------------------------------------

// resolveFilePaths queries the chats database to build a map of file_path → chat_id
// for all git-blob files in the target branch.
func resolveFilePaths(dbConn *sql.DB, refChatID int64) (map[string]int64, error) {
	query := `
		SELECT c.id, json_extract(c.meta, '$.path') as path
		FROM chats c
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`

	rows, err := dbConn.Query(query, refChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query file paths: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var chatID int64
		var path string
		if err := rows.Scan(&chatID, &path); err != nil {
			logger.Warning("Failed to scan file path row", "error", err)
			continue
		}
		result[path] = chatID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file path rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Message and Meta Construction
// ---------------------------------------------------------------------------

// buildMessageType converts a user-facing analyzer name to a full message type.
// "rust-depmap" → "rust-depmap::file-content::default"
// "rust-depmap::custom::v2" → "rust-depmap::custom::v2" (already fully qualified)
func buildMessageType(analyzer string) string {
	if strings.Contains(analyzer, "::") {
		return analyzer
	}
	return analyzer + "::file-content::default"
}

// generateMessageContent creates a minimal markdown overview from the field data.
// This is auto-generated by the set command - the consumer does not provide it.
func generateMessageContent(analyzer string, filePath string, fields map[string]interface{}) string {
	var sb strings.Builder

	// Extract a purpose if present, otherwise use a generic header
	purpose := ""
	if p, ok := fields["purpose"]; ok {
		if ps, ok := p.(string); ok {
			purpose = ps
		}
	}

	if purpose != "" {
		sb.WriteString(purpose)
		sb.WriteString("\n")
	} else {
		sb.WriteString(fmt.Sprintf("Analysis results for `%s` from `%s`.\n", filePath, analyzer))
	}

	return sb.String()
}

// buildMetaEnvelope constructs the full meta object for an analysis message.
// The consumer only provides fields - the set command wraps them in the standard envelope.
func buildMetaEnvelope(fields map[string]interface{}, analyzerModel string) map[string]interface{} {
	analysisHash := computeFieldsHash(fields)

	return map[string]interface{}{
		"extracted_metadata":       fields,
		"description":              "Automatically generated analysis",
		"label":                    "analysis",
		"version":                  "1.0.0",
		"tags":                     []string{"auto-generated"},
		"requires_reference_files": false,
		"analyzer_model":           analyzerModel,
		"analysis_hash":            analysisHash,
	}
}

func computeFieldsHash(fields map[string]interface{}) string {
	data, err := json.Marshal(fields)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", fields))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
