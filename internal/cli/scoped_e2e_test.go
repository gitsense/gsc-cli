/**
 * Component: Scoped Knowledge E2E Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: End-to-end smoke tests verifying consistent scoped behavior across rules, notes, and lessons.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestScopedKnowledgeE2E verifies the full scoped knowledge experience.
func TestScopedKnowledgeE2E(t *testing.T) {
	// Skip in short mode - E2E tests require gsc binary
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
	gscBin := filepath.Join(t.TempDir(), "gsc")
	buildCmd := exec.Command("go", "build", "-o", gscBin, "./cmd/gsc")
	buildCmd.Dir = projectRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build test gsc binary: %v\n%s", err, out)
	}

	// Set up temp repo and personal dir
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()

	// Set GSC_HOME for personal scope
	origGSCHome := os.Getenv("GSC_HOME")
	os.Setenv("GSC_HOME", personalDir)
	defer os.Setenv("GSC_HOME", origGSCHome)

	// Change to repo dir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Helper to run gsc commands
	runGSC := func(args ...string) (string, error) {
		cmd := exec.Command(gscBin, args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(), "GSC_HOME="+personalDir)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	writeScopedRulesFixtures(t, repoDir, personalDir)
	writeScopedNotesFixtures(t, repoDir, personalDir)

	// --- Test write commands require --target ---

	t.Run("rules new requires target", func(t *testing.T) {
		out, err := runGSC("rules", "new", "--summary", "test", "--topic", "test", "--instruction", "test", "--action", "edit")
		if err == nil {
			t.Errorf("rules new without --target should fail, got: %s", out)
		}
		if !strings.Contains(out, "required") {
			t.Errorf("error should mention required, got: %s", out)
		}
	})

	t.Run("notes add requires target", func(t *testing.T) {
		// Notes requires workspace init, so we test with a workspace
		initOut, _ := runGSC("manifest", "init")
		t.Logf("manifest init: %s", initOut)
		out, err := runGSC("notes", "add", "--summary", "test", "--topic", "test")
		if err == nil {
			t.Errorf("notes add without --target should fail, got: %s", out)
		}
		if !strings.Contains(out, "required") {
			t.Errorf("error should mention required, got: %s", out)
		}
	})

	t.Run("rules delete requires target", func(t *testing.T) {
		out, err := runGSC("rules", "delete", "fake_id")
		if err == nil {
			t.Errorf("rules delete without --target should fail, got: %s", out)
		}
		if !strings.Contains(out, "required") {
			t.Errorf("error should mention required, got: %s", out)
		}
	})

	// --- Test read commands accept --scope ---

	t.Run("rules get accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "get", "--file", "test.go", "--scope", "repo")
		if err != nil {
			t.Fatalf("rules get --scope repo failed: %s", out)
		}
		if !strings.Contains(out, "Scope: repo") {
			t.Errorf("output should mention scope, got: %s", out)
		}
	})

	t.Run("rules list accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "list", "--scope", "all", "-o", "json")
		if err != nil {
			t.Fatalf("rules list --scope all failed: %s", out)
		}
		if !strings.Contains(out, `"source": "repo"`) || !strings.Contains(out, `"source": "personal"`) {
			t.Errorf("json output should include repo and personal sources, got: %s", out)
		}
	})

	t.Run("rules show accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "show", "rule_personal_scoped_e2e", "--scope", "personal", "-o", "json")
		if err != nil {
			t.Fatalf("rules show --scope personal failed: %s", out)
		}
		if !strings.Contains(out, `"source": "personal"`) {
			t.Errorf("json output should include personal source, got: %s", out)
		}
	})

	t.Run("rules search accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "search", "Personal scoped e2e", "--scope", "personal", "-o", "json")
		if err != nil {
			t.Fatalf("rules search --scope personal failed: %s", out)
		}
		if !strings.Contains(out, `"source": "personal"`) || strings.Contains(out, `"source": "repo"`) {
			t.Errorf("json output should include only personal source, got: %s", out)
		}
	})

	t.Run("rules tags accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "tags", "--scope", "all", "-o", "json")
		if err != nil {
			t.Fatalf("rules tags --scope all failed: %s", out)
		}
		if !strings.Contains(out, `"tag": "phase9a"`) {
			t.Errorf("json output should include shared tag, got: %s", out)
		}
	})

	t.Run("rules overview accepts scope", func(t *testing.T) {
		out, err := runGSC("rules", "overview", "--scope", "all")
		if err != nil {
			t.Fatalf("rules overview --scope all failed: %s", out)
		}
		if !strings.Contains(out, "Scope: all (repo + personal)") {
			t.Errorf("overview should mention all scope, got: %s", out)
		}
	})

	t.Run("notes search accepts scope", func(t *testing.T) {
		// Notes requires workspace init
		runGSC("manifest", "init")
		out, err := runGSC("notes", "search", "test", "--scope", "personal")
		if err != nil {
			t.Fatalf("notes search --scope personal failed: %s", out)
		}
	})

	t.Run("notes tags accepts scope", func(t *testing.T) {
		out, err := runGSC("notes", "tags", "--scope", "all", "-o", "json")
		if err != nil {
			t.Fatalf("notes tags --scope all failed: %s", out)
		}
		if !strings.Contains(out, `"tag": "phase9b"`) {
			t.Errorf("json output should include shared note tag, got: %s", out)
		}
	})

	t.Run("notes overview accepts scope", func(t *testing.T) {
		out, err := runGSC("notes", "overview", "--scope", "all")
		if err != nil {
			t.Fatalf("notes overview --scope all failed: %s", out)
		}
		if !strings.Contains(out, "Scope: all (repo + personal)") {
			t.Errorf("overview should mention all scope, got: %s", out)
		}
	})

	t.Run("lessons list accepts scope", func(t *testing.T) {
		out, err := runGSC("lessons", "list", "--scope", "all")
		if err != nil {
			t.Fatalf("lessons list --scope all failed: %s", out)
		}
	})

	// --- Test help text ---

	t.Run("lessons add help shows draft commit flow", func(t *testing.T) {
		out, err := runGSC("lessons", "add", "--help")
		if err != nil {
			t.Fatalf("lessons add --help failed: %s", out)
		}
		if !strings.Contains(out, "draft commit") {
			t.Errorf("help should mention draft commit, got: %s", out)
		}
		if !strings.Contains(out, "--target") {
			t.Errorf("help should mention --target, got: %s", out)
		}
	})

	t.Run("rules get help shows scope", func(t *testing.T) {
		out, err := runGSC("rules", "get", "--help")
		if err != nil {
			t.Fatalf("rules get --help failed: %s", out)
		}
		if !strings.Contains(out, "--scope") {
			t.Errorf("help should mention --scope, got: %s", out)
		}
	})

	t.Run("read command help shows scope", func(t *testing.T) {
		commands := [][]string{
			{"rules", "get"},
			{"rules", "list"},
			{"rules", "show"},
			{"rules", "search"},
			{"rules", "tags"},
			{"rules", "overview"},
			{"notes", "get"},
			{"notes", "list"},
			{"notes", "show"},
			{"notes", "search"},
			{"notes", "tags"},
			{"notes", "overview"},
			{"lessons", "list"},
			{"lessons", "show"},
			{"lessons", "search"},
			{"lessons", "tags"},
			{"lessons", "overview"},
		}
		for _, parts := range commands {
			args := append(append([]string{}, parts...), "--help")
			out, err := runGSC(args...)
			if err != nil {
				t.Fatalf("%s --help failed: %s", strings.Join(parts, " "), out)
			}
			if !strings.Contains(out, "--scope") {
				t.Errorf("%s --help should mention --scope, got: %s", strings.Join(parts, " "), out)
			}
		}
	})

	// --- Test guide loads ---

	t.Run("rule-authoring guide loads", func(t *testing.T) {
		out, err := runGSC("experts", "guide", "rule-authoring")
		if err != nil {
			t.Fatalf("experts guide rule-authoring failed: %s", out)
		}
		if !strings.Contains(out, "--target") {
			t.Errorf("guide should mention --target, got: %s", out)
		}
		if !strings.Contains(out, "--scope") {
			t.Errorf("guide should mention --scope, got: %s", out)
		}
	})
}

func writeScopedNotesFixtures(t *testing.T, repoDir, personalDir string) {
	t.Helper()
	repoNotesDir := filepath.Join(repoDir, ".gitsense", "notes")
	personalNotesDir := filepath.Join(personalDir, "notes")
	if err := os.MkdirAll(repoNotesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(personalNotesDir, 0755); err != nil {
		t.Fatal(err)
	}

	repoNote := `{"id":"note_repo_scoped_e2e","schema_version":"1.0.0","created_at":"2026-06-28T00:00:00Z","updated_at":"2026-06-28T00:00:00Z","summary":"Repo scoped e2e note","content":"Repo scoped note content.","topic":"phase9b","tags":["phase9b","repo"],"importance":"medium"}`
	personalNote := `{"id":"note_personal_scoped_e2e","schema_version":"1.0.0","created_at":"2026-06-28T00:00:00Z","updated_at":"2026-06-28T00:00:00Z","summary":"Personal scoped e2e note","content":"Personal scoped note content.","topic":"phase9b","tags":["phase9b","personal"],"importance":"medium"}`

	if err := os.WriteFile(filepath.Join(repoNotesDir, "records.jsonl"), []byte(repoNote+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(personalNotesDir, "records.jsonl"), []byte(personalNote+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeScopedRulesFixtures(t *testing.T, repoDir, personalDir string) {
	t.Helper()
	repoRulesDir := filepath.Join(repoDir, ".gitsense", "rules")
	personalRulesDir := filepath.Join(personalDir, "rules")
	if err := os.MkdirAll(repoRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(personalRulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	repoRule := `{"id":"rule_repo_scoped_e2e","schema_version":"2.0.0","created_at":"2026-06-28T00:00:00Z","updated_at":"2026-06-28T00:00:00Z","summary":"Repo scoped e2e rule","topic":"phase9a","event":"pre_tool_use","instructions":["Repo scoped e2e instruction."],"actions":["edit"],"tags":["phase9a","repo"],"importance":"medium","type":"declarative"}`
	personalRule := `{"id":"rule_personal_scoped_e2e","schema_version":"2.0.0","created_at":"2026-06-28T00:00:00Z","updated_at":"2026-06-28T00:00:00Z","summary":"Personal scoped e2e rule","topic":"phase9a","event":"pre_tool_use","instructions":["Personal scoped e2e instruction."],"actions":["edit"],"tags":["phase9a","personal"],"importance":"medium","type":"declarative"}`

	if err := os.WriteFile(filepath.Join(repoRulesDir, "records.jsonl"), []byte(repoRule+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(personalRulesDir, "records.jsonl"), []byte(personalRule+"\n"), 0644); err != nil {
		t.Fatal(err)
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

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
