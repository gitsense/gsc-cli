/**
 * Component: Pi Sessions Verify Tests
 * Block-UUID: 0c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests lossless verification against controlled session fixtures.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package sessions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Test fixtures - real session UUIDs and their source directories
const (
	fixtureUUID1 = "019edb3e-984b-75b0-ab37-2e73b9087571" // create/edit/write
	fixtureUUID2 = "019edb67-64fb-7650-9abb-f9e3bba85cef" // read-only
)

// getFixturesSessionsDir returns the path to the fixture sessions directory.
// Uses environment variable or falls back to the known location.
func getFixturesSessionsDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("PI_TEST_SESSIONS_DIR")
	if dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}
	return filepath.Join(home, ".pi", "agent", "sessions")
}

// importFixtures creates a temp database and imports the fixture sessions.
func importFixtures(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-pi-sessions.sqlite3")
	sessionsDir := getFixturesSessionsDir(t)

	result, err := Sync(context.Background(), SyncOptions{
		SessionsDir: sessionsDir,
		DBPath:      dbPath,
	})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(result.Errors) > 0 {
		for _, syncErr := range result.Errors {
			t.Errorf("sync error on %s: %s", syncErr.Path, syncErr.Error)
		}
		t.Fatal("sync had errors")
	}
	if result.SessionsImported == 0 {
		t.Fatal("no sessions imported")
	}
	t.Logf("imported %d sessions, %d messages", result.SessionsImported, result.MessagesImported)
	return dbPath
}

func TestReconstructJSONL(t *testing.T) {
	dbPath := importFixtures(t)

	ctx := context.Background()
	database, err := OpenQueryMirror(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	tests := []struct {
		name    string
		uuid    string
		wantErr bool
	}{
		{"fixture 1 (create/edit/write)", fixtureUUID1, false},
		{"fixture 2 (read-only)", fixtureUUID2, false},
		{"nonexistent session", "nonexistent-uuid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ReconstructJSONL(ctx, database, tt.uuid)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(data) == 0 {
				t.Error("reconstructed bytes is empty")
			}
			// Verify it starts with a valid session header
			if len(data) > 0 && data[0] != '{' {
				t.Errorf("expected JSON start, got %q", data[0])
			}
		})
	}
}

func TestVerifyLossless(t *testing.T) {
	dbPath := importFixtures(t)

	ctx := context.Background()
	database, err := OpenQueryMirror(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	tests := []struct {
		name        string
		uuid        string
		wantMatch   bool
		wantMissing bool
		wantErr     bool
	}{
		{"fixture 1 lossless", fixtureUUID1, true, false, false},
		{"fixture 2 lossless", fixtureUUID2, true, false, false},
		{"nonexistent session", "nonexistent-uuid", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := VerifyLossless(ctx, database, tt.uuid)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Match != tt.wantMatch {
				t.Errorf("match = %v, want %v", report.Match, tt.wantMatch)
				t.Logf("  source bytes: %d, reconstructed bytes: %d", report.SourceBytes, report.ReconstructedBytes)
				t.Logf("  source sha256: %s", report.SourceSHA256)
				t.Logf("  reconstructed sha256: %s", report.ReconstructedSHA256)
			}
			if report.SourceMissing != tt.wantMissing {
				t.Errorf("source_missing = %v, want %v", report.SourceMissing, tt.wantMissing)
			}
			if report.Match {
				t.Logf("PASS: %s (%d bytes, sha256:%s...)", report.SessionFile, report.SourceBytes, report.SourceSHA256[:16])
			}
		})
	}
}

func TestVerifyLosslessWithDB(t *testing.T) {
	dbPath := importFixtures(t)
	ctx := context.Background()

	report, err := VerifyLosslessWithDB(ctx, dbPath, fixtureUUID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Match {
		t.Errorf("expected match=true, got false")
		t.Logf("  source: %d bytes, reconstructed: %d bytes", report.SourceBytes, report.ReconstructedBytes)
	}
}
