/**
 * Component: Pi Sessions Watcher Tests
 * Block-UUID: 66b63521-1038-46b0-8089-8e6481ca7563
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Verifies watcher event coalescing, periodic reconciliation, nested discovery, and graceful cancellation.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type manualWatchEventSource struct {
	events chan string
	errors chan error
}

func newManualWatchEventSource() *manualWatchEventSource {
	return &manualWatchEventSource{
		events: make(chan string, 32),
		errors: make(chan error, 1),
	}
}

func (source *manualWatchEventSource) Events() <-chan string { return source.events }
func (source *manualWatchEventSource) Errors() <-chan error  { return source.errors }
func (source *manualWatchEventSource) Close() error          { return nil }

func TestWatchLoopCoalescesEventsAndReconcilesPeriodically(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := newManualWatchEventSource()
	var reconciles atomic.Int32
	reconcile := func(context.Context) error {
		reconciles.Add(1)
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, 25*time.Millisecond, 150*time.Millisecond, source, reconcile, nil)
	}()

	waitForCondition(t, time.Second, func() bool { return reconciles.Load() == 1 })
	for i := 0; i < 10; i++ {
		source.events <- fmt.Sprintf("session-%d.jsonl", i)
	}
	waitForCondition(t, time.Second, func() bool { return reconciles.Load() == 2 })
	time.Sleep(60 * time.Millisecond)
	if got := reconciles.Load(); got != 2 {
		t.Fatalf("burst produced %d reconciliations, want 2 including startup", got)
	}
	waitForCondition(t, time.Second, func() bool { return reconciles.Load() >= 3 })

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch loop cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("watch loop did not stop after cancellation")
	}
}

func TestWatchPollingDiscoversNestedSessionAndStops(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	rootSession := filepath.Join(sessionsDir, "root.jsonl")
	writeFixtureLines(t, rootSession, incrementalFixtureLines[:3])

	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, WatchOptions{
			SessionsDir:       sessionsDir,
			DBPath:            dbPath,
			DebounceInterval:  20 * time.Millisecond,
			ReconcileInterval: 5 * time.Second,
			PollInterval:      15 * time.Millisecond,
		})
	}()
	waitForDatabaseCount(t, dbPath, "pi_messages", 2)

	nestedDir := filepath.Join(sessionsDir, "parent-session", "run-1")
	if err := os.MkdirAll(nestedDir, 0700); err != nil {
		t.Fatalf("create nested session directory: %v", err)
	}
	nestedUUID := "019edff0-0000-7000-8000-000000000020"
	nestedSession := filepath.Join(nestedDir, "nested.jsonl")
	writeFixtureLines(t, nestedSession, []string{
		fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2026-06-18T21:00:00.000Z","cwd":"/tmp/pi-nested"}`, nestedUUID),
		`{"type":"message","id":"10000001","parentId":null,"timestamp":"2026-06-18T21:00:01.000Z","message":{"role":"user","content":[{"type":"text","text":"nested"}]}}`,
	})
	waitForDatabaseCount(t, dbPath, "pi_chats", 2)
	waitForDatabaseCount(t, dbPath, "pi_messages", 3)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not stop after cancellation")
	}
}

func waitForDatabaseCount(t *testing.T, dbPath string, table string, want int) {
	t.Helper()
	waitForCondition(t, 3*time.Second, func() bool {
		database, err := OpenQueryMirror(dbPath)
		if err != nil {
			return false
		}
		defer database.Close()
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return false
		}
		return count == want
	})
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
