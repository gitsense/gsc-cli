/**
 * Component: Lessons Scoped Store Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for scoped record loading and target-based writes for lessons.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package lessons

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
	lessonsDir := filepath.Join(dir, "lessons")
	if err := os.MkdirAll(lessonsDir, 0755); err != nil {
		t.Fatal(err)
	}

	records := []Record{
		{ID: "lsn_1", Summary: "test lesson 1"},
		{ID: "lsn_2", Summary: "test lesson 2"},
	}
	writeTestLessons(t, filepath.Join(lessonsDir, "records.jsonl"), records)

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

	// Write repo lessons
	repoLessonsDir := filepath.Join(repoDir, ".gitsense", "lessons")
	if err := os.MkdirAll(repoLessonsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestLessons(t, filepath.Join(repoLessonsDir, "records.jsonl"), []Record{
		{ID: "lsn_repo_1", Summary: "repo lesson"},
	})

	// Write personal lessons
	personalLessonsDir := filepath.Join(personalDir, "lessons")
	if err := os.MkdirAll(personalLessonsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestLessons(t, filepath.Join(personalLessonsDir, "records.jsonl"), []Record{
		{ID: "lsn_personal_1", Summary: "personal lesson"},
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
	if got[0].Lesson.ID != "lsn_repo_1" {
		t.Errorf("got[0].Lesson.ID = %q, want %q", got[0].Lesson.ID, "lsn_repo_1")
	}

	// Personal should come second
	if got[1].Source != gitsensescope.SourcePersonal {
		t.Errorf("got[1].Source = %q, want %q", got[1].Source, gitsensescope.SourcePersonal)
	}
	if got[1].Lesson.ID != "lsn_personal_1" {
		t.Errorf("got[1].Lesson.ID = %q, want %q", got[1].Lesson.ID, "lsn_personal_1")
	}
}

func TestLoadRecordsFromScopeAllOutsideRepo(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(personalDir)

	personalLessonsDir := filepath.Join(personalDir, "lessons")
	if err := os.MkdirAll(personalLessonsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestLessons(t, filepath.Join(personalLessonsDir, "records.jsonl"), []Record{
		{ID: "lsn_personal_1", Summary: "personal lesson"},
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

	record := Record{ID: "lsn_test_1", Summary: "test lesson"}

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
	if records[0].ID != "lsn_test_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "lsn_test_1")
	}
}

func TestAppendRecordToTargetRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	record := Record{ID: "lsn_repo_1", Summary: "repo lesson"}

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
	if records[0].ID != "lsn_repo_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "lsn_repo_1")
	}
}

func TestDeleteRecordFromTargetIsolation(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// Write to personal
	if err := AppendRecordToTarget(Record{ID: "lsn_personal", Summary: "personal"}, gitsensescope.TargetPersonal); err != nil {
		t.Fatal(err)
	}

	// Try to delete from repo (should fail - not found)
	repoDir := initTempGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	deleted, err := DeleteRecordFromTarget("lsn_personal", gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("DeleteRecordFromTarget(repo) error: %v", err)
	}
	if deleted {
		t.Error("should not have deleted personal lesson from repo store")
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
	want := filepath.Join(personalDir, "lessons", "records.jsonl")
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

func TestResolveSourcedRecordFromRecordsAmbiguity(t *testing.T) {
	records := []SourcedLesson{
		{Source: gitsensescope.SourceRepo, Lesson: Record{ID: "lsn_abc_repo", Summary: "repo"}},
		{Source: gitsensescope.SourcePersonal, Lesson: Record{ID: "lsn_abc_personal", Summary: "personal"}},
	}

	// Should error on cross-source ambiguity
	_, err := ResolveSourcedRecordFromRecords("lsn_abc", records)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !containsStr(err.Error(), "repo and personal") {
		t.Errorf("error should mention repo and personal, got: %v", err)
	}
}

func TestResolveSourcedRecordFromRecordsExactAmbiguity(t *testing.T) {
	records := []SourcedLesson{
		{Source: gitsensescope.SourceRepo, Lesson: Record{ID: "lsn_same", Summary: "repo"}},
		{Source: gitsensescope.SourcePersonal, Lesson: Record{ID: "lsn_same", Summary: "personal"}},
	}

	_, err := ResolveSourcedRecordFromRecords("lsn_same", records)
	if err == nil {
		t.Fatal("expected exact cross-source ambiguity error")
	}
	if !containsStr(err.Error(), "repo and personal") {
		t.Errorf("error should mention repo and personal, got: %v", err)
	}
}

func writeTestLessons(t *testing.T, path string, records []Record) {
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

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrHelper(s, substr))
}

func containsStrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
