package rules

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
)

func TestRulesTriggerNewCommandRegistered(t *testing.T) {
	cmd := NewCmd()

	triggerCmd, _, err := cmd.Find([]string{"trigger"})
	if err != nil {
		t.Fatalf("Find(trigger) error: %v", err)
	}
	if triggerCmd == nil || triggerCmd.Name() != "trigger" {
		t.Fatalf("Find(trigger) = %v, want trigger command", triggerCmd)
	}

	newCmd, _, err := cmd.Find([]string{"trigger", "new"})
	if err != nil {
		t.Fatalf("Find(trigger new) error: %v", err)
	}
	if newCmd == nil || newCmd.Name() != "new" {
		t.Fatalf("Find(trigger new) = %v, want new command", newCmd)
	}
	if flag := newCmd.Flags().Lookup("target"); flag == nil {
		t.Fatal("trigger new command missing --target flag")
	}
	if flag := newCmd.Flags().Lookup("creator"); flag == nil {
		t.Fatal("trigger new command missing --creator flag")
	}
}

func TestRulesTemplateCommandRegistered(t *testing.T) {
	cmd := NewCmd()
	templateCmd, _, err := cmd.Find([]string{"template"})
	if err != nil {
		t.Fatal(err)
	}
	if templateCmd == nil {
		t.Fatal("rules template command missing")
	}
	if flag := templateCmd.Flags().Lookup("type"); flag == nil {
		t.Fatal("rules template command missing --type flag")
	}
}

func TestRulesTemplateMatchesNewTemplate(t *testing.T) {
	got := executeRulesCommandOutput(t, "template")
	want := executeRulesCommandOutput(t, "new", "--template")
	if got != want {
		t.Fatalf("rules template output differs from rules new --template\n got: %s\nwant: %s", got, want)
	}
	if !strings.Contains(got, `"instructions"`) {
		t.Fatalf("expected declarative template, got: %s", got)
	}
}

func TestRulesTemplateExecutableMatchesNewTemplate(t *testing.T) {
	got := executeRulesCommandOutput(t, "template", "--type", "executable")
	want := executeRulesCommandOutput(t, "new", "--template", "--type", "executable")
	if got != want {
		t.Fatalf("rules template --type executable output differs from rules new --template --type executable\n got: %s\nwant: %s", got, want)
	}
	if !strings.Contains(got, `"type": "executable"`) {
		t.Fatalf("expected executable template, got: %s", got)
	}
}

func TestRulesWriteCommandsExposeCreatorFlag(t *testing.T) {
	cmd := NewCmd()

	for _, args := range [][]string{
		{"new"},
		{"update"},
		{"trigger", "new"},
	} {
		found, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error: %v", args, err)
		}
		if found == nil {
			t.Fatalf("Find(%v) returned nil", args)
		}
		if flag := found.Flags().Lookup("creator"); flag == nil {
			t.Fatalf("%s command missing --creator flag", found.CommandPath())
		}
	}
}

func executeRulesCommandOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rules %v failed: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func TestRulesNewCreatorAgentRequiresJSONMode(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{
		"new",
		"--creator", "agent",
		"--target", "personal",
		"--summary", "Test rule",
		"--topic", "test",
		"--instruction", "Do the thing.",
		"--action", "edit",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --creator agent flag-mode creation to fail")
	}
	if !strings.Contains(err.Error(), "--creator agent requires --from-file or --stdin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRulesTriggerNewCreatorAgentRequiresFromFile(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{
		"trigger", "new",
		"--creator", "agent",
		"--target", "personal",
		"--title", "Test trigger",
		"--runtime", "node",
		"--entry", "test.mjs",
		"--topic", "test",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --creator agent flag-mode trigger creation to fail")
	}
	if !strings.Contains(err.Error(), "--creator agent requires --from-file or --stdin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRulesNewCreatorAgentFromFileCreatesRuleAndStripsChecklist(t *testing.T) {
	repoDir := initRulesTempGitRepo(t)
	writeTopicFixture(t, repoDir)
	restore := chdirForTest(t, repoDir)
	defer restore()

	ruleJSON := `{
  "summary": "Require TypeScript checks",
  "topic": "agent-lifecycle-rules",
  "event": "pre_tool_use",
  "instructions": ["Run tsc --noEmit after editing TypeScript files."],
  "actions": ["edit"],
  "glob_patterns": ["**/*.ts"],
  "importance": "medium",
  "creatorChecklist": {
    "creator": "agent",
    "intent": "Ensure agents verify TypeScript compilation after edits.",
    "scope": "repo",
    "ruleKind": "declarative",
    "topic": {
      "slug": "agent-lifecycle-rules",
      "source": "existing",
      "verifiedFrom": "gsc topics list"
    },
    "matching": {
      "event": "pre_tool_use",
      "action": "edit",
      "glob": "**/*.ts"
    },
    "risk": {
      "level": "medium"
    },
    "verification": {
      "syntaxVerifiedFrom": "gsc experts guide rules"
    },
    "unresolved": []
  }
}`
	rulePath := filepath.Join(t.TempDir(), "rule.json")
	if err := os.WriteFile(rulePath, []byte(ruleJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewCmd()
	cmd.SetArgs([]string{"new", "--creator", "agent", "--target", "repo", "--from-file", rulePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rules new --creator agent failed: %v", err)
	}

	records, err := rulespkg.LoadRecordsFromTarget("repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].CreatorChecklist != nil {
		t.Fatal("creatorChecklist should be stripped before saving")
	}
}

func TestRulesTriggerNewCreatorAgentStdinCreatesRuleAndStripsChecklist(t *testing.T) {
	repoDir := initRulesTempGitRepo(t)
	writeTopicFixture(t, repoDir)
	triggerDir := filepath.Join(repoDir, ".gitsense", "rules", "triggers")
	if err := os.MkdirAll(triggerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(triggerDir, "ts-check.mjs"), []byte(`console.log(JSON.stringify({matched:false, block:false}));`), 0644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, repoDir)
	defer restore()

	triggerJSON := `{
  "type": "executable",
  "summary": "Require TypeScript compile steer",
  "topic": "agent-lifecycle-rules",
  "event": "pre_tool_use",
  "actions": ["edit"],
  "glob_patterns": ["**/*.ts"],
  "trigger": {
    "runtime": "node",
    "entry": "ts-check.mjs",
    "timeoutMs": 5000
  },
  "instruction": {
    "mode": "inline",
    "text": "Run tsc --noEmit and fix errors before continuing."
  },
  "frequency": {
    "mode": "once-per-file"
  },
  "enabled": true,
  "creatorChecklist": {
    "creator": "agent",
    "intent": "Ensure agents fix TypeScript compilation errors.",
    "scope": "repo",
    "ruleKind": "executable-trigger",
    "topic": {
      "slug": "agent-lifecycle-rules",
      "source": "existing",
      "verifiedFrom": "gsc topics list"
    },
    "matching": {
      "event": "pre_tool_use",
      "action": "edit",
      "glob": "**/*.ts"
    },
    "delivery": {
      "mode": "steer",
      "blocks": true,
      "messageShownToAgent": "Run tsc --noEmit and fix errors before continuing."
    },
    "sideEffects": ["Runs local TypeScript compiler read-only."],
    "risk": {
      "level": "high",
      "reasons": ["Executable trigger", "Blocking steer delivery"]
    },
    "verification": {
      "lifecycleSupportVerifiedFrom": "gsc experts guide rules",
      "syntaxVerifiedFrom": "gsc rules trigger template",
      "deliveryModeVerifiedFrom": "gsc experts guide rules",
      "validationPlan": ["gsc rules trigger validate <created-rule-id>"]
    },
    "confirmation": {
      "required": true,
      "userConfirmed": true,
      "confirmedText": "confirm"
    },
    "unresolved": []
  }
}`

	cmd := NewCmd()
	cmd.SetArgs([]string{"trigger", "new", "--creator", "agent", "--target", "repo", "--stdin"})
	cmd.SetIn(strings.NewReader(triggerJSON))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rules trigger new --creator agent --stdin failed: %v", err)
	}

	records, err := rulespkg.LoadRecordsFromTarget("repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].CreatorChecklist != nil {
		t.Fatal("creatorChecklist should be stripped before saving")
	}
}

func initRulesTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
	return dir
}

func writeTopicFixture(t *testing.T, repoDir string) {
	t.Helper()
	topicsDir := filepath.Join(repoDir, ".gitsense", "topics")
	if err := os.MkdirAll(topicsDir, 0755); err != nil {
		t.Fatal(err)
	}
	topic := `{"slug":"agent-lifecycle-rules","description":"Lifecycle-aware GitSense rules","created_at":"2026-06-24T00:00:00Z","updated_at":"2026-06-24T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(topicsDir, "records.jsonl"), []byte(topic+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}
}
