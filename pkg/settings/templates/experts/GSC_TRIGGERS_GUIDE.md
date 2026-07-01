<!--
Component: GSC Triggers Guide
Block-UUID: e5f6a7b8-c9d0-1234-efab-567890123456
Parent-UUID: N/A
Version: 1.0.0
Description: On-demand reference guide for executable triggers. Covers V1 contract, runtimes, creation, validation, testing, and integration patterns.
Language: Markdown (Go Template)
Created-at: 2026-06-23T19:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0)
-->

# GSC Triggers Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc rules trigger new --runtime node --entry ...` | Create a new tool-trigger rule |
| `gsc rules trigger new --creator agent --target <repo\|personal> --from-file trigger.json` | Agent-safe trigger creation with required checklist (`--stdin` also supported) |
| `gsc rules trigger template` | Print minimal trigger template |
| `gsc rules trigger template --full` | Print full trigger template |
| `gsc rules trigger validate <id>` | Validate a trigger rule |
| `gsc rules trigger validate --all` | Validate all trigger rules |
| `gsc rules trigger run <id> --context <file>` | Execute a trigger with context |
| `gsc rules trigger run --all --context <file>` | Execute all triggers |
| `gsc rules test <id> --session <file>` | Replay-test trigger against session |
| `gsc rules list --type tool-trigger [--scope <all\|repo\|personal>]` | List trigger rules only |
| `gsc rules show <id> [--scope <all\|repo\|personal>]` | Show rule with ruleHash/triggerHash |

---

## 1. V1 Executable Trigger Contract

### Input (JSON on stdin)

```json
{
  "version": "1",
  "session": {
    "id": "019eeaab-...",
    "path": "/abs/session.jsonl",
    "cwd": "/tmp"
  },
  "conversation": {
    "leafId": "entry-9",
    "messageIds": ["entry-1", "entry-2", "entry-9"]
  },
  "model": {
    "provider": "xiaomi-token-plan-sgp",
    "id": "mimo-v2.5-pro",
    "thinkingLevel": "medium"
  },
  "toolCall": {
    "id": "call-001",
    "toolName": "read",
    "action": "read",
    "file": "/abs/repo/data/accounting/q1.ledger",
    "command": null,
    "input": { "path": "~/repo/data/accounting/q1.ledger" }
  },
  "repo": {
    "root": "/abs/repo",
    "normalizedFile": "data/accounting/q1.ledger"
  },
  "rule": {
    "id": "rule_...",
    "summary": "Accounting executable trigger",
    "type": "tool-trigger",
    "ruleHash": "sha256:...",
    "triggerHash": "sha256:..."
  }
}
```

### Required Fields

- `version`
- `session.id`, `session.path`, `session.cwd`
- `conversation.leafId`, `conversation.messageIds`
- `toolCall.id`, `toolCall.toolName`, `toolCall.action`, `toolCall.input`
- `rule.id`, `rule.summary`, `rule.type`, `rule.ruleHash`, `rule.triggerHash`

### Nullable Fields

- `model` (may be null)
- `repo` (may be null if outside repo)
- `toolCall.file` (null for bash commands)
- `toolCall.command` (null for file tools)
- `repo.normalizedFile` (null for bash commands)

### Action Values

| Action | Description |
| :--- | :--- |
| `read` | Built-in or mapped file read |
| `edit` | Built-in or mapped file edit |
| `write` | Built-in or mapped file write |
| `command` | Bash/shell command |
| `custom` | Unmapped custom tool |

### Output (JSON on stdout)

**Minimum allow:**
```json
{ "matched": false, "block": false }
```

**Matched but allow:**
```json
{
  "matched": true,
  "block": false,
  "notice": "No prior matching error was found."
}
```

**Block:**
```json
{
  "matched": true,
  "block": true,
  "message": "Before retrying, read docs/format.md.",
  "notice": "Blocked because guidance was required."
}
```

### Required Output Fields

| Field | Required | Description |
| :--- | :--- | :--- |
| `matched` | Yes | Does this rule apply? |
| `block` | Yes | Should this tool call stop? |
| `message` | When block=true | Model-facing block reason |
| `notice` | Optional | User-facing, never sent to LLM |

### Execution Rules

- `cwd` = `repo.root` when repo is known, otherwise `session.cwd`
- stdin receives context JSON
- stdout must be exactly one JSON object
- stderr is captured for diagnostics only
- Nonzero exit = trigger failed
- Timeout = trigger failed (default: 5000ms, max: 60000ms)

