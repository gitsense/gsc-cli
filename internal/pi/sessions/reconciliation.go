/**
 * Component: Pi Sessions Reconciliation
 * Block-UUID: 9b4d03cd-7390-496b-89f6-0d9986049b79
 * Parent-UUID: 96bb35de-5c03-4ced-9ec7-18df3db71aec
 * Version: 1.0.0
 * Description: Reassociates moved Pi sessions and purges content after guarded missing-file confirmation.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const deletionConfirmDelay = 200 * time.Millisecond

type missingChat struct {
	id          int64
	uuid        string
	sessionFile string
	fileSize    sql.NullInt64
	fileMtimeMS sql.NullInt64
	contentHash sql.NullString
	syncedSeq   int
}

func reassociateMovedSession(ctx context.Context, database *sql.DB, state chatSyncState, sessionFile string) error {
	if state.sessionFile == "" || state.sessionFile == sessionFile {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE pi_chats SET session_file = ?, file_deleted_at = NULL,
			sync_status = 'idle', sync_error = NULL, updated_at = ?
		WHERE id = ?`, sessionFile, now, state.id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pi_sync_events (
			chat_id, session_file, event_type, detected_at, old_size, new_size,
			old_mtime_ms, new_mtime_ms, old_hash, new_hash, old_seq, new_seq, note
		) VALUES (?, ?, 'file_moved', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		state.id, sessionFile, now, state.fileSize, state.fileSize,
		state.fileMtimeMS, state.fileMtimeMS, nullString(state.contentHash),
		nullString(state.contentHash), state.syncedSeq, state.syncedSeq,
		fmt.Sprintf("reassociated from %s", state.sessionFile),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func recordRestoredSession(ctx context.Context, database *sql.DB, state chatSyncState, sessionFile string, info os.FileInfo) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := database.ExecContext(ctx, `
		INSERT INTO pi_sync_events (
			chat_id, session_file, event_type, detected_at, old_size, new_size,
			old_mtime_ms, new_mtime_ms, old_hash, new_hash, old_seq, new_seq, note
		) VALUES (?, ?, 'file_restored', ?, ?, ?, ?, ?, ?, NULL, ?, NULL, ?)`,
		state.id, sessionFile, now, state.fileSize, info.Size(), state.fileMtimeMS,
		info.ModTime().UnixMilli(), nullString(state.contentHash), state.syncedSeq,
		"same UUID restored and re-ingested from byte zero",
	)
	return err
}

func reconcileMissingSessions(ctx context.Context, database *sql.DB, sessionsDir string, files []string) error {
	seenPaths := make(map[string]struct{}, len(files))
	for _, path := range files {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		seenPaths[absolute] = struct{}{}
	}

	rows, err := database.QueryContext(ctx, `
		SELECT id, uuid, session_file, file_size, file_mtime_ms, content_hash, synced_seq
		FROM pi_chats WHERE file_deleted_at IS NULL`)
	if err != nil {
		return err
	}
	var candidates []missingChat
	for rows.Next() {
		var candidate missingChat
		if err := rows.Scan(
			&candidate.id, &candidate.uuid, &candidate.sessionFile,
			&candidate.fileSize, &candidate.fileMtimeMS, &candidate.contentHash, &candidate.syncedSeq,
		); err != nil {
			rows.Close()
			return err
		}
		if _, ok := seenPaths[candidate.sessionFile]; ok {
			continue
		}
		if _, err := os.Stat(candidate.sessionFile); err == nil || !os.IsNotExist(err) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	timer := time.NewTimer(deletionConfirmDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	currentUUIDs, err := discoverSessionUUIDs(sessionsDir)
	if err != nil {
		return err
	}
	var confirmed []missingChat
	for _, candidate := range candidates {
		if _, ok := currentUUIDs[candidate.uuid]; ok {
			continue
		}
		if _, err := os.Stat(candidate.sessionFile); err == nil || !os.IsNotExist(err) {
			continue
		}
		confirmed = append(confirmed, candidate)
	}
	if len(confirmed) == 0 {
		return nil
	}
	if err := purgeMissingSessions(ctx, database, confirmed); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO fts_pi_messages(fts_pi_messages) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuild Pi sessions FTS after purge: %w", err)
	}
	if _, err := database.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("truncate Pi sessions WAL after purge: %w", err)
	}
	return nil
}

func discoverSessionUUIDs(root string) (map[string]string, error) {
	files, err := discoverSessionFiles(root)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(files))
	for _, path := range files {
		header, _, err := parseSessionHeader(path)
		if err != nil {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		result[header.UUID] = absolute
	}
	return result, nil
}

func purgeMissingSessions(ctx context.Context, database *sql.DB, candidates []missingChat) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, candidate := range candidates {
		if err := deleteChatDerivedRows(ctx, tx, candidate.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE pi_chats SET
				name = NULL, current_leaf_id = NULL, provider = NULL, model = NULL,
				first_user_text = NULL, last_user_text = NULL,
				message_count = 0, tool_call_count = 0, file_ref_count = 0,
				last_message_at = NULL, file_deleted_at = ?, sync_status = 'pending_delete',
				sync_error = NULL, updated_at = ?
			WHERE id = ?`, now, now, candidate.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pi_sync_events (
				chat_id, session_file, event_type, detected_at, old_size, new_size,
				old_mtime_ms, new_mtime_ms, old_hash, new_hash, old_seq, new_seq, note
			) VALUES (?, ?, 'file_deleted', ?, ?, NULL, ?, NULL, ?, NULL, ?, NULL, ?)`,
			candidate.id, candidate.sessionFile, now, candidate.fileSize,
			candidate.fileMtimeMS, candidate.contentHash, candidate.syncedSeq,
			"confirmed missing after debounce, re-stat, and UUID rescan",
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
