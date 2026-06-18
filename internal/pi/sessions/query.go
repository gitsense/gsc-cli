/**
 * Component: Pi Sessions Query Engine
 * Block-UUID: 52080e7b-c252-4955-a46e-d37be5ad6c03
 * Parent-UUID: N/A
 * Version: 1.2.0
 * Description: Executes phase-one discovery queries over imported Pi sessions.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v1.1.0, v1.2.0)
 */

package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
)

// graphNode represents an entry in the session graph for branch analysis.
type graphNode struct {
	entryID        string
	parentEntryID  string
	entryType      string
	seq            int
}

// branchGraph holds the graph structure for a single session.
type branchGraph struct {
	entryByID     map[string]graphNode
	parentByID    map[string]string
	childrenByID  map[string][]string
	leaves        []string
}

// buildBranchGraph constructs a branch graph from session message rows.
func buildBranchGraph(rows []graphNode) branchGraph {
	graph := branchGraph{
		entryByID:    make(map[string]graphNode, len(rows)),
		parentByID:   make(map[string]string, len(rows)),
		childrenByID: make(map[string][]string, len(rows)),
	}

	for _, row := range rows {
		graph.entryByID[row.entryID] = row
		if row.parentEntryID != "" {
			graph.parentByID[row.entryID] = row.parentEntryID
			graph.childrenByID[row.parentEntryID] = append(graph.childrenByID[row.parentEntryID], row.entryID)
		}
	}

	// Find leaves: entries that never appear as a parent
	isParent := make(map[string]bool, len(graph.childrenByID))
	for parentID := range graph.childrenByID {
		isParent[parentID] = true
		}
	for _, row := range rows {
		if !isParent[row.entryID] {
			graph.leaves = append(graph.leaves, row.entryID)
		}
	}
	return graph
}

// ancestorSet returns the set of all ancestor entry IDs (including the entry itself).
func ancestorSet(graph branchGraph, entryID string) map[string]bool {
	ancestors := make(map[string]bool)
	current := entryID
	for current != "" {
		ancestors[current] = true
		current = graph.parentByID[current]
	}
	return ancestors
}

// branchAnnotations holds the branch enrichment data for a single result.
type branchAnnotations struct {
	branchLeafIDs          []string
	nearestCompactionID    string
	nearestBranchSummaryID string
}

// annotateResult computes branch annotations for a single entry.
func annotateResult(graph branchGraph, entryID string) branchAnnotations {
	if entryID == "" {
		return branchAnnotations{}
	}

	// Build ancestor sets for all leaves
	leafAncestors := make(map[string]map[string]bool, len(graph.leaves))
	for _, leafID := range graph.leaves {
		leafAncestors[leafID] = ancestorSet(graph, leafID)
	}

	// Find all leaves that have this entry as an ancestor
	var branchLeafIDs []string
	for _, leafID := range graph.leaves {
		if leafAncestors[leafID][entryID] {
			branchLeafIDs = append(branchLeafIDs, leafID)
		}
	}
	// Sort for deterministic output
	sort.Strings(branchLeafIDs)

	// Find nearest compaction ancestor
	nearestCompactionID := ""
	current := entryID
	for current != "" {
		node, ok := graph.entryByID[current]
		if !ok {
			break
		}
		if node.entryType == "compaction" {
			nearestCompactionID = current
			break
		}
		current = graph.parentByID[current]
	}

	// Find nearest branch_summary ancestor
	nearestBranchSummaryID := ""
	current = entryID
	for current != "" {
		node, ok := graph.entryByID[current]
		if !ok {
			break
		}
		if node.entryType == "branch_summary" {
			nearestBranchSummaryID = current
			break
		}
		current = graph.parentByID[current]
	}

	return branchAnnotations{
		branchLeafIDs:          branchLeafIDs,
		nearestCompactionID:    nearestCompactionID,
		nearestBranchSummaryID: nearestBranchSummaryID,
	}
}

