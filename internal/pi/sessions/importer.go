/**
 * Component: Pi Sessions Importer
 * Block-UUID: 4e8a1c70-2d96-4b53-9a18-5f0c7b3e9d62
 * Parent-UUID: 96bb35de-5c03-4ced-9ec7-18df3db71aec
 * Version: 1.2.0
 * Description: Reconciles Pi session JSONL files into SQLite using atomic append-or-rebuild ingestion; skips Pi's ephemeral pi-runtime-cwd sessions so they never enter the mirror.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0, v1.1.0), claude-opus-4-8 (v1.2.0)
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

	logger := options.Logger
	result := SyncResult{SessionsDir: sessionsDir, DBPath: options.DBPath}
	files, err := discoverSessionFiles(sessionsDir)
	if err != nil {
		return result, err
	}
	result.FilesScanned = len(files)

	if logger != nil {
		logger.LogDebug(fmt.Sprintf("found %d session files in %s", len(files), sessionsDir))
	}

	for _, path := range files {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("importing session file: %s", path))
		}
		imported, err := importSessionFile(ctx, database, path, logger)
		if err != nil {
			if logger != nil {
				logger.LogError(fmt.Sprintf("%s: %s", path, err.Error()))
			}
			result.Errors = append(result.Errors, SyncError{Path: path, Error: err.Error()})
			continue
		}
		if imported.sessionChanged {
			result.SessionsImported++
		}
		result.MessagesImported += imported.messages
		result.ToolCallsImported += imported.toolCalls
		result.FileRefsImported += imported.fileRefs
	}
	// Clear any ephemeral sessions imported before the skip filter existed.
	if purged, err := purgeEphemeralSessions(ctx, database); err != nil {
		return result, err
	} else if purged > 0 && logger != nil {
		logger.LogInfo(fmt.Sprintf("purged %d ephemeral runtime session(s) from the mirror", purged))
	}
	if err := reconcileMissingSessions(ctx, database, sessionsDir, files); err != nil {
		return result, err
	}
	if err := resolveParentChats(ctx, database); err != nil {
		return result, err
	}
	return result, nil
}

// purgeEphemeralSessions hard-deletes already-imported sessions whose cwd is an
// ephemeral Pi runtime/temp directory. Skip-on-import keeps new ones out; this
// removes rows imported before the filter existed. Returns the count removed.
func purgeEphemeralSessions(ctx context.Context, database *sql.DB) (int, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, cwd FROM pi_chats`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var cwd sql.NullString
		if err := rows.Scan(&id, &cwd); err != nil {
			rows.Close()
			return 0, err
		}
		if isEphemeralCWD(cwd.String) {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if err := deleteChatDerivedRows(ctx, tx, id); err != nil { // messages/tool_calls/file_refs (FTS via trigger)
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pi_sync_events WHERE chat_id = ?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pi_chats WHERE id = ?`, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO fts_pi_messages(fts_pi_messages) VALUES('rebuild')`); err != nil {
		return len(ids), fmt.Errorf("rebuild Pi sessions FTS after ephemeral purge: %w", err)
	}
	return len(ids), nil
}

type importCounts struct {
	sessionChanged bool
	messages       int
	toolCalls      int
	fileRefs       int
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

type chatSyncState struct {
	id               int64
	sessionFile      string
	syncedSeq        int
	syncedByteOffset int64
	fileSize         int64
	fileMtimeMS      int64
	headerHash       string
	contentHash      string
	fileDeleted      bool
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

func importSessionFile(ctx context.Context, database *sql.DB, path string, logger SyncLogger) (importCounts, error) {
	info, err := os.Stat(path)
	if err != nil {
		return importCounts{}, err
	}
	sessionFile, err := filepath.Abs(path)
	if err != nil {
		return importCounts{}, err
	}
	header, headerBytes, err := parseSessionHeader(path)
	if err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("parse header failed for %s: %v", path, err))
		}
		return importCounts{}, err
	}
	if logger != nil {
		logger.LogDebug(fmt.Sprintf("parsed header: uuid=%s, version=%d, entries=%d", header.UUID, header.Version, headerBytes))
	}
	// Skip Pi's ephemeral runtime working directories (pi-runtime-cwd-*); these
	// are RPC/subagent scratch sessions, not real user sessions.
	if isEphemeralCWD(header.CWD) {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("skipping ephemeral runtime session: %s (cwd=%s)", path, header.CWD))
		}
		return importCounts{}, nil
	}
	state, found, err := loadChatSyncState(ctx, database, header.UUID, sessionFile)
	if err != nil {
		return importCounts{}, err
	}
	if found && !state.fileDeleted && state.sessionFile != sessionFile {
		if err := reassociateMovedSession(ctx, database, state, sessionFile); err != nil {
			return importCounts{}, err
		}
	}
	if found && !state.fileDeleted && state.headerHash == hashText(header.RawLine) && state.syncedByteOffset >= headerBytes && state.syncedByteOffset <= info.Size() {
		anchorOK, err := validateTailAnchor(ctx, database, path, state)
		if err != nil {
			return importCounts{}, err
		}
		if anchorOK {
			if state.syncedByteOffset < info.Size() {
				entries, newOffset, partial, err := parseSessionTail(path, state.syncedByteOffset, state.syncedSeq+1)
				if err != nil {
					return importCounts{}, err
				}
				if len(entries) == 0 {
					if err := updateIncompleteTailState(ctx, database, state.id, info, partial); err != nil {
						return importCounts{}, err
					}
					return importCounts{}, nil
				}
				return appendSessionEntries(ctx, database, state, header, sessionFile, info, entries, newOffset, partial)
			}
			if info.Size() == state.fileSize {
				if info.ModTime().UnixMilli() == state.fileMtimeMS {
					return importCounts{}, nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return importCounts{}, err
				}
				if hashText(string(content)) == state.contentHash {
					if err := updateIncompleteTailState(ctx, database, state.id, info, false); err != nil {
						return importCounts{}, err
					}
					return importCounts{}, nil
				}
			}
		}
	}
	if logger != nil {
		logger.LogDebug(fmt.Sprintf("rebuilding session file: %s", path))
	}
	counts, err := rebuildSessionFile(ctx, database, path, sessionFile, info, logger)
	if err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("rebuild failed for %s: %v", path, err))
		}
		return importCounts{}, err
	}
	if found && state.fileDeleted {
		if err := recordRestoredSession(ctx, database, state, sessionFile, info); err != nil {
			return importCounts{}, err
		}
	}
	return counts, nil
}

func rebuildSessionFile(ctx context.Context, database *sql.DB, path string, sessionFile string, info os.FileInfo, logger SyncLogger) (importCounts, error) {
	parsed, err := parseSessionFile(path)
	if err != nil {
		return importCounts{}, err
	}
	repoRoot := findRepoRoot(parsed.header.CWD)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	contentHash := hashFileContent(parsed.header.RawLine, parsed.entries)

	if logger != nil {
		logger.LogDebug(fmt.Sprintf("rebuilding session: uuid=%s, entries=%d", parsed.header.UUID, len(parsed.entries)))
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return importCounts{}, err
	}
	defer tx.Rollback()

	if logger != nil {
		logger.LogDebug("upserting chat record")
	}
	chatID, oldChatID, err := upsertChat(ctx, tx, parsed, sessionFile, repoRoot, info, contentHash, now)
	if err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("upsertChat failed: %v", err))
		}
		return importCounts{}, err
	}
	if oldChatID != 0 {
		if err := deleteChatDerivedRows(ctx, tx, oldChatID); err != nil {
			return importCounts{}, err
		}
	}

	if logger != nil {
		logger.LogDebug(fmt.Sprintf("inserting %d messages", len(parsed.entries)))
	}
	messages, results, err := insertMessages(ctx, tx, chatID, parsed.entries, now)
	if err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("insertMessages failed: %v", err))
		}
		return importCounts{}, err
	}
	if logger != nil {
		logger.LogDebug("inserting derived rows")
	}
	counts, err := insertDerivedRows(ctx, tx, chatID, parsed.header.CWD, repoRoot, messages, results)
	if err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("insertDerivedRows failed: %v", err))
		}
		return importCounts{}, err
	}
	if logger != nil {
		logger.LogDebug("refreshing chat metadata")
	}
	if err := refreshChatDerivedMetadata(ctx, tx, chatID, parsed.entries, now); err != nil {
		if logger != nil {
			logger.LogDebug(fmt.Sprintf("refreshChatDerivedMetadata failed: %v", err))
		}
		return importCounts{}, err
	}
	if err := tx.Commit(); err != nil {
		return importCounts{}, err
	}
	return importCounts{sessionChanged: true, messages: len(messages), toolCalls: counts.toolCalls, fileRefs: counts.fileRefs}, nil
}

func loadChatSyncState(ctx context.Context, database *sql.DB, uuid string, sessionFile string) (chatSyncState, bool, error) {
	var state chatSyncState
	var syncedSeq, syncedOffset, fileSize, fileMtime sql.NullInt64
	var headerHash, contentHash, fileDeletedAt sql.NullString
	err := database.QueryRowContext(ctx, `
		SELECT id, session_file, synced_seq, synced_byte_offset, file_size, file_mtime_ms,
			header_hash, content_hash, file_deleted_at
		FROM pi_chats WHERE uuid = ? OR session_file = ?
		ORDER BY CASE WHEN uuid = ? THEN 0 ELSE 1 END, id LIMIT 1`, uuid, sessionFile, uuid).Scan(
		&state.id, &state.sessionFile, &syncedSeq, &syncedOffset, &fileSize, &fileMtime,
		&headerHash, &contentHash, &fileDeletedAt,
	)
	if err == sql.ErrNoRows {
		return chatSyncState{}, false, nil
	}
	if err != nil {
		return chatSyncState{}, false, err
	}
	state.syncedSeq = int(syncedSeq.Int64)
	state.syncedByteOffset = syncedOffset.Int64
	state.fileSize = fileSize.Int64
	state.fileMtimeMS = fileMtime.Int64
	state.headerHash = headerHash.String
	state.contentHash = contentHash.String
	state.fileDeleted = fileDeletedAt.Valid
	return state, true, nil
}

func validateTailAnchor(ctx context.Context, database *sql.DB, path string, state chatSyncState) (bool, error) {
	if state.syncedSeq < 0 {
		return true, nil
	}
	var rawLine string
	var rawHash string
	err := database.QueryRowContext(ctx, `
		SELECT raw_line, raw_hash FROM pi_messages
		WHERE chat_id = ? AND seq = ?`, state.id, state.syncedSeq).Scan(&rawLine, &rawHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	anchorSize := int64(len(rawLine) + 1)
	anchorStart := state.syncedByteOffset - anchorSize
	if anchorStart < 0 {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	anchor := make([]byte, anchorSize)
	if _, err := file.ReadAt(anchor, anchorStart); err != nil {
		return false, nil
	}
	return string(anchor) == rawLine+"\n" && hashText(rawLine) == rawHash, nil
}

func hashFileContent(header string, entries []parsedEntry) string {
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, header)
	for _, entry := range entries {
		lines = append(lines, entry.RawLine)
	}
	return hashText(strings.Join(lines, "\n") + "\n")
}

func appendSessionEntries(ctx context.Context, database *sql.DB, state chatSyncState, header sessionHeader, sessionFile string, info os.FileInfo, entries []parsedEntry, newOffset int64, partial bool) (importCounts, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	repoRoot := findRepoRoot(header.CWD)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return importCounts{}, err
	}
	defer tx.Rollback()

	messages, results, err := insertMessages(ctx, tx, state.id, entries, now)
	if err != nil {
		return importCounts{}, err
	}
	if err := applyToolResults(ctx, tx, state.id, results); err != nil {
		return importCounts{}, err
	}
	counts, err := insertDerivedRows(ctx, tx, state.id, header.CWD, repoRoot, messages, results)
	if err != nil {
		return importCounts{}, err
	}
	if err := refreshChatDerivedMetadata(ctx, tx, state.id, entries, now); err != nil {
		return importCounts{}, err
	}
	status := "idle"
	if partial {
		status = "pending_partial"
	}
	lastSeq := entries[len(entries)-1].Seq
	_, err = tx.ExecContext(ctx, `
		UPDATE pi_chats SET
				uuid = ?, version = ?, cwd = ?, session_file = ?, parent_session_file = ?,
			parent_chat_id = NULL, repo_root = ?, file_size = ?, file_mtime_ms = ?,
			header_hash = ?, file_deleted_at = NULL, synced_seq = ?, synced_byte_offset = ?,
			last_synced_at = ?, sync_status = ?, sync_error = NULL,
			last_ingest_started_at = ?, last_ingest_completed_at = ?, raw_header = ?, updated_at = ?
		WHERE id = ?`,
		header.UUID,
		header.Version,
		nullString(header.CWD),
		sessionFile,
		nullString(header.ParentSession),
		nullString(repoRoot),
		info.Size(),
		info.ModTime().UnixMilli(),
		hashText(header.RawLine),
		lastSeq,
		newOffset,
		now,
		status,
		now,
		now,
		header.RawLine,
		now,
		state.id,
	)
	if err != nil {
		return importCounts{}, err
	}
	if err := tx.Commit(); err != nil {
		return importCounts{}, err
	}
	return importCounts{sessionChanged: true, messages: len(messages), toolCalls: counts.toolCalls, fileRefs: counts.fileRefs}, nil
}

func applyToolResults(ctx context.Context, tx *sql.Tx, chatID int64, results map[string]toolResultInfo) error {
	for toolCallID, result := range results {
		isError := 0
		if result.isError {
			isError = 1
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE pi_tool_calls SET result_message_id = ?, result_entry_id = ?, is_error = ?, result_text = ?
			WHERE chat_id = ? AND tool_call_id = ?`,
			result.messageID, result.entryID, isError, nullString(result.text), chatID, toolCallID,
		); err != nil {
			return err
		}
	}
	return nil
}

func updateIncompleteTailState(ctx context.Context, database *sql.DB, chatID int64, info os.FileInfo, partial bool) error {
	status := "idle"
	if partial {
		status = "pending_partial"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := database.ExecContext(ctx, `
		UPDATE pi_chats SET file_size = ?, file_mtime_ms = ?, sync_status = ?, sync_error = NULL,
			last_synced_at = ?, updated_at = ? WHERE id = ?`,
		info.Size(), info.ModTime().UnixMilli(), status, now, now, chatID,
	)
	return err
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
	status := "idle"
	if parsed.hasPartialTail {
		status = "pending_partial"
	}

	if existingID == 0 {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO pi_chats (
				uuid, version, cwd, session_file, parent_session_file, repo_root, file_size,
				file_mtime_ms, header_hash, content_hash, synced_seq, synced_byte_offset,
				last_synced_at, sync_status, last_ingest_started_at, last_ingest_completed_at,
				raw_header, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
			parsed.syncedByteOffset,
			now,
			status,
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
				uuid = ?, version = ?, cwd = ?, name = NULL, session_file = ?, parent_session_file = ?,
			parent_chat_id = NULL, repo_root = ?, file_size = ?, file_mtime_ms = ?,
			header_hash = ?, content_hash = ?, file_deleted_at = NULL,
		synced_seq = ?, synced_byte_offset = ?, last_synced_at = ?,
			sync_status = ?, sync_error = NULL, last_ingest_started_at = ?,
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
		parsed.syncedByteOffset,
		now,
		status,
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

func refreshChatDerivedMetadata(ctx context.Context, tx *sql.Tx, chatID int64, entries []parsedEntry, now string) error {
	var name string
	nameSet := 0
	for _, entry := range entries {
		if entry.Type == "session_info" && entry.Name != "" {
			name = entry.Name
			nameSet = 1
		}
	}

	_, err := tx.ExecContext(ctx, `
		UPDATE pi_chats SET
			name = CASE WHEN ? = 1 THEN ? ELSE name END,
			current_leaf_id = (SELECT entry_id FROM pi_messages WHERE chat_id = ? ORDER BY seq DESC LIMIT 1),
			provider = (SELECT provider FROM pi_messages WHERE chat_id = ? AND role = 'assistant' AND provider IS NOT NULL ORDER BY seq DESC LIMIT 1),
			model = (SELECT model FROM pi_messages WHERE chat_id = ? AND role = 'assistant' AND model IS NOT NULL ORDER BY seq DESC LIMIT 1),
			first_user_text = (SELECT text FROM pi_messages WHERE chat_id = ? AND role = 'user' ORDER BY seq ASC LIMIT 1),
			last_user_text = (SELECT text FROM pi_messages WHERE chat_id = ? AND role = 'user' ORDER BY seq DESC LIMIT 1),
			last_text = (SELECT text FROM pi_messages WHERE chat_id = ? ORDER BY seq DESC LIMIT 1),
			last_display_text = (SELECT text FROM pi_messages WHERE chat_id = ? AND type != 'session_info' AND text != '' AND text IS NOT NULL ORDER BY seq DESC LIMIT 1),
			message_count = (SELECT COUNT(*) FROM pi_messages WHERE chat_id = ?),
			tool_call_count = (SELECT COUNT(*) FROM pi_tool_calls WHERE chat_id = ?),
			file_ref_count = (SELECT COUNT(*) FROM pi_file_refs WHERE chat_id = ?),
			last_message_at = (SELECT timestamp FROM pi_messages WHERE chat_id = ? ORDER BY seq DESC LIMIT 1),
			updated_at = ?
		WHERE id = ?`,
		nameSet,
		nullString(name),
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		chatID,
		now,
		chatID,
	)
	return err
}

func resolveParentChats(ctx context.Context, database *sql.DB) error {
	type chatIdentity struct {
		id          int64
		uuid        string
		sessionFile string
		parentFile  sql.NullString
	}
	rows, err := database.QueryContext(ctx, `SELECT id, uuid, session_file, parent_session_file FROM pi_chats`)
	if err != nil {
		return err
	}
	var chats []chatIdentity
	byPath := make(map[string]int64)
	byUUID := make(map[string]int64)
	for rows.Next() {
		var chat chatIdentity
		if err := rows.Scan(&chat.id, &chat.uuid, &chat.sessionFile, &chat.parentFile); err != nil {
			rows.Close()
			return err
		}
		chats = append(chats, chat)
		byPath[chat.sessionFile] = chat.id
		byUUID[chat.uuid] = chat.id
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, chat := range chats {
		if !chat.parentFile.Valid || chat.parentFile.String == "" {
			continue
		}
		var parentID sql.NullInt64
		if id, ok := byPath[chat.parentFile.String]; ok {
			parentID = sql.NullInt64{Int64: id, Valid: true}
		} else if uuid := sessionUUIDFromPath(chat.parentFile.String); uuid != "" {
			if id, ok := byUUID[uuid]; ok {
				parentID = sql.NullInt64{Int64: id, Valid: true}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pi_chats SET parent_chat_id = ? WHERE id = ?`, parentID, chat.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func sessionUUIDFromPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	separator := strings.LastIndex(name, "_")
	if separator < 0 || separator == len(name)-1 {
		return ""
	}
	return name[separator+1:]
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
