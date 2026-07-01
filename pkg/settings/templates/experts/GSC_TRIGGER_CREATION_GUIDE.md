<!--
Component: GSC Trigger Creation Guide
Block-UUID: b2c3d4e5-f6a7-8901-bcde-234567890123
Parent-UUID: N/A
Version: 1.0.0
Description: Step-by-step guide for guiding users to create executable triggers. Covers the full workflow from understanding intent to validation.
Language: Markdown (Go Template)
Created-at: 2026-06-23T21:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0)
-->

# GSC Trigger Creation Guide

## Purpose

This guide teaches you how to **guide users** through creating executable triggers. It's not just a command reference — it's a conversation playbook.

---

## The Conversation Flow

When a user says "I want to create a rule that does X", follow this flow:

### Step 1: Understand Intent

**Ask:** "What should happen when this situation is detected?"

| User Response | Behavior |
|---------------|----------|
| "Block it" | `block: true`, `message` required |
| "Allow but warn" | `block: false`, `notice` set |
| "Just log/monitor" | `block: false`, `notice` set |
| "Inject guidance" | `block: false`, `notice` set |

**Example questions:**
- "Should it block the action or just warn?"
- "What message should the user see?"
- "What should the agent do instead?"

### Step 2: Identify the Trigger

**Ask:** "When should this trigger fire?"

| Trigger Type | Example |
|--------------|---------|
| File read | "When someone reads a Go file" |
| File edit | "When someone edits README.md" |
| File write | "When someone writes to config/" |
| Bash command | "When someone runs rm -rf" |
| Specific file | "When someone touches foo.bar" |
| Glob pattern | "When someone edits anything in src/" |

### Step 3: Determine Frequency

**Ask:** "How often should this trigger fire?"

| Frequency | Use Case |
|-----------|----------|
| `always` | Every time (noisy, use sparingly) |
| `once-per-turn` | Once per agent turn |
| `once-per-context` | Once per context window (default) |
| `once-per-session` | Once per session |
| `once-per-file` | Once per file path |
| `once-per-rule-hash` | Once until rule changes |

**Recommend:** "For most cases, `once-per-context` is sufficient. Use `once-per-session` for important policies."

### Step 4: Choose Runtime

| Runtime | When to Use |
|---------|-------------|
| `node` | Default, good for JSON parsing |
| `python` | User prefers Python |
| `bash` | Simple string matching |

**Recommend:** "Node is the most common choice. Python works well too."

### Step 5: Create the Rule

**Command template:**
```bash
gsc rules trigger new \
  --title "<title>" \
  --runtime <node|python|bash> \
  --entry <filename>.<ext> \
  --instruction "<fallback message>" \
  --frequency <mode> \
  --topic <topic-slug>
```

**Example:**
```bash
gsc rules trigger new \
  --title "Go file read guidance" \
  --runtime node \
  --entry go-file-read.mjs \
  --instruction "Run 'gsc knowledge search go-style-guide' first" \
  --frequency once-per-session \
  --topic go-conventions
```

### Step 6: Write the Trigger File

**Location:** `.gitsense/rules/triggers/<entry>`

**V1 Contract:**
```javascript
// .gitsense/rules/triggers/<entry>
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

// 1. Check if this rule applies
const applies = <condition>;

// 2. Determine if we should block
const shouldBlock = applies && <block-condition>;

// 3. Build output
console.log(JSON.stringify({
    matched: applies,
    block: shouldBlock,
    message: shouldBlock ? "<model-facing message>" : undefined,
    notice: applies ? "<user-facing notice>" : undefined
}));
```

**Condition examples:**

```javascript
// File read on specific extension
const isRead = ctx.toolCall?.action === "read";
const isGoFile = ctx.repo?.normalizedFile?.endsWith(".go");
const applies = isRead && isGoFile;

// Edit on specific file
const isEdit = ctx.toolCall?.action === "edit";
const isTargetFile = ctx.repo?.normalizedFile === "foo.bar";
const applies = isEdit && isTargetFile;

// Edit on glob pattern
const isEdit = ctx.toolCall?.action === "edit";
const isAccountingFile = ctx.repo?.normalizedFile?.startsWith("data/accounting/");
const applies = isEdit && isAccountingFile;

// Bash command with specific pattern
const isCommand = ctx.toolCall?.action === "command";
const hasRiskyCommand = ctx.toolCall?.command?.includes("rm -rf");
const applies = isCommand && hasRiskyCommand;
```

### Step 7: Validate

```bash
# Validate the rule
gsc rules trigger validate <rule-id>

# Test with a fixture
gsc rules trigger run <rule-id> --context fixture.json
```

### Step 8: Verify in Tree

```bash
# Show where the rule applies
gsc rules tree --rule-id <rule-id>

# Show with all rules
gsc rules tree
```

---

## Complete Example: Block Go File Edits

**User request:** "I want to block edits to Go files until the user acknowledges the style guide."

### Step 1: Understand
- **What:** Block edits to Go files
- **When:** Before edit action
- **Message:** "Run 'gsc knowledge search go-style-guide' first"

