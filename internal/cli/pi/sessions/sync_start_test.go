/**
 * Component: Pi Sessions Sync Start CLI Tests
 * Block-UUID: 46bd5a67-b05b-43d9-9339-1b98c32b388e
 * Parent-UUID: 2e36c783-d48f-407e-b5ae-e7ff9f674fa2
 * Version: 1.0.0
 * Description: Verifies foreground watcher wiring, inherited flags, default paths, cancellation, and detached-mode rejection.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
)

func TestSyncStartRunsForegroundWatcherUntilCancellation(t *testing.T) {
	sessionsDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.sqlite3")
	called := make(chan pisessions.WatchOptions, 1)
	var signalCancel context.CancelFunc
	dependencies := syncStartDependencies{
		watch: func(ctx context.Context, options pisessions.WatchOptions) error {
			called <- options
			<-ctx.Done()
			return nil
		},
		notifyContext: func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			signalCancel = cancel
			return ctx, cancel
		},
	}
	cmd := syncCmdWithDependencies(dependencies)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"start", "--sessions-dir", sessionsDir, "--db", dbPath})
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(context.Background()) }()

	select {
	case options := <-called:
		absSessionsDir, _ := filepath.Abs(sessionsDir)
		absDBPath, _ := filepath.Abs(dbPath)
		if options.SessionsDir != absSessionsDir || options.DBPath != absDBPath {
			t.Fatalf("watch options = %+v, want sessions=%q db=%q", options, absSessionsDir, absDBPath)
		}
		signalCancel()
	case <-time.After(time.Second):
		t.Fatal("sync start did not invoke watcher")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("sync start: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("sync start did not stop after cancellation")
	}
	if !strings.Contains(output.String(), "Watching Pi sessions in") {
		t.Fatalf("start output = %q", output.String())
	}
}

func TestResolvePiSessionsDirDefaultsToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := resolvePiSessionsDir("")
	if err != nil {
		t.Fatalf("resolve default sessions dir: %v", err)
	}
	want := filepath.Join(home, ".pi", "agent", "sessions")
	if got != want {
		t.Fatalf("default sessions dir = %q, want %q", got, want)
	}
}

func TestSyncStartRejectsDetachedModeUntilImplemented(t *testing.T) {
	cmd := syncCmdWithDependencies(defaultSyncStartDependencies())
	cmd.SetArgs([]string{"start", "--detach"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "detached sync is not implemented") {
		t.Fatalf("detach error = %v", err)
	}
}