// enrichWithBranches adds branch metadata to query results.
func enrichWithBranches(ctx context.Context, database *sql.DB, results []QueryResult) error {
	// Collect unique session IDs
	sessionIDs := make(map[string]bool)
	for _, r := range results {
		if r.SessionID != "" {
			sessionIDs[r.SessionID] = true
		}
	}

	// Build graph per session
	type sessionGraph struct {
		chatID int64
		graph  branchGraph
	}
	graphs := make(map[string]sessionGraph, len(sessionIDs))

	for sessionID := range sessionIDs {
		// Get internal chat_id
		var chatID int64
		err := database.QueryRowContext(ctx, "SELECT id FROM pi_chats WHERE uuid = ?", sessionID).Scan(&chatID)
		if err != nil {
			continue
		}

		// Load graph rows
		rows, err := database.QueryContext(ctx,
			"SELECT entry_id, parent_entry_id, type, seq FROM pi_messages WHERE chat_id = ? ORDER BY seq",
			chatID,
		)
		if err != nil {
			continue
		}

		var graphRows []graphNode
		for rows.Next() {
			var node graphNode
			var parentEntryID sql.NullString
			var entryType sql.NullString
			if err := rows.Scan(&node.entryID, &parentEntryID, &entryType, &node.seq); err != nil {
				rows.Close()
				continue
			}
			node.parentEntryID = parentEntryID.String
			node.entryType = entryType.String
			graphRows = append(graphRows, node)
		}
		rows.Close()

		// Skip enrichment for very large sessions
		if len(graphRows) > 20000 {
			continue
		}

		graphs[sessionID] = sessionGraph{
			chatID: chatID,
			graph:  buildBranchGraph(graphRows),
		}
	}

	// Annotate each result
	for i := range results {
		sessionID := results[i].SessionID
		entryID := results[i].EntryID
		if sessionID == "" || entryID == "" {
			continue
		}
		sessionGraph, ok := graphs[sessionID]
		if !ok {
			continue
		}
		annotations := annotateResult(sessionGraph.graph, entryID)
		results[i].BranchLeafIDs = annotations.branchLeafIDs
		results[i].NearestCompactionID = annotations.nearestCompactionID
		results[i].NearestBranchSummaryID = annotations.nearestBranchSummaryID
	}

	return nil
}

func hasToolCallFilters(options QueryOptions) bool {
	return options.CommandStartsWith != "" ||
		options.CommandContains != "" ||
		options.OutputContains != "" ||
		options.ToolArgsContains != ""
}

func hasSessionFilters(options QueryOptions) bool {
	return options.SessionName != "" || options.SessionNamePrefix != ""
}

func validateToolCallOptions(options QueryOptions) error {
	if hasToolCallFilters(options) && options.Tool != "" && strings.ToLower(options.Tool) != "bash" {
		return fmt.Errorf("--command-* and --output-* flags only apply to bash tool calls; remove --tool or use --tool bash")
	}
	return nil
}

func resolveEntryType(options QueryOptions) string {
	if options.EntryType != "" {
		return options.EntryType
	}
	return options.Type
}