---

## 2. Supported Runtimes

| Runtime | Command | Extensions |
| :--- | :--- | :--- |
| `node` | `node <triggerPath>` | `.mjs`, `.js` |
| `python` | `python3 <triggerPath>` | `.py` |
| `bash` | `bash <triggerPath>` | `.sh` |

Runtime must be explicit. Do not infer from file extension.

---

## 3. Creating Triggers

AI agents must use `--creator agent --from-file <json>`. Human users may omit `--creator`.

Before building the JSON, verify the topic and lifecycle:
- Topic is required. Use `gsc topics list` first; if none fits, create one with `gsc topics add <slug> --description "..."`. Do not use `--target` with topic commands.
- Use `pre_tool_use` for checks before a tool call runs.
- Use `post_tool_use` for checks after a tool call runs. For "after editing files, verify compilation", use `post_tool_use`.
- If the rule has multiple actions, files, or globs, include all of them in `creatorChecklist.matching.actions`, `files`, or `globs`.

### From Flags

```bash
gsc rules trigger new \
  --target repo \
  --title "Go file read guidance" \
  --runtime node \
  --entry go-file-read.mjs \
  --instruction "Run 'gsc knowledge search go-style-guide' first" \
  --frequency once-per-session \
  --topic go-conventions
```

### From JSON File

```bash
# Create definition file for an agent-created trigger
cat > trigger-def.json << 'EOF'
{
  "type": "executable",
  "summary": "Block unsafe rg usage",
  "topic": "safety",
  "event": "pre_tool_use",
  "actions": ["bash"],
  "command_filter": "rg .*--hidden",
  "trigger": {
    "runtime": "node",
    "entry": "unsafe-rg.mjs",
    "timeoutMs": 3000
  },
  "frequency": {
    "mode": "once-per-rule-hash"
  },
  "instruction": {
    "mode": "inline",
    "text": "Use ripgrep with proper flags."
  },
  "enabled": true,
  "priority": 100,
  "creatorChecklist": {
    "creator": "agent",
    "intent": "Prevent unsafe ripgrep usage.",
    "scope": "repo",
    "ruleKind": "executable-trigger",
    "topic": {
      "slug": "safety",
      "source": "existing",
      "verifiedFrom": "gsc topics list"
    },
    "matching": {
      "event": "pre_tool_use",
      "actions": ["bash"],
      "matches": "rg .*--hidden"
    },
    "delivery": {
      "mode": "steer",
      "blocks": true,
      "messageShownToAgent": "Use ripgrep with proper flags."
    },
    "sideEffects": ["Runs local trigger code only."],
    "risk": {
      "level": "high",
      "reasons": ["Executable trigger", "Blocking behavior"]
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
}
EOF

# Create the rule after the trigger entry file exists
gsc rules trigger new --creator agent --target repo --from-file trigger-def.json
```

### Writing the Trigger File

Before creating the rule, write the trigger file to `.gitsense/rules/triggers/<entry>` so validation can resolve and hash it:

```javascript
// .gitsense/rules/triggers/unsafe-rg.mjs
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

const isRead = ctx.toolCall?.action === "read";
const isGoFile = ctx.repo?.normalizedFile?.endsWith(".go");
const applies = isRead && isGoFile;

console.log(JSON.stringify({
    matched: applies,
    block: applies,
    message: applies
        ? "Before reading Go files, run 'gsc knowledge search go-style-guide'."
        : undefined,
    notice: applies
        ? "Blocked: Go style guide must be consulted."
        : undefined
}));
```

---

## 4. Validation

### Validate Single Rule

```bash
gsc rules trigger validate rule_019efxxx
```

### Validate All Triggers

```bash
gsc rules trigger validate --all
```

### Validation Checks

- `type == "tool-trigger"`
- `trigger.runtime` is one of `node`, `python`, `bash`
- `trigger.entry` is non-empty
- Resolved path stays under `.gitsense/rules/triggers/`
- Trigger file exists
- Timeout is positive and ≤ 60000ms
- Rule has stable `ruleHash`
- Executable code has stable `triggerHash`

### Validate with Fixture

```bash
gsc rules trigger validate rule_019efxxx --context fixture.json
```

---

## 5. Hashing

### RuleHash

SHA-256 hash of delivery-affecting rule metadata:
- id, type, summary, instructions, topic, importance
- actions, glob_patterns, exclude_globs
- applies_to.files, applies_to.commands

