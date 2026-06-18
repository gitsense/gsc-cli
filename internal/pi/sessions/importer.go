/**
 * Component: Pi Sessions Importer
 * Block-UUID: 96bb35de-5c03-4ced-9ec7-18df3db71aec
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Imports Pi session JSONL files into the phase-one SQLite mirror with derived tool and file indexes.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
)

func Sync(ctx context.Context, options SyncOptions) (SyncResult, error) {
	if options.SessionsDir == "" {
		return SyncResult{}, fmt.Errorf("sessions dir is required")
	}
	if options.DBPath == "" {
		return SyncResult{}, fmt.Errorf("db path is required")
	}
	sessionsDir, err := filepath.Abs(options.SessionsDir)
	if err != nil {
		return SyncResult{}, err
	}
	database, err := openMirror(ctx, options.DBPath)
	if err != nil {
		return SyncResult{}, err
	}
	defer db.CloseDB(database)

	result := SyncResult{SessionsDir: sessionsDir, DBPath: options.DBPath}
	files, err := discoverSessionFiles(sessionsDir)
	if err != nil {
		return result, err
	}
	result.FilesScanned = len(files)

	for _, path := range files {
		imported, err := importSessionFile(ctx, database, path)
		if err != nil {
			result.Errors = append(result.Errors, SyncError{Path: path, Error: err.Error()})
			continue
		}
		result.ChatsImported++
		result.MessagesImported += imported.messages
		result.ToolCallsImported += imported.toolCalls
		result.FileRefsImported += imported.fileRefs
	}
	if err := resolveParentChats(ctx, database); err != nil {
		return result, err
	}
	return result, nil
}

type importCounts struct {
	messages  int
	toolCalls int
	fileRefs  int
}

type messageInsert struct {
	rowID int64
	entry parsedEntry
	text  string
}

type toolResultInfo struct {
	messageID int64
	entryID   string
	isError   bool
	text      string
}

func discoverSessionFiles(root string) ([]string, error) {
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func importSessionFile(ctx context.Context, database *sql.DB, path string) (importCounts, error) {
	parsed, err := parseSessionFile(path)
	if err != nil {
		return importCounts{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return importCounts{}, err
	}
	sessionFile, err := filepath.Abs(path)
	if err != nil {
		return importCounts{}, err
	}
	repoRoot := findRepoRoot(parsed.header.CWD)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	contentHash := hashFileContent(parsed.header.RawLine, parsed.entries)

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return importCounts{}, err
	}
	defer tx.Rollback()

	chatID, oldChatID, err := upsertChat(ctx, tx, parsed, sessionFile, repoRoot, info, contentHash, now)
	if err != nil {
		return importCounts{}, err
	}
	if oldChatID != 0 {
		if err := deleteChatDerivedRows(ctx, tx, oldChatID); err != nil {
			return importCounts{}, err
		}
	}

	messages, results, err := insertMessages(ctx, tx, chatID, parsed.entries, now)
	if err != nil {
		return importCounts{}, err
	}
	counts, err := insertDerivedRows(ctx, tx, chatID, parsed.header.CWD, repoRoot, messages, results)
	if err != nil {
		return importCounts{}, err
	}
	if err := updateChatDerivedMetadata(ctx, tx, chatID, parsed.entries, counts, now); err != nil {
		return importCounts{}, err
	}
	if err := tx.Commit(); err != nil {
		return importCounts{}, err
	}
	return importCounts{messages: len(messages), toolCalls: counts.toolCalls, fileRefs: counts.fileRefs}, nil
}

func hashFileContent(header string, entries []parsedEntry) string {
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, header)
	for _, entry := range entries {
		lines = append(lines, entry.RawLine)
	}
	return hashText(strings.Join(lines, "\n") + "\n")
}

func upsertChat(ctx context.Context, tx *sql.Tx, parsed parsedSession, sessionFile string, repoRoot string, info os.FileInfo, contentHash string, now string) (int64, int64, error) {
	var existingID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM pi_chats WHERE uuid = ? OR session_file = ? ORDER BY id LIMIT 1", parsed.header.UUID, sessionFile).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}

	parentSession := sql.NullString{}
	if parsed.header.ParentSession != "" {
		parentSession = sql.NullString{String: parsed.header.ParentSession, Valid: true}
	}

	if existingID == 0 {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO pi_chats (
				uuid, version, cwd, session_file, parent_session_file, repo_root, file_size,
				file_mtime_ms, header_hash, content_hash, synced_seq, synced_byte_offset,
				last_synced_at, sync_status, last_ingest_started_at, last_ingest_completed_at,
				raw_header, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'idle', ?, ?, ?, ?, ?)`,
			parsed.header.UUID,
			parsed.header.Version,
			nullString(parsed.header.CWD),
			sessionFile,
			parentSession,
			nullString(repoRoot),
			info.Size(),
			info.ModTime().UnixMilli(),
			hashText(parsed.header.RawLine),
			contentHash,
			len(parsed.entries)-1,
			info.Size(),
			now,
			now,
			now,
			parsed.header.RawLine,
			parsed.header.Timestamp,
			now,
		)
		if err != nil {
			return 0, 0, err
		}
		chatID, err := result.LastInsertId()
		return chatID, 0, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE pi_chats SET
			uuid = ?, version = ?, cwd = ?, session_file = ?, parent_session_file = ?,
			parent_chat_id = NULL, repo_root = ?, file_size = ?, file_mtime_ms = ?,
			header_hash = ?, content_hash = ?, file_deleted_at = NULL,
			synced_seq = ?, synced_byte_offset = ?, last_synced_at = ?,
			sync_status = 'idle', sync_error = NULL, last_ingest_started_at = ?,
			last_ingest_completed_at = ?, raw_header = ?, created_at = ?, updated_at = ?
		WHERE id = ?`,
		parsed.header.UUID,
		parsed.header.Version,
		nullString(parsed.header.CWD),
		sessionFile,
		parentSession,
		nullString(repoRoot),
		info.Size(),
		info.ModTime().UnixMilli(),
		hashText(parsed.header.RawLine),
		contentHash,
		len(parsed.entries)-1,
		info.Size(),
		now,
		now,
		now,
		parsed.header.RawLine,
		parsed.header.Timestamp,
		now,
		existingID,
	)
	if err != nil {
		return 0, 0, err
	}
	return existingID, existingID, nil
}

func deleteChatDerivedRows(ctx context.Context, tx *sql.Tx, chatID int64) error {
	for _, statement := range []string{
		"DELETE FROM pi_file_refs WHERE chat_id = ?",
		"DELETE FROM pi_tool_calls WHERE chat_id = ?",
		"DELETE FROM pi_messages WHERE chat_id = ?",
	} {
		if _, err := tx.ExecContext(ctx, statement, chatID); err != nil {
			return err
		}
	}
	return nil
}

func insertMessages(ctx context.Context, tx *sql.Tx, chatID int64, entries []parsedEntry, now string) ([]messageInsert, map[string]toolResultInfo, error) {
	messages := make([]messageInsert, 0, len(entries))
	results := make(map[string]toolResultInfo)
	for _, entry := range entries {
		text := flattenMessageText(entry.Message)
		role := ""
		model := ""
		provider := ""
		if entry.Message != nil {
			role = entry.Message.Role
			model = entry.Message.Model
			provider = entry.Message.Provider
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO pi_messages (
				chat_id, entry_id, parent_entry_id, seq, type, role, model, provider,
				text, raw_line, raw_hash, timestamp, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chatID,
			entry.ID,
			nullStringPtr(entry.ParentID),
			entry.Seq,
			entry.Type,
			nullString(role),
			nullString(model),
			nullString(provider),
			nullString(text),
			entry.RawLine,
			entry.RawHash,
			entry.Timestamp,
			now,
		)
		if err != nil {
			return nil, nil, err
		}
		messageID, err := result.LastInsertId()
		if err != nil {
			return nil, nil, err
		}
		messages = append(messages, messageInsert{rowID: messageID, entry: entry, text: text})
		if entry.Message != nil && entry.Message.Role == "toolResult" && entry.Message.ToolCallID != "" {
			results[entry.Message.ToolCallID] = toolResultInfo{
				messageID: messageID,
				entryID:   entry.ID,
				isError:   entry.Message.IsError,
				text:      text,
			}
		}
	}
	return messages, results, nil
}

type derivedCounts struct {
	toolCalls int
	fileRefs  int
}

func insertDerivedRows(ctx context.Context, tx *sql.Tx, chatID int64, cwd string, repoRoot string, messages []messageInsert, results map[string]toolResultInfo) (derivedCounts, error) {
	counts := derivedCounts{}
	seenRefs := make(map[string]struct{})
	for _, message := range messages {
		entry := message.entry
		if entry.Type == "message" && entry.Message != nil && entry.Message.Role == "assistant" {
			for blockIndex, block := range entry.Message.Content {
				if block.Type != "toolCall" || block.ID == "" {
					continue
				}
				resultInfo, hasResult := results[block.ID]
				var resultMessageID sql.NullInt64
				var resultEntryID sql.NullString
				var isError sql.NullInt64
				var resultText sql.NullString
				if hasResult {
					resultMessageID = sql.NullInt64{Int64: resultInfo.messageID, Valid: true}
					resultEntryID = sql.NullString{String: resultInfo.entryID, Valid: true}
					if resultInfo.isError {
						isError = sql.NullInt64{Int64: 1, Valid: true}
					} else {
						isError = sql.NullInt64{Int64: 0, Valid: true}
					}
					resultText = nullString(resultInfo.text)
				}
				argumentsJSON := "{}"
				if len(block.Arguments) > 0 {
					argumentsJSON = string(block.Arguments)
				}
				_, err := tx.ExecContext(ctx, `
					INSERT INTO pi_tool_calls (
						chat_id, message_id, entry_id, block_index, tool_call_id, tool_name,
						arguments_json, result_message_id, result_entry_id, is_error,
						result_text, seq, timestamp
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					chatID,
					message.rowID,
					entry.ID,
					blockIndex,
					block.ID,
					strings.ToLower(block.Name),
					argumentsJSON,
					resultMessageID,
					resultEntryID,
					isError,
					resultText,
					entry.Seq,
					entry.Timestamp,
				)
				if err != nil {
					return counts, err
				}
				counts.toolCalls++
				if isFileRefTool(block.Name) {
					rawPath := toolArgumentPath(block.Arguments)
					if rawPath != "" {
						inserted, err := insertFileRef(ctx, tx, seenRefs, chatID, message.rowID, entry.ID, block.ID, "tool_call", strings.ToLower(block.Name), strings.ToLower(block.Name), rawPath, cwd, repoRoot, entry.Timestamp)
						if err != nil {
							return counts, err
						}
						if inserted {
							counts.fileRefs++
						}
					}
				}
			}
		}
		if entry.Type == "compaction" {
			refs, err := insertAggregateRefs(ctx, tx, seenRefs, chatID, message.rowID, entry, "compaction", cwd, repoRoot)
			if err != nil {
				return counts, err
			}
			counts.fileRefs += refs
		}
		if entry.Type == "branch_summary" {
			refs, err := insertAggregateRefs(ctx, tx, seenRefs, chatID, message.rowID, entry, "branch_summary", cwd, repoRoot)
			if err != nil {
				return counts, err
			}
			counts.fileRefs += refs
		}
	}
	return counts, nil
}

func isFileRefTool(name string) bool {
	switch strings.ToLower(name) {
	case "read", "edit", "write":
		return true
	default:
		return false
	}
}

func toolArgumentPath(arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return ""
	}
	var values map[string]interface{}
	if err := json.Unmarshal(arguments, &values); err != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path"} {
		if value, ok := values[key].(string); ok {
			return value
		}
	}
	return ""
}

func insertAggregateRefs(ctx context.Context, tx *sql.Tx, seen map[string]struct{}, chatID int64, messageID int64, entry parsedEntry, source string, cwd string, repoRoot string) (int, error) {
	count := 0
	readOp := source + "_read"
	modifiedOp := source + "_modified"
	for _, rawPath := range entry.Details.ReadFiles {
		inserted, err := insertFileRef(ctx, tx, seen, chatID, messageID, entry.ID, "", source, readOp, "", rawPath, cwd, repoRoot, entry.Timestamp)
		if err != nil {
			return count, err
		}
		if inserted {
			count++
		}
	}
	for _, rawPath := range entry.Details.ModifiedFiles {
		inserted, err := insertFileRef(ctx, tx, seen, chatID, messageID, entry.ID, "", source, modifiedOp, "", rawPath, cwd, repoRoot, entry.Timestamp)
		if err != nil {
			return count, err
		}
		if inserted {
			count++
		}
	}
	return count, nil
}

func insertFileRef(ctx context.Context, tx *sql.Tx, seen map[string]struct{}, chatID int64, messageID int64, entryID string, toolCallID string, source string, op string, toolName string, rawPath string, cwd string, repoRoot string, timestamp string) (bool, error) {
	normalized := normalizePath(rawPath, cwd, repoRoot)
	key := fmt.Sprintf("%d\x00%d\x00%s\x00%s\x00%s\x00%s\x00%s", chatID, messageID, source, op, toolCallID, normalized.absPath, normalized.filePathRel)
	if _, ok := seen[key]; ok {
		return false, nil
	}
	seen[key] = struct{}{}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO pi_file_refs (
			chat_id, message_id, entry_id, tool_call_id, source, op, tool_name,
			raw_path, abs_path, repo_root, file_path_rel, cwd_rel_path, confidence, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'high', ?)`,
		chatID,
		messageID,
		entryID,
		nullString(toolCallID),
		source,
		op,
		nullString(toolName),
		normalized.rawPath,
		nullString(normalized.absPath),
		nullString(normalized.repoRoot),
		nullString(normalized.filePathRel),
		nullString(normalized.cwdRelPath),
		timestamp,
	)
	return err == nil, err
}

func updateChatDerivedMetadata(ctx context.Context, tx *sql.Tx, chatID int64, entries []parsedEntry, counts derivedCounts, now string) error {
	var name string
	var provider string
	var model string
	var firstUser string
	var lastUser string
	var currentLeaf string
	var lastMessageAt string

	for _, entry := range entries {
		currentLeaf = entry.ID
		lastMessageAt = entry.Timestamp
		if entry.Type == "session_info" && entry.Name != "" {
			name = entry.Name
		}
		if entry.Message == nil {
			continue
		}
		if entry.Message.Role == "assistant" {
			if entry.Message.Provider != "" {
				provider = entry.Message.Provider
			}
			if entry.Message.Model != "" {
				model = entry.Message.Model
			}
		}
		if entry.Message.Role == "user" {
			text := flattenMessageText(entry.Message)
			if firstUser == "" {
				firstUser = text
			}
			lastUser = text
		}
	}

	_, err := tx.ExecContext(ctx, `
		UPDATE pi_chats SET
			name = ?, current_leaf_id = ?, provider = ?, model = ?,
			first_user_text = ?, last_user_text = ?, message_count = ?,
			tool_call_count = ?, file_ref_count = ?, last_message_at = ?, updated_at = ?
		WHERE id = ?`,
		nullString(name),
		nullString(currentLeaf),
		nullString(provider),
		nullString(model),
		nullString(firstUser),
		nullString(lastUser),
		len(entries),
		counts.toolCalls,
		counts.fileRefs,
		nullString(lastMessageAt),
		now,
		chatID,
	)
	return err
}

func resolveParentChats(ctx context.Context, database *sql.DB) error {
	_, err := database.ExecContext(ctx, `
		UPDATE pi_chats
		SET parent_chat_id = (
			SELECT parent.id
			FROM pi_chats parent
			WHERE parent.session_file = pi_chats.parent_session_file
			   OR parent.uuid = substr(pi_chats.parent_session_file, instr(pi_chats.parent_session_file, '_') + 1, 36)
			LIMIT 1
		)
		WHERE parent_session_file IS NOT NULL AND parent_session_file != ''`)
	return err
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullStringPtr(value *string) sql.NullString {
	if value == nil || *value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