### Step 2: Identify
- **Action:** `edit`
- **Target:** `*.go` files
- **Condition:** `ctx.toolCall?.action === "edit"` AND `ctx.repo?.normalizedFile?.endsWith(".go")`

### Step 3: Frequency
- **Recommendation:** `once-per-session` (don't block every file)
- **User choice:** `once-per-session`

### Step 4: Runtime
- **Recommendation:** `node`
- **User choice:** `node`

### Step 5: Create Rule

```bash
gsc rules trigger new \
  --title "Block Go file edits" \
  --runtime node \
  --entry block-go-edit.mjs \
  --instruction "Run 'gsc knowledge search go-style-guide' before editing Go files" \
  --frequency once-per-session \
  --topic go-conventions
```

### Step 6: Write Trigger

```javascript
// .gitsense/rules/triggers/block-go-edit.mjs
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

const isEdit = ctx.toolCall?.action === "edit";
const isGoFile = ctx.repo?.normalizedFile?.endsWith(".go");
const applies = isEdit && isGoFile;

console.log(JSON.stringify({
    matched: applies,
    block: applies,
    message: applies
        ? "Before editing Go files, run 'gsc knowledge search go-style-guide' and apply the returned guidance."
        : undefined,
    notice: applies
        ? "Blocked: Go style guide must be consulted before editing Go files."
        : undefined
}));
```

### Step 7: Validate

```bash
gsc rules trigger validate <rule-id>
```

### Step 8: Verify

```bash
gsc rules tree --rule-id <rule-id>
```

---

## Common Patterns

### Pattern 1: Block Specific File

```javascript
const isEdit = ctx.toolCall?.action === "edit";
const isTarget = ctx.repo?.normalizedFile === "config/production.yml";
const applies = isEdit && isTarget;
```

### Pattern 2: Block Glob Pattern

```javascript
const isEdit = ctx.toolCall?.action === "edit";
const isAccounting = ctx.repo?.normalizedFile?.startsWith("data/accounting/");
const applies = isEdit && isAccounting;
```

### Pattern 3: Block Bash Command

```javascript
const isCommand = ctx.toolCall?.action === "command";
const hasRisky = ctx.toolCall?.command?.includes("rm -rf");
const applies = isCommand && hasRisky;
```

### Pattern 4: Warn on Read

```javascript
const isRead = ctx.toolCall?.action === "read";
const isSensitive = ctx.repo?.normalizedFile?.startsWith("secrets/");
const applies = isRead && isSensitive;
```

### Pattern 5: Notice-Only (No Block)

```javascript
const isEdit = ctx.toolCall?.action === "edit";
const isTracked = ctx.repo?.normalizedFile?.endsWith(".md");
const applies = isEdit && isTracked;

console.log(JSON.stringify({
    matched: applies,
    block: false,  // Don't block
    notice: applies
        ? "Warning: Markdown file detected. Consider running markdown linter."
        : undefined
}));
```

---

## Troubleshooting

### Trigger doesn't fire

1. Check if rule is enabled: `gsc rules show <id> [--scope <all|repo|personal>]`
2. Check if trigger file exists: `ls .gitsense/rules/triggers/`
3. Validate: `gsc rules trigger validate <id>`
4. Test with fixture: `gsc rules trigger run <id> --context fixture.json`

### Trigger fires but doesn't block

1. Check `block` field is `true`
2. Check `message` is non-empty when `block: true`
3. Check `matched` is `true`

### Trigger throws error

1. Check stderr output
2. Validate JSON output
3. Check for syntax errors in trigger file

---

## Quick Reference

| Step | Command |
|------|---------|
| Create rule | `gsc rules trigger new --title "..." --runtime node --entry ...` |
| Write trigger | Edit `.gitsense/rules/triggers/<entry>` |
| Validate | `gsc rules trigger validate <id>` |
| Test | `gsc rules trigger run <id> --context fixture.json` |
| Verify | `gsc rules tree --rule-id <id>` |
| Debug | `gsc rules show <id> [--scope <all|repo|personal>]` |

---

## Questions to Ask the User

1. "What should happen when this situation is detected?"
2. "Should it block the action or just warn?"
3. "What message should the user see?"
4. "What should the agent do instead?"
5. "How often should this trigger fire?"
6. "Which runtime do you prefer? (node/python/bash)"
7. "What topic should this rule be filed under?"

---

## Output Schema Reminder

### Input (V1)

```json
{
  "version": "1",
  "session": { "id": "...", "path": "...", "cwd": "..." },
  "conversation": { "leafId": "...", "messageIds": [...] },
  "toolCall": {
    "id": "...",
    "toolName": "read|edit|write|bash",
    "action": "read|edit|write|command|custom",
    "file": "/abs/path" | null,
    "command": "cmd" | null,
    "input": {}
  },
  "repo": { "root": "...", "normalizedFile": "..." | null },
  "rule": { "id": "...", "summary": "...", "type": "tool-trigger", "ruleHash": "...", "triggerHash": "..." }
}
```

### Output (V1)

```json
{
  "matched": true|false,
  "block": true|false,
  "message": "required when block=true",
  "notice": "optional, user-facing"
}
```
