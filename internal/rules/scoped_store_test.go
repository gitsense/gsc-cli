/**
 * Component: Rules Scoped Store Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for scoped record loading with source provenance.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

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
	rulesDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write test records
	records := []Rule{
		{ID: "rule_1", Summary: "test rule 1"},
		{ID: "rule_2", Summary: "test rule 2"},
	}
	writeTestRecords(t, filepath.Join(rulesDir, "records.jsonl"), records)

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
	if got[0].Rule.ID != "rule_1" {
		t.Errorf("got[0].Rule.ID = %q, want %q", got[0].Rule.ID, "rule_1")
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

func TestLoadRecordsFromScopePersonal(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	rulesDir := filepath.Join(personalDir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	records := []Rule{
		{ID: "personal_rule_1", Summary: "personal rule"},
	}
	writeTestRecords(t, filepath.Join(rulesDir, "records.jsonl"), records)

	got, err := LoadRecordsFromScope(gitsensescope.ScopePersonal)
	if err != nil {
		t.Fatalf("LoadRecordsFromScope(personal) error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].Source != gitsensescope.SourcePersonal {
		t.Errorf("got[0].Source = %q, want %q", got[0].Source, gitsensescope.SourcePersonal)
	}
	if got[0].Rule.ID != "personal_rule_1" {
		t.Errorf("got[0].Rule.ID = %q, want %q", got[0].Rule.ID, "personal_rule_1")
	}
}

func TestLoadRecordsFromScopeAllInRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Write repo records
	repoRulesDir := filepath.Join(repoDir, ".gitsense", "rules")
	if err := os.MkdirAll(repoRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestRecords(t, filepath.Join(repoRulesDir, "records.jsonl"), []Rule{
		{ID: "repo_rule_1", Summary: "repo rule"},
	})

	// Write personal records
	personalRulesDir := filepath.Join(personalDir, "rules")
	if err := os.MkdirAll(personalRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestRecords(t, filepath.Join(personalRulesDir, "records.jsonl"), []Rule{
		{ID: "personal_rule_1", Summary: "personal rule"},
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
	if got[0].Rule.ID != "repo_rule_1" {
		t.Errorf("got[0].Rule.ID = %q, want %q", got[0].Rule.ID, "repo_rule_1")
	}

	// Personal should come second
	if got[1].Source != gitsensescope.SourcePersonal {
		t.Errorf("got[1].Source = %q, want %q", got[1].Source, gitsensescope.SourcePersonal)
	}
	if got[1].Rule.ID != "personal_rule_1" {
		t.Errorf("got[1].Rule.ID = %q, want %q", got[1].Rule.ID, "personal_rule_1")
	}
}

func TestLoadRecordsFromScopeAllOutsideRepo(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(personalDir) // Not a git repo

	personalRulesDir := filepath.Join(personalDir, "rules")
	if err := os.MkdirAll(personalRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestRecords(t, filepath.Join(personalRulesDir, "records.jsonl"), []Rule{
		{ID: "personal_rule_1", Summary: "personal rule"},
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

func TestLoadRecordsFromScopeRepoOutsideRepo(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	_, err := LoadRecordsFromScope(gitsensescope.ScopeRepo)
	if err == nil {
		t.Fatal("LoadRecordsFromScope(repo) outside repo expected error, got nil")
	}
}

func TestLoadRecordsFromScopeForRepoOverride(t *testing.T) {
	repoDir := t.TempDir()
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// Write repo records to overridden path
	repoRulesDir := filepath.Join(repoDir, ".gitsense", "rules")
	if err := os.MkdirAll(repoRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestRecords(t, filepath.Join(repoRulesDir, "records.jsonl"), []Rule{
		{ID: "repo_rule_1", Summary: "repo rule"},
	})

	got, err := LoadRecordsFromScopeForRepo(gitsensescope.ScopeRepo, repoDir)
	if err != nil {
		t.Fatalf("LoadRecordsFromScopeForRepo(repo) error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].Source != gitsensescope.SourceRepo {
		t.Errorf("got[0].Source = %q, want %q", got[0].Source, gitsensescope.SourceRepo)
	}
}

func TestSourcedMatchingHelpers(t *testing.T) {
	sourcedRecords := []SourcedRule{
		{
			Source: gitsensescope.SourceRepo,
			Rule: Rule{
				ID:           "repo_rule",
				GlobPatterns: []string{"**/*.go"},
				Actions:      []string{"edit"},
			},
		},
		{
			Source: gitsensescope.SourcePersonal,
			Rule: Rule{
				ID:           "personal_rule",
				GlobPatterns: []string{"**/*.md"},
				Actions:      []string{"edit"},
			},
		},
	}

	// Test GetSourcedRulesForFile
	matched := GetSourcedRulesForFile(sourcedRecords, "test.go", "edit", "")
	if len(matched) != 1 {
		t.Fatalf("GetSourcedRulesForFile(.go) got %d, want 1", len(matched))
	}
	if matched[0].Source != gitsensescope.SourceRepo {
		t.Errorf("matched[0].Source = %q, want %q", matched[0].Source, gitsensescope.SourceRepo)
	}

	// Test GetSourcedRulesForGlob
	matched = GetSourcedRulesForGlob(sourcedRecords, "**/*.md", "")
	if len(matched) != 1 {
		t.Fatalf("GetSourcedRulesForGlob(.md) got %d, want 1", len(matched))
	}
	if matched[0].Source != gitsensescope.SourcePersonal {
		t.Errorf("matched[0].Source = %q, want %q", matched[0].Source, gitsensescope.SourcePersonal)
	}
}

func writeTestRecords(t *testing.T, path string, records []Rule) {
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