func Query(ctx context.Context, options QueryOptions) ([]QueryResult, error) {
	if options.DBPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	if err := validateToolCallOptions(options); err != nil {
		return nil, err
	}
	database, err := openQueryMirror(options.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	if options.View == "sessions" {
		return nil, fmt.Errorf("use QuerySessions for --view sessions")
	}

	var results []QueryResult
	if options.File != "" || options.AbsFile != "" || options.Op != "" {
		results, err = queryFileRefs(ctx, database, options)
	} else if options.Tool != "" || hasToolCallFilters(options) {
		results, err = queryToolCalls(ctx, database, options)
	} else if options.Text != "" {
		results, err = queryText(ctx, database, options)
	} else {
		results, err = queryMessages(ctx, database, options)
	}
	if err != nil {
		return nil, err
	}

	// Enrich with branch metadata if requested
	if options.WithBranches && len(results) > 0 {
		if err := enrichWithBranches(ctx, database, results); err != nil {
			return results, err
		}
	}

	return results, nil
}

// QuerySessions returns aggregated session-level results.
func QuerySessions(ctx context.Context, options QueryOptions) ([]SessionQueryResult, error) {
	if options.DBPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	if err := validateToolCallOptions(options); err != nil {
		return nil, err
	}
	database, err := openQueryMirror(options.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	hasFileFilter := options.File != "" || options.AbsFile != ""
	hasToolFilter := options.Tool != "" || hasToolCallFilters(options)
	hasMessageFilter := options.Text != ""

	if hasFileFilter || hasToolFilter || hasMessageFilter {
		return querySessionsWithMatches(ctx, database, options)
	}
	return querySessionsBasic(ctx, database, options)
}

func querySessionsBasic(ctx context.Context, database *sql.DB, options QueryOptions) ([]SessionQueryResult, error) {
	query := `
		SELECT c.uuid, c.name, c.cwd, c.repo_root, c.provider, c.model,
		       c.created_at, c.last_message_at, c.message_count,
		       c.tool_call_count, c.file_ref_count, c.first_user_text
		FROM pi_chats c
		WHERE c.file_deleted_at IS NULL`
	var args []interface{}
	query, args = appendSessionFilters(query, args, options)

	sort := strings.ToLower(options.Sort)
	switch sort {
	case "oldest":
		query += " ORDER BY c.created_at ASC"
	default:
		query += " ORDER BY c.last_message_at DESC"
	}
	query += " LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SessionQueryResult
	for rows.Next() {
		var r SessionQueryResult
		var name, cwd, repoRoot, provider, model sql.NullString
		var createdAt, lastMessageAt, firstUserText sql.NullString
		if err := rows.Scan(
			&r.SessionID, &name, &cwd, &repoRoot, &provider, &model,
			&createdAt, &lastMessageAt, &r.MessageCount,
			&r.ToolCallCount, &r.FileRefCount, &firstUserText,
		); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.CWD = cwd.String
		r.RepoRoot = repoRoot.String
		r.Provider = provider.String
		r.Model = model.String
		r.CreatedAt = createdAt.String
		r.LastMessageAt = lastMessageAt.String
		r.Title = sessionTitle(r.Name, firstUserText.String)
		results = append(results, r)
	}
	return results, rows.Err()
}

func querySessionsWithMatches(ctx context.Context, database *sql.DB, options QueryOptions) ([]SessionQueryResult, error) {
	// First get all sessions matching session-level filters
	baseQuery := `
		SELECT c.id, c.uuid, c.name, c.cwd, c.repo_root, c.provider, c.model,
		       c.created_at, c.last_message_at, c.message_count,
		       c.tool_call_count, c.file_ref_count, c.first_user_text
		FROM pi_chats c
		WHERE c.file_deleted_at IS NULL`
	var baseArgs []interface{}
	baseQuery, baseArgs = appendSessionFilters(baseQuery, baseArgs, options)

	sort := strings.ToLower(options.Sort)
	switch sort {
	case "oldest":
		baseQuery += " ORDER BY c.created_at ASC"
	default:
		baseQuery += " ORDER BY c.last_message_at DESC"
	}
	baseQuery += " LIMIT ?"
	baseArgs = append(baseArgs, limitOrDefault(options.Limit)*10) // fetch more since we'll filter

	rows, err := database.QueryContext(ctx, baseQuery, baseArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type sessionRow struct {
		id int64
		r  SessionQueryResult
	}
	var candidates []sessionRow
	for rows.Next() {
		var sr sessionRow
		var name, cwd, repoRoot, provider, model sql.NullString
		var createdAt, lastMessageAt, firstUserText sql.NullString
		if err := rows.Scan(
			&sr.id, &sr.r.SessionID, &name, &cwd, &repoRoot, &provider, &model,
			&createdAt, &lastMessageAt, &sr.r.MessageCount,
			&sr.r.ToolCallCount, &sr.r.FileRefCount, &firstUserText,
		); err != nil {
			return nil, err
		}
		sr.r.Name = name.String
		sr.r.CWD = cwd.String
		sr.r.RepoRoot = repoRoot.String
		sr.r.Provider = provider.String
		sr.r.Model = model.String
		sr.r.CreatedAt = createdAt.String
		sr.r.LastMessageAt = lastMessageAt.String
		sr.r.Title = sessionTitle(sr.r.Name, firstUserText.String)
		candidates = append(candidates, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Now count matches for each candidate
	var results []SessionQueryResult
	limit := limitOrDefault(options.Limit)

	for _, sr := range candidates {
		if len(results) >= limit {
			break
		}

		// File ref matches
		if options.File != "" || options.AbsFile != "" {
			countQuery := `SELECT COUNT(*), GROUP_CONCAT(DISTINCT file_path_rel) FROM pi_file_refs WHERE chat_id = ?`
			countArgs := []interface{}{sr.id}
			if options.File != "" {
				countQuery += ` AND file_path_rel = ?`
				countArgs = append(countArgs, options.File)
			}
			if options.AbsFile != "" {
				countQuery += ` AND abs_path = ?`
				countArgs = append(countArgs, options.AbsFile)
			}
			if options.Op != "" {
				countQuery += ` AND op = ?`
				countArgs = append(countArgs, options.Op)
			}
			var count int
			var paths sql.NullString
			if err := database.QueryRowContext(ctx, countQuery, countArgs...).Scan(&count, &paths); err != nil {
				return nil, err
			}
			sr.r.MatchedFileRefCount = count
			if paths.Valid && paths.String != "" {
				sr.r.MatchedPaths = strings.Split(paths.String, ",")
			}
		}

		// Tool call matches
		if options.Tool != "" || hasToolCallFilters(options) {
			countQuery := `SELECT COUNT(*) FROM pi_tool_calls t
				JOIN pi_messages m ON m.id = t.message_id
				WHERE t.chat_id = ?`
			countArgs := []interface{}{sr.id}
			toolName := strings.ToLower(options.Tool)
			if hasToolCallFilters(options) && toolName == "" {
				toolName = "bash"
			}
			if toolName != "" {
				countQuery += ` AND t.tool_name = ?`
				countArgs = append(countArgs, toolName)
			}
			commandExpr := "json_extract(t.arguments_json, '$.command')"
			if options.CommandStartsWith != "" {
				pattern := options.CommandStartsWith + "%"
				if options.CaseInsensitive {
					countQuery += ` AND LOWER(` + commandExpr + `) LIKE LOWER(?)`
				} else {
					countQuery += ` AND ` + commandExpr + ` LIKE ?`
				}
				countArgs = append(countArgs, pattern)
			}
			if options.CommandContains != "" {
				pattern := "%" + options.CommandContains + "%"
				if options.CaseInsensitive {
					countQuery += ` AND LOWER(` + commandExpr + `) LIKE LOWER(?)`
				} else {
					countQuery += ` AND ` + commandExpr + ` LIKE ?`
				}
				countArgs = append(countArgs, pattern)
			}
			if options.OutputContains != "" {
				pattern := "%" + options.OutputContains + "%"
				if options.CaseInsensitive {
					countQuery += ` AND LOWER(t.result_text) LIKE LOWER(?)`
				} else {
					countQuery += ` AND t.result_text LIKE ?`
				}
				countArgs = append(countArgs, pattern)
			}
			if options.ToolArgsContains != "" {
				pattern := "%" + options.ToolArgsContains + "%"
				if options.CaseInsensitive {
					countQuery += ` AND LOWER(t.arguments_json) LIKE LOWER(?)`
				} else {
					countQuery += ` AND t.arguments_json LIKE ?`
				}
				countArgs = append(countArgs, pattern)
			}
			var count int
			if err := database.QueryRowContext(ctx, countQuery, countArgs...).Scan(&count); err != nil {
				return nil, err
			}
			sr.r.MatchedToolCallCount = count
		}

		// Message matches
		if options.Text != "" {
			countQuery := `SELECT COUNT(*) FROM fts_pi_messages fts
				JOIN pi_messages m ON m.id = fts.rowid
				WHERE m.chat_id = ? AND fts MATCH ?`
			var count int
			if err := database.QueryRowContext(ctx, countQuery, sr.id, options.Text).Scan(&count); err != nil {
				return nil, err
			}
			sr.r.MatchedMessageCount = count
		}

		sr.r.MatchCount = sr.r.MatchedFileRefCount + sr.r.MatchedToolCallCount + sr.r.MatchedMessageCount
		if sr.r.MatchCount > 0 {
			results = append(results, sr.r)
		}
	}

	// Sort results
	switch strings.ToLower(options.Sort) {
	case "match-count":
		// Already sorted by match count due to candidate ordering
	case "oldest":
		// Already sorted by created_at ASC
	default:
		// Already sorted by last_message_at DESC
	}

	return results, nil
}

// List returns a compact list of sessions.
func List(ctx context.Context, options ListOptions) ([]ListResult, error) {
	if options.DBPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	database, err := openQueryMirror(options.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	query := `
		SELECT c.uuid, c.cwd, c.repo_root, c.created_at, c.last_message_at,
		       c.message_count, c.last_user_text
		FROM pi_chats c
		WHERE c.file_deleted_at IS NULL`
	var args []interface{}

	if options.Repo != "" {
		query += " AND c.repo_root = ?"
		args = append(args, options.Repo)
	}
	if options.Provider != "" {
		query += " AND c.provider = ?"
		args = append(args, options.Provider)
	}
	if options.Model != "" {
		query += " AND c.model = ?"
		args = append(args, options.Model)
	}
	if options.Since != "" {
		query += " AND c.created_at >= ?"
		args = append(args, options.Since)
	}
	if options.Until != "" {
		query += " AND c.created_at <= ?"
		args = append(args, options.Until)
	}

	sort := strings.ToLower(options.Sort)
	switch sort {
	case "oldest":
		query += " ORDER BY c.created_at ASC"
	case "messages":
		query += " ORDER BY c.message_count DESC"
	default:
		query += " ORDER BY c.last_message_at DESC"
	}
	query += " LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ListResult
	for rows.Next() {
		var r ListResult
		var cwd, repoRoot sql.NullString
		var createdAt, lastMessageAt, lastUserText sql.NullString
		if err := rows.Scan(
			&r.SessionID, &cwd, &repoRoot, &createdAt, &lastMessageAt,
			&r.MessageCount, &lastUserText,
		); err != nil {
			return nil, err
		}
		r.CWD = cwd.String
		r.RepoRoot = repoRoot.String
		r.CreatedAt = createdAt.String
		r.LastMessageAt = lastMessageAt.String
		r.LastUserText = compactText(lastUserText.String)
		results = append(results, r)
	}
	return results, rows.Err()
}

func appendSessionFilters(query string, args []interface{}, options QueryOptions) (string, []interface{}) {
	if options.SessionID != "" {
		query += " AND c.uuid = ?"
		args = append(args, options.SessionID)
	}
	if options.Repo != "" {
		query += " AND c.repo_root = ?"
		args = append(args, options.Repo)
	}
	if options.Provider != "" {
		query += " AND c.provider = ?"
		args = append(args, options.Provider)
	}
	if options.Model != "" {
		query += " AND c.model = ?"
		args = append(args, options.Model)
	}
	if options.SessionName != "" {
		query += " AND c.name = ?"
		args = append(args, options.SessionName)
	}
	if options.SessionNamePrefix != "" {
		query += " AND c.name LIKE ?"
		args = append(args, options.SessionNamePrefix+"%")
	}
	if options.Since != "" {
		query += " AND c.created_at >= ?"
		args = append(args, options.Since)
	}
	if options.Until != "" {
		query += " AND c.created_at <= ?"
		args = append(args, options.Until)
	}
	return query, args
}

func sessionTitle(name, firstUserText string) string {
	if name != "" {
		return name
	}
	if firstUserText != "" {
		return compactText(firstUserText)
	}
	return ""
}

func queryFileRefs(ctx context.Context, database *sql.DB, options QueryOptions) ([]QueryResult, error) {
	query := `
		SELECT c.uuid, c.name, c.cwd, c.repo_root, m.entry_id, r.tool_call_id,
		       r.tool_name, r.op, r.source, r.raw_path, r.abs_path, r.file_path_rel,
		       r.timestamp, m.text
		FROM pi_file_refs r
		JOIN pi_chats c ON c.id = r.chat_id
		JOIN pi_messages m ON m.id = r.message_id
		WHERE c.file_deleted_at IS NULL`
	var args []interface{}
	query, args = appendCommonFilters(query, args, options, "c", "m")
	if options.File != "" {
		query += " AND r.file_path_rel = ?"
		args = append(args, options.File)
	}
	if options.AbsFile != "" {
		query += " AND r.abs_path = ?"
		args = append(args, options.AbsFile)
	}
	if options.Op != "" {
		query += " AND r.op = ?"
		args = append(args, options.Op)
	}
	query += " ORDER BY r.timestamp DESC LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var result QueryResult
		var name, cwd, repoRoot, toolCallID, toolName, text sql.NullString
		var rawPath, absPath, filePathRel sql.NullString
		if err := rows.Scan(
			&result.SessionID,
			&name,
			&cwd,
			&repoRoot,
			&result.EntryID,
			&toolCallID,
			&toolName,
			&result.Op,
			&result.Source,
			&rawPath,
			&absPath,
			&filePathRel,
			&result.Timestamp,
			&text,
		); err != nil {
			return nil, err
		}
		result.Kind = "file_ref"
		result.SessionName = name.String
		result.CWD = cwd.String
		result.RepoRoot = repoRoot.String
		result.ToolCallID = toolCallID.String
		result.ToolName = toolName.String
		result.RawPath = rawPath.String
		result.AbsPath = absPath.String
		result.FilePathRel = filePathRel.String
		result.Text = compactText(text.String)
		results = append(results, result)
	}
	return results, rows.Err()
}

func queryToolCalls(ctx context.Context, database *sql.DB, options QueryOptions) ([]QueryResult, error) {
	query := `
		SELECT c.uuid, c.name, c.cwd, c.repo_root, t.entry_id, t.tool_call_id,
		       t.tool_name, t.timestamp, t.arguments_json, t.result_text
		FROM pi_tool_calls t
		JOIN pi_chats c ON c.id = t.chat_id
		JOIN pi_messages m ON m.id = t.message_id
		WHERE c.file_deleted_at IS NULL`
	var args []interface{}
	query, args = appendCommonFilters(query, args, options, "c", "m")

	toolName := strings.ToLower(options.Tool)
	if hasToolCallFilters(options) && toolName == "" {
		toolName = "bash"
	}
	if toolName != "" {
		query += " AND t.tool_name = ?"
		args = append(args, toolName)
	}

	commandExpr := "json_extract(t.arguments_json, '$.command')"
	if options.CommandStartsWith != "" {
		pattern := options.CommandStartsWith + "%"
		if options.CaseInsensitive {
			query += " AND LOWER(" + commandExpr + ") LIKE LOWER(?)"
		} else {
			query += " AND " + commandExpr + " LIKE ?"
		}
		args = append(args, pattern)
	}
	if options.CommandContains != "" {
		pattern := "%" + options.CommandContains + "%"
		if options.CaseInsensitive {
			query += " AND LOWER(" + commandExpr + ") LIKE LOWER(?)"
		} else {
			query += " AND " + commandExpr + " LIKE ?"
		}
		args = append(args, pattern)
	}
	if options.OutputContains != "" {
		pattern := "%" + options.OutputContains + "%"
		if options.CaseInsensitive {
			query += " AND LOWER(t.result_text) LIKE LOWER(?)"
		} else {
			query += " AND t.result_text LIKE ?"
		}
		args = append(args, pattern)
	}
	if options.ToolArgsContains != "" {
		pattern := "%" + options.ToolArgsContains + "%"
		if options.CaseInsensitive {
			query += " AND LOWER(t.arguments_json) LIKE LOWER(?)"
		} else {
			query += " AND t.arguments_json LIKE ?"
		}
		args = append(args, pattern)
	}

	query += " ORDER BY t.timestamp DESC LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var result QueryResult
		var name, cwd, repoRoot, argumentsJSON, resultText sql.NullString
		if err := rows.Scan(
			&result.SessionID,
			&name,
			&cwd,
			&repoRoot,
			&result.EntryID,
			&result.ToolCallID,
			&result.ToolName,
			&result.Timestamp,
			&argumentsJSON,
			&resultText,
		); err != nil {
			return nil, err
		}
		result.Kind = "tool_call"
		result.SessionName = name.String
		result.CWD = cwd.String
		result.RepoRoot = repoRoot.String
		result.ArgumentsJSON = argumentsJSON.String
		result.Command = extractCommand(argumentsJSON.String)
		result.Text = compactText(resultText.String)

		// Add highlighting for command matches
		if options.CommandContains != "" {
			result.MatchRanges = highlightSubstring(result.Command, options.CommandContains, options.CaseInsensitive)
		} else if options.CommandStartsWith != "" {
			result.MatchRanges = highlightSubstring(result.Command, options.CommandStartsWith, options.CaseInsensitive)
		} else if options.OutputContains != "" {
			result.MatchRanges = highlightSubstring(resultText.String, options.OutputContains, options.CaseInsensitive)
		}

		results = append(results, result)
	}
	return results, rows.Err()
}

func extractCommand(argumentsJSON string) string {
	if argumentsJSON == "" {
		return ""
	}
	// Simple extraction: look for "command":"..."
	// Handle escaped quotes (\") inside the value.
	const key = `"command":"`
	start := strings.Index(argumentsJSON, key)
	if start == -1 {
		return ""
	}
	start += len(key)
	// Scan forward, skipping \" sequences.
	i := start
	for i < len(argumentsJSON) {
		if argumentsJSON[i] == '\\' && i+1 < len(argumentsJSON) {
			i += 2 // skip escaped char
			continue
		}
		if argumentsJSON[i] == '"' {
			break
		}
		i++
	}
	return argumentsJSON[start:i]
}

func queryText(ctx context.Context, database *sql.DB, options QueryOptions) ([]QueryResult, error) {
	query := `
		SELECT c.uuid, c.name, c.cwd, c.repo_root, m.entry_id, m.timestamp,
		       m.type, m.role, m.provider, m.model,
		       snippet(fts_pi_messages, 0, '[', ']', '...', 16)
		FROM fts_pi_messages
		JOIN pi_messages m ON m.id = fts_pi_messages.rowid
		JOIN pi_chats c ON c.id = m.chat_id
		WHERE c.file_deleted_at IS NULL AND fts_pi_messages MATCH ?`
	args := []interface{}{options.Text}
	query, args = appendCommonFilters(query, args, options, "c", "m")
	query += " ORDER BY m.timestamp DESC LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var result QueryResult
		var name, cwd, repoRoot, role, provider, model, text sql.NullString
		if err := rows.Scan(
			&result.SessionID,
			&name,
			&cwd,
			&repoRoot,
			&result.EntryID,
			&result.Timestamp,
			&result.Type,
			&role,
			&provider,
			&model,
			&text,
		); err != nil {
			return nil, err
		}
		result.Kind = "message"
		result.SessionName = name.String
		result.CWD = cwd.String
		result.RepoRoot = repoRoot.String
		result.Role = role.String
		result.Provider = provider.String
		result.Model = model.String
		result.Snippet = text.String
		result.MatchRanges = extractBracketRanges(text.String, '[', ']')
		result.Text = compactText(stripBrackets(text.String, '[', ']'))
		results = append(results, result)
	}
	return results, rows.Err()
}

// extractBracketRanges finds all regions enclosed by open/close brackets.
func extractBracketRanges(s string, open, close byte) []MatchRange {
	var ranges []MatchRange
	i := 0
	for i < len(s) {
		start := strings.IndexByte(s[i:], open)
		if start == -1 {
			break
		}
		start += i
		end := strings.IndexByte(s[start+1:], close)
		if end == -1 {
			break
		}
		end += start + 1
		ranges = append(ranges, MatchRange{Start: start, End: end - 1})
		i = end + 1
	}
	return ranges
}

// stripBrackets removes bracket delimiters from a string.
func stripBrackets(s string, open, close byte) string {
	var result strings.Builder
	result.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != open && s[i] != close {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// highlightSubstring finds all occurrences of substr in text and returns match ranges.
func highlightSubstring(text, substr string, caseInsensitive bool) []MatchRange {
	if substr == "" {
		return nil
	}
	var ranges []MatchRange
	searchText := text
	searchSubstr := substr
	if caseInsensitive {
		searchText = strings.ToLower(text)
		searchSubstr = strings.ToLower(substr)
	}
	i := 0
	for i <= len(searchText)-len(searchSubstr) {
		idx := strings.Index(searchText[i:], searchSubstr)
		if idx == -1 {
			break
		}
		start := i + idx
		end := start + len(searchSubstr)
		ranges = append(ranges, MatchRange{Start: start, End: end})
		i = end
	}
	return ranges
}

func queryMessages(ctx context.Context, database *sql.DB, options QueryOptions) ([]QueryResult, error) {
	query := `
		SELECT c.uuid, c.name, c.cwd, c.repo_root, m.entry_id, m.timestamp,
		       m.type, m.role, m.provider, m.model, m.text
		FROM pi_messages m
		JOIN pi_chats c ON c.id = m.chat_id
		WHERE c.file_deleted_at IS NULL`
	var args []interface{}
	query, args = appendCommonFilters(query, args, options, "c", "m")
	query += " ORDER BY m.timestamp DESC LIMIT ?"
	args = append(args, limitOrDefault(options.Limit))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var result QueryResult
		var name, cwd, repoRoot, role, provider, model, text sql.NullString
		if err := rows.Scan(
			&result.SessionID,
			&name,
			&cwd,
			&repoRoot,
			&result.EntryID,
			&result.Timestamp,
			&result.Type,
			&role,
			&provider,
			&model,
			&text,
		); err != nil {
			return nil, err
		}
		result.Kind = "message"
		result.SessionName = name.String
		result.CWD = cwd.String
		result.RepoRoot = repoRoot.String
		result.Role = role.String
		result.Provider = provider.String
		result.Model = model.String
		result.Text = compactText(text.String)
		results = append(results, result)
	}
	return results, rows.Err()
}

func appendCommonFilters(query string, args []interface{}, options QueryOptions, chatAlias string, messageAlias string) (string, []interface{}) {
	if options.SessionID != "" {
		query += " AND " + chatAlias + ".uuid = ?"
		args = append(args, options.SessionID)
	}
	if options.Repo != "" {
		query += " AND " + chatAlias + ".repo_root = ?"
		args = append(args, options.Repo)
	}
	if options.Provider != "" {
		query += " AND " + chatAlias + ".provider = ?"
		args = append(args, options.Provider)
	}
	if options.Model != "" {
		query += " AND " + chatAlias + ".model = ?"
		args = append(args, options.Model)
	}
	entryType := resolveEntryType(options)
	if entryType != "" {
		query += " AND " + messageAlias + ".type = ?"
		args = append(args, entryType)
	}
	if options.Role != "" {
		query += " AND " + messageAlias + ".role = ?"
		args = append(args, options.Role)
	}
	if options.EntryID != "" {
		query += " AND " + messageAlias + ".entry_id = ?"
		args = append(args, options.EntryID)
	}
	if options.Since != "" {
		query += " AND " + messageAlias + ".timestamp >= ?"
		args = append(args, options.Since)
	}
	if options.Until != "" {
		query += " AND " + messageAlias + ".timestamp <= ?"
		args = append(args, options.Until)
	}
	return query, args
}

func limitOrDefault(limit int) int {
	if limit <= 0 {
		return 50
	}
	return limit
}

func compactText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 240 {
		return value
	}
	return value[:237] + "..."
}
