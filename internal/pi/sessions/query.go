/**
 * Component: Pi Sessions Query Engine
 * Block-UUID: 52080e7b-c252-4955-a46e-d37be5ad6c03
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Executes phase-one discovery queries over imported Pi sessions.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
)

func hasToolCallFilters(options QueryOptions) bool {
	return options.CommandStartsWith != "" ||
		options.CommandContains != "" ||
		options.OutputContains != "" ||
		options.ToolArgsContains != ""
}

func validateToolCallOptions(options QueryOptions) error {
	if hasToolCallFilters(options) && options.Tool != "" && strings.ToLower(options.Tool) != "bash" {
		return fmt.Errorf("--command-* and --output-* flags only apply to bash tool calls; remove --tool or use --tool bash")
	}
	return nil
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

	if options.File != "" || options.AbsFile != "" || options.Op != "" {
		return queryFileRefs(ctx, database, options)
	}
	if options.Tool != "" || hasToolCallFilters(options) {
		return queryToolCalls(ctx, database, options)
	}
	if options.Text != "" {
		return queryText(ctx, database, options)
	}
	return queryMessages(ctx, database, options)
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
		result.Text = compactText(text.String)
		results = append(results, result)
	}
	return results, rows.Err()
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
	if options.Type != "" {
		query += " AND " + messageAlias + ".type = ?"
		args = append(args, options.Type)
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
