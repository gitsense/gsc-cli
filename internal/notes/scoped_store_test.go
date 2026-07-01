/**
 * Component: Notes Scoped Store Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for scoped record loading and target-based writes for notes.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

func TestLoadRecordsFromSourcedDir(t *testing.T) {
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatal(err)
	}

	records := []Note{
		{ID: "note_1", Summary: "test note 1"},
		{ID: "note_2", Summary: "test note 2"},
	}
	writeTestNotes(t, filepath.Join(notesDir, "records.jsonl"), records)

	sourcedDir := gitsensescope.SourcedDir{
		Source: gitsensescope.SourceRepo,
		Path:   dir,
	}

	got, err := LoadRecordsFromSourcedDir(sourcedDir)
	if err != nil {
		t.Fatalf("LoadRecordsFromSourcedDir() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0].Source != gitsensescope.SourceRepo {
		t.Errorf("got[0].Source = %q, want %q", got[0].Source, gitsensescope.SourceRepo)
	}
}

func TestLoadRecordsFromSourcedDirMissing(t *testing.T) {
	dir := t.TempDir()
	sourcedDir := gitsensescope.SourcedDir{
		Source: gitsensescope.SourcePersonal,
		Path:   dir,
	}

	got, err := LoadRecordsFromSourcedDir(sourcedDir)
	if err != nil {
		t.Fatalf("LoadRecordsFromSourcedDir() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d records, want 0 for missing file", len(got))
	}
}

func TestLoadRecordsFromScopeAllInRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Write repo notes
	repoNotesDir := filepath.Join(repoDir, ".gitsense", "notes")
	if err := os.MkdirAll(repoNotesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestNotes(t, filepath.Join(repoNotesDir, "records.jsonl"), []Note{
		{ID: "repo_note_1", Summary: "repo note"},
	})

	// Write personal notes
	personalNotesDir := filepath.Join(personalDir, "notes")
	if err := os.MkdirAll(personalNotesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestNotes(t, filepath.Join(personalNotesDir, "records.jsonl"), []Note{
		{ID: "personal_note_1", Summary: "personal note"},
	})

	got, err := LoadRecordsFromScope(gitsensescope.ScopeAll)
	if err != nil {
		t.Fatalf("LoadRecordsFromScope(all) error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}

	// Repo should come first
	if got[0].Source != gitsensescope.SourceRepo {
		t.Errorf("got[0].Source = %q, want %q", got[0].Source, gitsensescope.SourceRepo)
	}
	if got[0].Note.ID != "repo_note_1" {
		t.Errorf("got[0].Note.ID = %q, want %q", got[0].Note.ID, "repo_note_1")
	}

	// Personal should come second
	if got[1].Source != gitsensescope.SourcePersonal {
		t.Errorf("got[1].Source = %q, want %q", got[1].Source, gitsensescope.SourcePersonal)
	}
	if got[1].Note.ID != "personal_note_1" {
		t.Errorf("got[1].Note.ID = %q, want %q", got[1].Note.ID, "personal_note_1")
	}
}

func TestLoadRecordsFromScopeAllOutsideRepo(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(personalDir)

	personalNotesDir := filepath.Join(personalDir, "notes")
	if err := os.MkdirAll(personalNotesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestNotes(t, filepath.Join(personalNotesDir, "records.jsonl"), []Note{
		{ID: "personal_note_1", Summary: "personal note"},
	})

	got, err := LoadRecordsFromScope(gitsensescope.ScopeAll)
	if err != nil {
		t.Fatalf("LoadRecordsFromScope(all) outside repo error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].Source != gitsensescope.SourcePersonal {
		t.Errorf("got[0].Source = %q, want %q", got[0].Source, gitsensescope.SourcePersonal)
	}
}

func TestAppendRecordToTargetPersonal(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	record := Note{ID: "test_note_1", Summary: "test note"}

	err := AppendRecordToTarget(record, gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("AppendRecordToTarget(personal) error: %v", err)
	}

	records, err := LoadRecordsFromTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("LoadRecordsFromTarget(personal) error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].ID != "test_note_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "test_note_1")
	}
}

func TestAppendRecordToTargetRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	record := Note{ID: "repo_note_1", Summary: "repo note"}

	err := AppendRecordToTarget(record, gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("AppendRecordToTarget(repo) error: %v", err)
	}

	records, err := LoadRecordsFromTarget(gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("LoadRecordsFromTarget(repo) error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].ID != "repo_note_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "repo_note_1")
	}
}

func TestDeleteRecordFromTargetIsolation(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// Write to personal
	if err := AppendRecordToTarget(Note{ID: "personal_note", Summary: "personal"}, gitsensescope.TargetPersonal); err != nil {
		t.Fatal(err)
	}

	// Try to delete from repo (should fail - not found)
	repoDir := initTempGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	deleted, err := DeleteRecordFromTarget("personal_note", gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("DeleteRecordFromTarget(repo) error: %v", err)
	}
	if deleted {
		t.Error("should not have deleted personal note from repo store")
	}

	// Verify personal record still exists
	records, err := LoadRecordsFromTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("personal records should still exist, got %d", len(records))
	}
}

func TestTargetPathHelpers(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// RecordsPathForTarget
	path, err := RecordsPathForTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("RecordsPathForTarget(personal) error: %v", err)
	}
	want := filepath.Join(personalDir, "notes", "records.jsonl")
	if path != want {
		t.Errorf("RecordsPathForTarget(personal) = %q, want %q", path, want)
	}

	// ManifestPathForTarget
	path, err = ManifestPathForTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("ManifestPathForTarget(personal) error: %v", err)
	}
	want = filepath.Join(personalDir, "manifests", DatabaseName+".json")
	if path != want {
		t.Errorf("ManifestPathForTarget(personal) = %q, want %q", path, want)
	}
}

func TestSourcedQueryHelpers(t *testing.T) {
	sourcedRecords := []SourcedNote{
		{
			Source: gitsensescope.SourceRepo,
			Note: Note{
				ID:           "repo_note",
				GlobPatterns: []string{"**/*.go"},
			},
		},
		{
			Source: gitsensescope.SourcePersonal,
			Note: Note{
				ID:           "personal_note",
				GlobPatterns: []string{"**/*.md"},
			},
		},
	}

	// Test GetSourcedNotesForFile
	matched := GetSourcedNotesForFile(sourcedRecords, "test.go")
	if len(matched) != 1 {
		t.Fatalf("GetSourcedNotesForFile(.go) got %d, want 1", len(matched))
	}
	if matched[0].Source != gitsensescope.SourceRepo {
		t.Errorf("matched[0].Source = %q, want %q", matched[0].Source, gitsensescope.SourceRepo)
	}

	// Test GetSourcedNotesForGlob
	matched = GetSourcedNotesForGlob(sourcedRecords, "**/*.md")
	if len(matched) != 1 {
		t.Fatalf("GetSourcedNotesForGlob(.md) got %d, want 1", len(matched))
	}
	if matched[0].Source != gitsensescope.SourcePersonal {
		t.Errorf("matched[0].Source = %q, want %q", matched[0].Source, gitsensescope.SourcePersonal)
	}
}

