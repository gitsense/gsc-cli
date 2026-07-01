/**
 * Component: Pi Sessions Reconciliation Tests
 * Block-UUID: 8d6a20a3-8edf-4a06-a20c-fc873fbc2688
 * Parent-UUID: 96bb35de-5c03-4ced-9ec7-18df3db71aec
 * Version: 1.0.0
 * Description: Verifies session move reassociation and guarded deletion purging during reconciliation.
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
	"testing"
)

const (
	parentFixtureUUID = "019edff0-0000-7000-8000-000000000010"
	childFixtureUUID  = "019edff0-0000-7000-8000-000000000011"
)

func TestSyncReassociatesMovedSessionByUUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	oldPath := filepath.Join(sessionsDir, "old.jsonl")
	writeFixtureLines(t, oldPath, incrementalFixtureLines[:3])
	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	before := messageRowIDs(t, database)

	nestedDir := filepath.Join(sessionsDir, "nested")
	if err := os.MkdirAll(nestedDir, 0700); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	newPath := filepath.Join(nestedDir, "moved.jsonl")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("move session: %v", err)
	}

	result, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("move reconciliation: %v", err)
	}
	if len(result.Errors) != 0 || result.MessagesImported != 0 {
		t.Fatalf("move result = %+v", result)
	}
	after := messageRowIDs(t, database)
	for entryID, rowID := range before {
		if after[entryID] != rowID {
			t.Fatalf("entry %s row id changed across move", entryID)
		}
	}
	absNewPath, err := filepath.Abs(newPath)
	if err != nil {
		t.Fatalf("absolute moved path: %v", err)
	}
	var storedPath string
	if err := database.QueryRow(`SELECT session_file FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&storedPath); err != nil {
		t.Fatalf("query moved session: %v", err)
	}
	if storedPath != absNewPath {
		t.Fatalf("session path = %q, want %q", storedPath, absNewPath)
	}
	if got := countRowsWhere(t, database, "pi_sync_events", "event_type = 'file_moved'"); got != 1 {
		t.Fatalf("file_moved events = %d, want 1", got)
	}
	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil || !report.Match {
		t.Fatalf("lossless after move: report=%+v err=%v", report, err)
	}
}

func TestSyncPurgesConfirmedMissingSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	sessionPath := filepath.Join(sessionsDir, "session.jsonl")
	writeFixtureLines(t, sessionPath, incrementalFixtureLines[:3])
	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if err := os.Remove(sessionPath); err != nil {
		t.Fatalf("remove session: %v", err)
	}

	result, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("deletion reconciliation: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("deletion result = %+v", result)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	for _, table := range []string{"pi_messages", "pi_tool_calls", "pi_file_refs", "fts_pi_messages"} {
		if got := countRows(t, database, table); got != 0 {
			t.Fatalf("%s rows after deletion = %d", table, got)
		}
	}
	var deletedAt sql.NullString
	var status string
	var messages, toolCalls, fileRefs int
	var name, provider, model, firstUser, lastUser sql.NullString
	err = database.QueryRow(`
		SELECT file_deleted_at, sync_status, message_count, tool_call_count, file_ref_count,
			name, provider, model, first_user_text, last_user_text
		FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(
		&deletedAt, &status, &messages, &toolCalls, &fileRefs,
		&name, &provider, &model, &firstUser, &lastUser,
	)
	if err != nil {
		t.Fatalf("query deleted stub: %v", err)
	}
	if !deletedAt.Valid || status != "pending_delete" || messages != 0 || toolCalls != 0 || fileRefs != 0 {
		t.Fatalf("deleted stub state = deleted:%v status:%q counts:(%d,%d,%d)", deletedAt.Valid, status, messages, toolCalls, fileRefs)
	}
	if name.Valid || provider.Valid || model.Valid || firstUser.Valid || lastUser.Valid {
		t.Fatal("deleted stub retained preview content")
	}
	if got := countRowsWhere(t, database, "pi_sync_events", "event_type = 'file_deleted'"); got != 1 {
		t.Fatalf("file_deleted events = %d, want 1", got)
	}

	restoreDir := filepath.Join(sessionsDir, "restored")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("create restore dir: %v", err)
	}
	restoredPath := filepath.Join(restoreDir, "restored.jsonl")
	writeFixtureLines(t, restoredPath, incrementalFixtureLines[:3])
	restored, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("restore reconciliation: %v", err)
	}
	if len(restored.Errors) != 0 || restored.MessagesImported != 2 {
		t.Fatalf("restore result = %+v", restored)
	}
	if got := countRows(t, database, "pi_messages"); got != 2 {
		t.Fatalf("messages after restore = %d, want 2", got)
	}
	if got := countRowsWhere(t, database, "pi_sync_events", "event_type = 'file_restored'"); got != 1 {
		t.Fatalf("file_restored events = %d, want 1", got)
	}
	if err := database.QueryRow(`SELECT file_deleted_at, sync_status FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&deletedAt, &status); err != nil {
		t.Fatalf("query restored chat: %v", err)
	}
	if deletedAt.Valid || status != "idle" {
		t.Fatalf("restored chat state = deleted:%v status:%q", deletedAt.Valid, status)
	}
	var restoredSessionFile string
	if err := database.QueryRow(`SELECT session_file FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&restoredSessionFile); err != nil {
		t.Fatalf("query restored session path: %v", err)
	}
	absRestoredPath, err := filepath.Abs(restoredPath)
	if err != nil {
		t.Fatalf("absolute restored path: %v", err)
	}
	if restoredSessionFile != absRestoredPath {
		t.Fatalf("restored session path = %q, want %q", restoredSessionFile, absRestoredPath)
	}
	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil || !report.Match {
		t.Fatalf("lossless after restore: report=%+v err=%v", report, err)
	}
}

func TestSyncResolvesParentAfterParentMove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	parentName := "2026-06-18T20-00-00-000Z_" + parentFixtureUUID + ".jsonl"
	parentPath := filepath.Join(sessionsDir, parentName)
	childPath := filepath.Join(sessionsDir, "2026-06-18T20-01-00-000Z_"+childFixtureUUID+".jsonl")
	writeFixtureLines(t, parentPath, []string{
		fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2026-06-18T20:00:00.000Z","cwd":"/tmp/pi-parent"}`, parentFixtureUUID),
	})
	writeFixtureLines(t, childPath, []string{
		fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2026-06-18T20:01:00.000Z","cwd":"/tmp/pi-parent","parentSession":%q}`, childFixtureUUID, parentPath),
	})
	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	var parentID int64
	if err := database.QueryRow(`SELECT id FROM pi_chats WHERE uuid = ?`, parentFixtureUUID).Scan(&parentID); err != nil {
		t.Fatalf("query parent id: %v", err)
	}
	assertParentChatID(t, database, childFixtureUUID, parentID)

	movedDir := filepath.Join(sessionsDir, "nested")
	if err := os.MkdirAll(movedDir, 0700); err != nil {
		t.Fatalf("create moved dir: %v", err)
	}
	movedParentPath := filepath.Join(movedDir, parentName)
	if err := os.Rename(parentPath, movedParentPath); err != nil {
		t.Fatalf("move parent: %v", err)
	}
	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("move reconciliation: %v", err)
	}
	assertParentChatID(t, database, childFixtureUUID, parentID)
}

func assertParentChatID(t *testing.T, database *sql.DB, childUUID string, want int64) {
	t.Helper()
	var got sql.NullInt64
	if err := database.QueryRow(`SELECT parent_chat_id FROM pi_chats WHERE uuid = ?`, childUUID).Scan(&got); err != nil {
		t.Fatalf("query parent_chat_id: %v", err)
	}
	if !got.Valid || got.Int64 != want {
		t.Fatalf("parent_chat_id = %+v, want %d", got, want)
	}
}
