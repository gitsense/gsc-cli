/**
 * Component: Analysis Status Command
 * Block-UUID: 0f5aa563-8cbc-4ed9-9154-b6c065f78e2f
 * Parent-UUID: 25bb9960-2d72-40bd-9854-c612d324f7d7
 * Version: 1.0.3
 * Description: Implements the 'gsc app analysis status' command for checking the existence and coverage of analyzers for a specific branch. Supports auto-detection of branch context via import state, or manual specification via --owner/--repo/--branch flags. Outputs JSON for programmatic consumption by external tools like depmap. v1.0.3: Updated resolveRefChatID call to match new signature (added owner, repo, branch arguments).
 * Language: Go
 * Created-at: 2026-06-10T03:17:52.358Z
 * Authors: MiMo-v2.5-Pro (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package analysis

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	flagStatusAnalyzer string
	flagStatusOwner    string
	flagStatusRepo     string
	flagStatusBranch   string
)

// StatusCmd represents the 'gsc app analysis status' command.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of analyzers for a branch",
	Long: `Checks if specific analyzers exist and how many files they cover for a given branch.
This is useful for pre-flight checks before running tools that depend on specific analysis data.

The command attempts to auto-detect the current branch context. If that fails, you can
manually specify the repository and branch using flags.

Examples:
  # Auto-detect branch and check multiple analyzers
  gsc app analysis status --analyzer code-intent,agent-file-triage

  # Manually specify branch context
  gsc app analysis status --analyzer rust-depmap --owner gitsense --repo chat --branch main`,
	RunE: runStatus,
}

func init() {
	StatusCmd.Flags().StringVar(&flagStatusAnalyzer, "analyzer", "", "Analyzer name(s) (comma-separated, e.g., code-intent,agent-file-triage)")
	_ = StatusCmd.MarkFlagRequired("analyzer")

	StatusCmd.Flags().StringVar(&flagStatusOwner, "owner", "", "Repository owner (for manual branch resolution)")
	StatusCmd.Flags().StringVar(&flagStatusRepo, "repo", "", "Repository name (for manual branch resolution)")
	StatusCmd.Flags().StringVar(&flagStatusBranch, "branch", "", "Branch name (for manual branch resolution)")
}

// runStatus is the main entry point for the status command.
func runStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// 1. Parse analyzers
	analyzers := parseAnalyzers(flagStatusAnalyzer)
	if len(analyzers) == 0 {
		return fmt.Errorf("--analyzer is required")
	}

	// 2. Open database
	dbConn, err := openChatDB()
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// 3. Resolve branch context (refChatID)
	refChatID, err := resolveRefChatIDWithFallback(dbConn)
	if err != nil {
		return err
	}

	// 4. Query status for each analyzer
	var results []AnalyzerStatus
	for _, analyzer := range analyzers {
		status, err := getAnalyzerStatus(dbConn, refChatID, analyzer)
		if err != nil {
			logger.Warning("Failed to query analyzer status", "analyzer", analyzer, "error", err)
			// Add a failed entry rather than failing the whole command
			results = append(results, AnalyzerStatus{
				Name:        analyzer,
				Exists:      false,
				FileCount:   0,
				MessageType: "",
			})
		} else {
			results = append(results, status)
		}
	}

	// 5. Output JSON
	response := StatusResponse{
		RefChatID: refChatID,
		Analyzers: results,
	}
	return outputJSON(response)
}

// resolveRefChatIDWithFallback attempts to auto-detect the branch context.
// If auto-detection fails, it falls back to manual resolution using --owner/--repo/--branch.
func resolveRefChatIDWithFallback(dbConn *sql.DB) (int64, error) {
	// Try auto-detection first (pass empty strings for owner/repo/branch to trigger auto-detection logic)
	refChatID, err := resolveRefChatID(dbConn, 0, "", "", "")
	if err == nil {
		return refChatID, nil
	}

	// Auto-detection failed. Check if manual flags are provided.
	if flagStatusOwner == "" || flagStatusRepo == "" || flagStatusBranch == "" {
		return 0, fmt.Errorf("could not determine branch context automatically and manual flags not provided.\n\n" +
			"Please either:\n" +
			"  1. Run 'gsc import' to import the current branch, or\n" +
			"  2. Provide --owner, --repo, and --branch flags manually.")
	}

	// Manual resolution
	groupID, err := db.GetGroupID(dbConn, flagStatusOwner, flagStatusRepo)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve repository '%s/%s': %w", flagStatusOwner, flagStatusRepo, err)
	}

	refChatID, err = db.GetRefChatID(dbConn, groupID, flagStatusBranch)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve branch '%s': %w", flagStatusBranch, err)
	}

	return refChatID, nil
}

// AnalyzerStatus represents the status of a single analyzer.
type AnalyzerStatus struct {
	Name        string `json:"name"`
	Exists      bool   `json:"exists"`
	FileCount   int    `json:"file_count"`
	MessageType string `json:"message_type"`
}

// StatusResponse is the top-level JSON response structure.
type StatusResponse struct {
	RefChatID int64            `json:"ref_chat_id"`
	Analyzers []AnalyzerStatus `json:"analyzers"`
}

// getAnalyzerStatus queries the database for a specific analyzer's status.
// It returns the file count and the actual message type found.
func getAnalyzerStatus(dbConn *sql.DB, refChatID int64, analyzer string) (AnalyzerStatus, error) {
	var query string
	var arg interface{}

	// Determine matching strategy (match CountAnalysisDump logic)
	if strings.Contains(analyzer, "::") {
		// Exact match
		query = `
			SELECT m.type, COUNT(*)
			FROM messages m
			JOIN chats c ON m.chat_id = c.id
			  AND m.type = ?
			  AND m.deleted = 0
			WHERE c.type = 'git-blob'
			  AND c.deleted = 0
			  AND json_extract(c.meta, '$.refContext.refChatId') = ?
			GROUP BY m.type
			LIMIT 1`
		arg = analyzer
	} else {
		// Prefix match
		query = `
			SELECT m.type, COUNT(*)
			FROM messages m
			JOIN chats c ON m.chat_id = c.id
			  AND m.type LIKE ?
			  AND m.deleted = 0
			WHERE c.type = 'git-blob'
			  AND c.deleted = 0
			  AND json_extract(c.meta, '$.refContext.refChatId') = ?
			GROUP BY m.type
			LIMIT 1`
		arg = analyzer + "::%"
	}

	var messageType string
	var count int

	err := dbConn.QueryRow(query, arg, refChatID).Scan(&messageType, &count)
	if err != nil {
		if err == sql.ErrNoRows {
			// No analysis found for this analyzer
			return AnalyzerStatus{
				Name:        analyzer,
				Exists:      false,
				FileCount:   0,
				MessageType: "",
			}, nil
		}
		return AnalyzerStatus{}, fmt.Errorf("database query failed: %w", err)
	}

	return AnalyzerStatus{
		Name:        analyzer,
		Exists:      true,
		FileCount:   count,
		MessageType: messageType,
	}, nil
}