func TestResolveSourcedRecordFromRecordsAmbiguousAcrossSources(t *testing.T) {
	records := []SourcedNote{
		{
			Source: gitsensescope.SourceRepo,
			Note:   Note{ID: "note_same"},
		},
		{
			Source: gitsensescope.SourcePersonal,
			Note:   Note{ID: "note_same"},
		},
	}

	got, err := ResolveSourcedRecordFromRecords("note_same", records)
	if err == nil {
		t.Fatal("ResolveSourcedRecordFromRecords() error = nil, want ambiguity error")
	}
	if got != nil {
		t.Fatalf("ResolveSourcedRecordFromRecords() record = %#v, want nil", got)
	}
}

func TestResolveSourcedRecordFromRecordsPreservesSource(t *testing.T) {
	records := []SourcedNote{
		{
			Source: gitsensescope.SourceRepo,
			Note:   Note{ID: "note_repo"},
		},
		{
			Source: gitsensescope.SourcePersonal,
			Note:   Note{ID: "note_personal"},
		},
	}

	got, err := ResolveSourcedRecordFromRecords("personal", records)
	if err != nil {
		t.Fatalf("ResolveSourcedRecordFromRecords() error: %v", err)
	}
	if got == nil {
		t.Fatal("ResolveSourcedRecordFromRecords() record = nil, want personal record")
	}
	if got.Source != gitsensescope.SourcePersonal {
		t.Fatalf("got Source = %q, want %q", got.Source, gitsensescope.SourcePersonal)
	}
	if got.Note.ID != "note_personal" {
		t.Fatalf("got Note.ID = %q, want note_personal", got.Note.ID)
	}
}

func writeTestNotes(t *testing.T, path string, records []Note) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, r := range records {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		file.Write(append(data, '\n'))
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
	return dir
}