### TriggerHash

SHA-256 hash of the executable file contents:
```
<git_root>/.gitsense/rules/triggers/<entry>
```

### Viewing Hashes

```bash
gsc rules show <rule-id> [--scope <all|repo|personal>]
```

Output includes:
```
RuleHash: sha256:...
TriggerHash: sha256:...
```

---

## 6. Testing

### Replay Against Session

```bash
gsc rules test <rule-id> --session /path/to/session.jsonl --format json
```

### Output Format

```json
{
  "rule": {
    "id": "rule_...",
    "type": "tool-trigger",
    "summary": "...",
    "ruleHash": "sha256:...",
    "triggerHash": "sha256:..."
  },
  "session": {
    "path": "/abs/session.jsonl",
    "leaf": "entry-9",
    "latestLeafUsed": true
  },
  "evaluated": 3,
  "matched": [
    {
      "toolCallId": "call-1",
      "toolName": "bash",
      "action": "command",
      "path": null,
      "command": "rg REV data/accounting/q1.ledger",
      "result": {
        "matched": true,
        "block": true,
        "message": "...",
        "notice": "..."
      }
    }
  ],
  "notMatched": [
    {
      "toolCallId": "call-2",
      "toolName": "read",
      "action": "read",
      "path": "README.md",
      "reason": "trigger returned matched=false"
    }
  ],
  "errors": []
}
```

---

## 7. Execution

### Run Single Trigger

```bash
gsc rules trigger run <rule-id> --context context.json
```

### Run All Triggers

```bash
gsc rules trigger run --all --context context.json
```

### With Custom Timeout

```bash
gsc rules trigger run <rule-id> --context context.json --timeout 10000
```

---

## 8. Frequency Modes

| Mode | Behavior |
| :--- | :--- |
| `always` | Deliver every time |
| `once-per-turn` | Once per agent turn |
| `once-per-context` | Once per context window |
| `once-per-session` | Once per session |
| `once-per-branch` | Once per branch |
| `once-per-file` | Once per file path |
| `once-per-rule-hash` | Once per rule hash (re-deliver on change) |

---

## 9. Security

- Never execute trigger paths outside `.gitsense/rules/triggers/`
- Runtime must be explicit (not inferred)
- Apply timeout (default 5000ms, max 60000ms)
- Capture stdout/stderr with size limits
- Do not pass secrets
- Do not give trigger access to Pi internals
- Fail open in integrations; `gsc rules test` reports errors

---

## 10. Integration with pi-brains

### Discovery

```bash
gsc rules get --file <path> --action <action> --format json
```

Returns `ruleHash` and `triggerHash` for matched rules.

### Execution

pi-brains builds V1 context and passes to trigger:
1. Resolve rule from `gsc rules get`
2. Build V1 context from tool call
3. Execute trigger with `node <triggerPath>`
4. Parse JSON response
5. Apply `matched`/`block`/`message`/`notice`

### Example pi-brains Flow

```
1. Agent calls tool: read("data/accounting/q1.ledger")
2. pi-brains queries: gsc rules get --file data/accounting/q1.ledger --action read --format json
3. Returns: rule with triggerHash
4. pi-brains builds V1 context
5. pi-brains executes: node .gitsense/rules/triggers/accounting-read.mjs
6. Trigger returns: { matched: true, block: true, message: "..." }
7. pi-brains blocks tool call and injects message
```

---

## 11. Best Practices

1. **Keep triggers simple** — They should recognize situations, not contain knowledge
2. **Store knowledge in GitSense** — Use lessons, notes, or rules for the "what"
3. **Use appropriate frequency** — `once-per-context` is usually sufficient
4. **Set priorities** — Higher priority triggers run first
5. **Test with fixtures** — Validate triggers in CI with `--context`
6. **Use query mode** — For dynamic knowledge, use `instruction.mode: query`

---

## 12. Common Mistakes

| Mistake | Correction |
| :--- | :--- |
| Forgetting to write trigger file | Rule creates metadata; you write the `.mjs` file |
| Using wrong runtime | Must be `node`, `python`, or `bash` |
| Missing `matched` field | Required in V1 output |
| Missing `message` when blocking | Required when `block=true` |
| Trigger path escapes directory | Entry must stay under `.gitsense/rules/triggers/` |
| Not testing before deploying | Use `gsc rules trigger validate --context` |
| Using `always` frequency unnecessarily | Use `once-per-context` or `once-per-file` |
