/**
 * Component: Experts Init Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for gsc experts init stdout default, --out flag, and outside-repo behavior.
 * Language: Go
 * Created-at: 2026-06-28T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package experts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gscBinary returns the path to a compiled gsc binary for testing.
func gscBinary(t *testing.T) string {
	t.Helper()
	// Find project root by looking for go.mod
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find project root (go.mod)")
		}
		root = parent
	}

	bin := filepath.Join(t.TempDir(), "gsc")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gsc")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build gsc: %v\n%s", err, out)
	}
	return bin
}

func TestInitStdoutDefault(t *testing.T) {
	bin := gscBinary(t)
	repoDir := initTempGitRepo(t)

	// Run init and capture stdout
	cmd := exec.Command(bin, "experts", "init")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init failed: %s", out)
	}

	output := string(out)

	// Should contain the role section
	if !strings.Contains(output, "Domain Expert") {
		t.Errorf("stdout should contain 'Domain Expert', got: %s", output)
	}

	// Should mention stdout
	if !strings.Contains(output, "stdout") {
		t.Errorf("output should mention 'stdout', got: %s", output)
	}

	// Should NOT create experts-context.md in the repo
	contextPath := filepath.Join(repoDir, ".gitsense", "experts-context.md")
	if _, err := os.Stat(contextPath); err == nil {
		t.Error("init should not create experts-context.md by default")
	}
}

func TestInitDefaultsRulesModeToAdvisory(t *testing.T) {
	bin := gscBinary(t)
	repoDir := initTempGitRepo(t)
	rulesDir := filepath.Join(repoDir, ".gitsense", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	rule := `{"id":"rule_test","schema_version":"2.0.0","created_at":"2026-06-28T00:00:00Z","updated_at":"2026-06-28T00:00:00Z","summary":"Test rule","topic":"test","event":"pre_tool_use","instructions":["Test instruction."],"actions":["edit"],"tags":["test"],"importance":"medium","type":"declarative"}`
	if err := os.WriteFile(filepath.Join(rulesDir, "records.jsonl"), []byte(rule+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "experts", "init")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init failed: %s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Rules Mode: advisory") {
		t.Fatalf("default rules mode should be advisory, got: %s", output)
	}
	if !strings.Contains(output, "Consult repository rules automatically") {
		t.Fatalf("advisory instructions missing from output: %s", output)
	}
	if strings.Contains(output, "Before your first edit, ask the user once") {
		t.Fatalf("default output should not ask before consulting rules: %s", output)
	}
}

func TestInitOutFlag(t *testing.T) {
	bin := gscBinary(t)
	repoDir := initTempGitRepo(t)
	outPath := filepath.Join(t.TempDir(), "experts-context.md")

	// Run init with --out
	cmd := exec.Command(bin, "experts", "init", "--out", outPath)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init --out failed: %s", out)
	}

	// Should create the file
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "Domain Expert") {
		t.Errorf("output file should contain 'Domain Expert'")
	}

	// Should mention written to
	if !strings.Contains(string(out), "written to") {
		t.Errorf("output should mention 'written to', got: %s", out)
	}
}

func TestInitOutsideRepo(t *testing.T) {
	bin := gscBinary(t)
	tempDir := t.TempDir()

	// Run init outside a repo
	cmd := exec.Command(bin, "experts", "init")
	cmd.Dir = tempDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init outside repo failed: %s", out)
	}

	output := string(out)

	// Should succeed and mention personal scope
	if !strings.Contains(output, "personal") {
		t.Errorf("outside-repo output should mention 'personal', got: %s", output)
	}

	// Should mention repo scope unavailable
	if !strings.Contains(output, "Repo scope is unavailable") && !strings.Contains(output, "not in a git repository") {
		t.Errorf("outside-repo output should mention repo unavailable, got: %s", output)
	}
}

func TestInitOutsideRepoWithOut(t *testing.T) {
	bin := gscBinary(t)
	tempDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "experts-context.md")

	// Run init with --out outside a repo
	cmd := exec.Command(bin, "experts", "init", "--out", outPath)
	cmd.Dir = tempDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init --out outside repo failed: %s", out)
	}

	// Should create the file
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "personal") {
		t.Errorf("output file should mention personal scope")
	}

	if !strings.Contains(string(out), "cat "+outPath) {
		t.Errorf("output should tell the agent to read the written file, got: %s", out)
	}
}

func TestInitSilentMode(t *testing.T) {
	bin := gscBinary(t)
	repoDir := initTempGitRepo(t)

	// Run init with --silent
	cmd := exec.Command(bin, "experts", "init", "--silent")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init --silent failed: %s", out)
	}

	// Should produce no output
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("silent mode should produce no output, got: %s", out)
	}
}

func TestInitHelpText(t *testing.T) {
	bin := gscBinary(t)

	// Run init --help
	cmd := exec.Command(bin, "experts", "init", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsc experts init --help failed: %s", out)
	}

	output := string(out)

	// Should mention --out
	if !strings.Contains(output, "--out") {
		t.Errorf("help should mention --out flag, got: %s", output)
	}

	// Should mention stdout
	if !strings.Contains(output, "stdout") {
		t.Errorf("help should mention stdout, got: %s", output)
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
