/**
 * Component: Pi Sessions Incremental Sync Tests
 * Block-UUID: 28e42ad2-501c-49c3-8860-b0586afed2fb
 * Parent-UUID: 96bb35de-5c03-4ced-9ec7-18df3db71aec
 * Version: 1.0.0
 * Description: Verifies append-only Pi session ingestion, idempotency, partial-line handling, and lossless reconstruction.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const incrementalFixtureUUID = "019edff0-0000-7000-8000-000000000001"

var incrementalFixtureLines = []string{
	`{"type":"session","version":3,"id":"019edff0-0000-7000-8000-000000000001","timestamp":"2026-06-18T20:00:00.000Z","cwd":"/tmp/pi-incremental"}`,
	`{"type":"message","id":"00000001","parentId":null,"timestamp":"2026-06-18T20:00:01.000Z","message":{"role":"user","content":[{"type":"text","text":"read hello.txt"}]}}`,
	`{"type":"message","id":"00000002","parentId":"00000001","timestamp":"2026-06-18T20:00:02.000Z","message":{"role":"assistant","provider":"openai","model":"test-model","content":[{"type":"toolCall","id":"call_1","name":"read","arguments":{"path":"hello.txt"}}]}}`,
	`{"type":"message","id":"00000003","parentId":"00000002","timestamp":"2026-06-18T20:00:03.000Z","message":{"role":"toolResult","toolCallId":"call_1","toolName":"read","isError":false,"content":[{"type":"text","text":"hello"}]}}`,
	`{"type":"message","id":"00000004","parentId":"00000003","timestamp":"2026-06-18T20:00:04.000Z","message":{"role":"assistant","provider":"openai","model":"test-model","content":[{"type":"text","text":"done"}]}}`,
}

var aggregateFixtureLines = []string{
	`{"type":"compaction","id":"00000005","parentId":"00000003","timestamp":"2026-06-18T20:00:05.000Z","summary":"placeholder","details":{"readFiles":[],"modifiedFiles":[]}}`,
	`{"type":"branch_summary","id":"00000006","parentId":"00000005","timestamp":"2026-06-18T20:00:06.000Z","fromId":"00000005","summary":"branch","details":{"readFiles":[],"modifiedFiles":["branch.txt"]}}`,
}

func TestSyncAppendsWithoutReplacingExistingRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	sessionPath := filepath.Join(sessionsDir, "session.jsonl")
	writeFixtureLines(t, sessionPath, incrementalFixtureLines[:3])

	first, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if len(first.Errors) != 0 || first.MessagesImported != 2 {
		t.Fatalf("initial result = %+v", first)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	before := messageRowIDs(t, database)
	if _, err := database.Exec(`UPDATE pi_chats SET name = 'fixture-name' WHERE uuid = ?`, incrementalFixtureUUID); err != nil {
		t.Fatalf("set fixture name: %v", err)
	}

	appendFixtureLine(t, sessionPath, incrementalFixtureLines[3])
	second, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("append sync: %v", err)
	}
	if len(second.Errors) != 0 || second.MessagesImported != 1 {
		t.Fatalf("append result = %+v", second)
	}

	after := messageRowIDs(t, database)
	for entryID, rowID := range before {
		if after[entryID] != rowID {
			t.Fatalf("entry %s row id changed from %d to %d", entryID, rowID, after[entryID])
		}
	}

	var resultEntryID string
	var isError int
	if err := database.QueryRow(`SELECT result_entry_id, is_error FROM pi_tool_calls WHERE tool_call_id = 'call_1'`).Scan(&resultEntryID, &isError); err != nil {
		t.Fatalf("query updated tool result: %v", err)
	}
	if resultEntryID != "00000003" || isError != 0 {
		t.Fatalf("tool result = (%q, %d)", resultEntryID, isError)
	}
	var sessionName string
	if err := database.QueryRow(`SELECT name FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&sessionName); err != nil {
		t.Fatalf("query preserved session name: %v", err)
	}
	if sessionName != "fixture-name" {
		t.Fatalf("session name after append = %q", sessionName)
	}
	assertSyncBookmark(t, database, sessionPath, 2)
	assertChatCounts(t, database, 3, 1, 1)

	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil {
		t.Fatalf("verify lossless: %v", err)
	}
	if !report.Match {
		t.Fatalf("incremental reconstruction mismatch: %+v", report)
	}

	third, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("idempotent sync: %v", err)
	}
	if len(third.Errors) != 0 || third.MessagesImported != 0 || third.SessionsImported != 0 {
		t.Fatalf("idempotent result = %+v", third)
	}
}

func TestSyncDefersPartialTrailingLine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	sessionPath := filepath.Join(sessionsDir, "session.jsonl")
	writeFixtureLines(t, sessionPath, incrementalFixtureLines[:4])

	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	line := incrementalFixtureLines[4]
	split := len(line) / 2
	appendFixtureBytes(t, sessionPath, []byte(line[:split]))
	partial, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("partial sync: %v", err)
	}
	if len(partial.Errors) != 0 || partial.MessagesImported != 0 {
		t.Fatalf("partial result = %+v", partial)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	if got := countRows(t, database, "pi_messages"); got != 3 {
		t.Fatalf("messages after partial line = %d, want 3", got)
	}
	var syncedOffset int64
	var status string
	if err := database.QueryRow(`SELECT synced_byte_offset, sync_status FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&syncedOffset, &status); err != nil {
		t.Fatalf("query partial bookmark: %v", err)
	}
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("stat partial fixture: %v", err)
	}
	if syncedOffset >= info.Size() || status != "pending_partial" {
		t.Fatalf("partial bookmark = (%d, %q), file size %d", syncedOffset, status, info.Size())
	}

	appendFixtureBytes(t, sessionPath, []byte(line[split:]+"\n"))
	complete, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("completed-line sync: %v", err)
	}
	if len(complete.Errors) != 0 || complete.MessagesImported != 1 {
		t.Fatalf("completed result = %+v", complete)
	}
	if got := countRows(t, database, "pi_messages"); got != 4 {
		t.Fatalf("messages after completed line = %d, want 4", got)
	}

	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil || !report.Match {
		t.Fatalf("lossless after completing line: report=%+v err=%v", report, err)
	}
	assertSyncBookmark(t, database, sessionPath, 3)
}

func TestSyncRebuildsWhenPrefixChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	sessionPath := filepath.Join(sessionsDir, "session.jsonl")
	writeFixtureLines(t, sessionPath, incrementalFixtureLines[:3])

	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	before := messageRowIDs(t, database)

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rewritten := strings.ReplaceAll(string(data), "hello.txt", "other.txt")
	if len(rewritten) != len(data) {
		t.Fatal("test rewrite must preserve file size")
	}
	if err := os.WriteFile(sessionPath, []byte(rewritten), 0600); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(sessionPath, future, future); err != nil {
		t.Fatalf("advance fixture mtime: %v", err)
	}

	result, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("rewrite sync: %v", err)
	}
	if len(result.Errors) != 0 || result.SessionsImported != 1 || result.MessagesImported != 2 {
		t.Fatalf("rewrite result = %+v", result)
	}
	after := messageRowIDs(t, database)
	if after["00000001"] == before["00000001"] {
		t.Fatal("prefix rewrite did not rebuild existing message rows")
	}
	var rawPath string
	if err := database.QueryRow(`SELECT raw_path FROM pi_file_refs LIMIT 1`).Scan(&rawPath); err != nil {
		t.Fatalf("query rebuilt file ref: %v", err)
	}
	if rawPath != "other.txt" {
		t.Fatalf("rebuilt raw path = %q, want other.txt", rawPath)
	}
	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil || !report.Match {
		t.Fatalf("lossless after rebuild: report=%+v err=%v", report, err)
	}
}

func TestSyncAppendsCompactionAndBranchSummary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	sessionPath := filepath.Join(sessionsDir, "session.jsonl")
	writeFixtureLines(t, sessionPath, incrementalFixtureLines[:4])

	if _, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	for _, line := range aggregateFixtureLines {
		appendFixtureLine(t, sessionPath, line)
	}
	result, err := Sync(ctx, SyncOptions{SessionsDir: sessionsDir, DBPath: dbPath})
	if err != nil {
		t.Fatalf("aggregate append sync: %v", err)
	}
	if len(result.Errors) != 0 || result.MessagesImported != 2 || result.FileRefsImported != 1 {
		t.Fatalf("aggregate append result = %+v", result)
	}

	database := openIncrementalTestDB(t, dbPath)
	defer database.Close()
	var source, op, rawPath string
	if err := database.QueryRow(`SELECT source, op, raw_path FROM pi_file_refs WHERE source = 'branch_summary'`).Scan(&source, &op, &rawPath); err != nil {
		t.Fatalf("query branch summary ref: %v", err)
	}
	if source != "branch_summary" || op != "branch_summary_modified" || rawPath != "branch.txt" {
		t.Fatalf("branch summary ref = (%q, %q, %q)", source, op, rawPath)
	}
	if got := countRowsWhere(t, database, "pi_file_refs", "source = 'compaction'"); got != 0 {
		t.Fatalf("empty compaction emitted %d file refs", got)
	}
	var currentLeaf string
	if err := database.QueryRow(`SELECT current_leaf_id FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&currentLeaf); err != nil {
		t.Fatalf("query current leaf: %v", err)
	}
	if currentLeaf != "00000006" {
		t.Fatalf("current leaf = %q, want 00000006", currentLeaf)
	}
	report, err := VerifyLossless(ctx, database, incrementalFixtureUUID)
	if err != nil || !report.Match {
		t.Fatalf("lossless aggregate append: report=%+v err=%v", report, err)
	}
}

func writeFixtureLines(t *testing.T, path string, lines []string) {
	t.Helper()
	data := ""
	for _, line := range lines {
		data += line + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func appendFixtureLine(t *testing.T, path string, line string) {
	t.Helper()
	appendFixtureBytes(t, path, []byte(line+"\n"))
}

func appendFixtureBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("open fixture for append: %v", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		t.Fatalf("append fixture: %v", err)
	}
}

func openIncrementalTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := OpenQueryMirror(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return database
}

func messageRowIDs(t *testing.T, database *sql.DB) map[string]int64 {
	t.Helper()
	rows, err := database.Query(`SELECT entry_id, id FROM pi_messages ORDER BY seq`)
	if err != nil {
		t.Fatalf("query message ids: %v", err)
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var entryID string
		var rowID int64
		if err := rows.Scan(&entryID, &rowID); err != nil {
			t.Fatalf("scan message id: %v", err)
		}
		result[entryID] = rowID
	}
	return result
}

func countRows(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func countRowsWhere(t *testing.T, database *sql.DB, table string, where string) int {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&count); err != nil {
		t.Fatalf("count %s where %s: %v", table, where, err)
	}
	return count
}

func assertSyncBookmark(t *testing.T, database *sql.DB, sessionPath string, wantSeq int) {
	t.Helper()
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	var seq int
	var offset int64
	var status string
	if err := database.QueryRow(`SELECT synced_seq, synced_byte_offset, sync_status FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&seq, &offset, &status); err != nil {
		t.Fatalf("query sync bookmark: %v", err)
	}
	if seq != wantSeq || offset != info.Size() || status != "idle" {
		t.Fatalf("bookmark = (seq=%d offset=%d status=%q), want (%d, %d, idle)", seq, offset, status, wantSeq, info.Size())
	}
}

func assertChatCounts(t *testing.T, database *sql.DB, messages int, toolCalls int, fileRefs int) {
	t.Helper()
	var gotMessages, gotToolCalls, gotFileRefs int
	if err := database.QueryRow(`SELECT message_count, tool_call_count, file_ref_count FROM pi_chats WHERE uuid = ?`, incrementalFixtureUUID).Scan(&gotMessages, &gotToolCalls, &gotFileRefs); err != nil {
		t.Fatalf("query chat counts: %v", err)
	}
	if gotMessages != messages || gotToolCalls != toolCalls || gotFileRefs != fileRefs {
		t.Fatalf("chat counts = (%d, %d, %d), want (%d, %d, %d)", gotMessages, gotToolCalls, gotFileRefs, messages, toolCalls, fileRefs)
	}
}
