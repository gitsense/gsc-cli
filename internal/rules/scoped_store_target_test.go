/**
 * Component: Rules Targeted Store Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for target-based write operations (repo vs personal).
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

func TestAppendRecordToTargetPersonal(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	record := Rule{ID: "test_rule_1", Summary: "test rule"}

	err := AppendRecordToTarget(record, gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("AppendRecordToTarget(personal) error: %v", err)
	}

	// Verify record was written to personal
	records, err := LoadRecordsFromTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("LoadRecordsFromTarget(personal) error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].ID != "test_rule_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "test_rule_1")
	}
}

func TestAppendRecordToTargetRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	record := Rule{ID: "repo_rule_1", Summary: "repo rule"}

	err := AppendRecordToTarget(record, gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("AppendRecordToTarget(repo) error: %v", err)
	}

	// Verify record was written to repo
	records, err := LoadRecordsFromTarget(gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("LoadRecordsFromTarget(repo) error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].ID != "repo_rule_1" {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, "repo_rule_1")
	}
}

func TestDeleteRecordFromTargetIsolation(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// Write to personal
	if err := AppendRecordToTarget(Rule{ID: "personal_rule", Summary: "personal"}, gitsensescope.TargetPersonal); err != nil {
		t.Fatal(err)
	}

	// Try to delete from repo (should fail - not found)
	repoDir := initTempGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	deleted, err := DeleteRecordFromTarget("personal_rule", gitsensescope.TargetRepo)
	if err != nil {
		t.Fatalf("DeleteRecordFromTarget(repo) error: %v", err)
	}
	if deleted {
		t.Error("should not have deleted personal rule from repo store")
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

func TestWriteRecordsToTarget(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	records := []Rule{
		{ID: "rule_1", Summary: "rule 1"},
		{ID: "rule_2", Summary: "rule 2"},
	}

	err := WriteRecordsToTarget(records, gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("WriteRecordsToTarget(personal) error: %v", err)
	}

	// Verify records
	loaded, err := LoadRecordsFromTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("got %d records, want 2", len(loaded))
	}
}

func TestSourceFromTarget(t *testing.T) {
	tests := []struct {
		target gitsensescope.Target
		want   gitsensescope.Source
	}{
		{gitsensescope.TargetRepo, gitsensescope.SourceRepo},
		{gitsensescope.TargetPersonal, gitsensescope.SourcePersonal},
		{"", gitsensescope.SourceRepo}, // default
	}
	for _, tt := range tests {
		t.Run(string(tt.target), func(t *testing.T) {
			got := SourceFromTarget(tt.target)
			if got != tt.want {
				t.Errorf("SourceFromTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
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
	want := filepath.Join(personalDir, "rules", "records.jsonl")
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

	// TriggersDirForTarget
	path, err = TriggersDirForTarget(gitsensescope.TargetPersonal)
	if err != nil {
		t.Fatalf("TriggersDirForTarget(personal) error: %v", err)
	}
	want = filepath.Join(personalDir, "rules", "triggers")
	if path != want {
		t.Errorf("TriggersDirForTarget(personal) = %q, want %q", path, want)
	}
}
