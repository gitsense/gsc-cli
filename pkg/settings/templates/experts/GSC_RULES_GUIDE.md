<!--
Component: GSC Rules Guide
Block-UUID: c1d2e3f4-a5b6-7890-cdef-890123456789
Parent-UUID: N/A
Version: 3.1.0
Description: On-demand reference guide for gsc rules commands. Covers rule creation, querying, management, tool-trigger rules, lifecycle events, and agent integration patterns.
Language: Markdown (Go Template)
Created-at: 2026-06-21T03:00:00Z
Updated-at: 2026-06-24T12:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), terrchen (v3.1.0)
Changelog:
  v3.1.0 - Add lifecycle events documentation
-->

# GSC Rules Guide

## Quick Reference Card

### Instruction Rules

| Command | Purpose |
| :--- | :--- |
| `gsc rules new --target <repo\|personal>` | Create a rule from flags, file, or stdin |
| `gsc rules new --creator agent --target <repo\|personal> --from-file rule.json` | Agent-safe rule creation with required checklist |
| `gsc rules template` | Print a rule template |
| `gsc rules new --template` | Print a rule template from the create command |
| `gsc rules get --file <path> [--scope <all\|repo\|personal>]` | Query rules for a specific file |
| `gsc rules get --file <path> --action edit [--scope <all\|repo\|personal>]` | Query rules for a file when editing |
| `gsc rules get --event pre_tool_use --file <path> --action edit` | Query rules for a lifecycle event |
| `gsc rules get --event agent_end` | Query rules for agent-end event |
| `gsc rules get --glob <pattern> [--scope <all\|repo\|personal>]` | Query rules matching a glob pattern |
| `gsc rules get --tag <tag> [--scope <all\|repo\|personal>]` | Query rules by tag |
| `gsc rules get --format rules-json` | Get rules in execute-compatible format |
| `gsc rules execute --context <ctx> --rules <rules>` | Execute matched rules against context |
| `gsc rules update --target <repo\|personal> --id <id>` | Update an existing rule (requires `--changelog`) |
| `gsc rules delete <id> --target <repo\|personal>` | Delete a rule |
| `gsc rules list [--scope <all\|repo\|personal>]` | List rules |
| `gsc rules show <id> [--scope <all\|repo\|personal>]` | Show a rule in detail |
| `gsc rules search <query> [--scope <all\|repo\|personal>]` | Full-text search rules |
| `gsc rules tags [--scope <all\|repo\|personal>]` | List rule tags with counts |
| `gsc rules overview [--scope <all\|repo\|personal>]` | Summary digest for rules |
| `gsc rules build --target <repo\|personal>` | Rebuild the gsc-rules Brain for repo target, or manifest for personal target |

### Tool-Trigger Rules

| Command | Purpose |
| :--- | :--- |
| `gsc rules trigger new --target <repo\|personal>` | Create a new tool-trigger rule |
| `gsc rules trigger new --creator agent --target <repo\|personal> --from-file trigger.json` | Agent-safe trigger creation with required checklist (`--stdin` also supported) |
| `gsc rules trigger template` | Print a trigger template |
| `gsc rules trigger validate <id>` | Validate a trigger rule |
| `gsc rules trigger validate --all` | Validate all trigger rules |
| `gsc rules trigger run <id> --context <file>` | Execute a single trigger |
| `gsc rules trigger run --all --context <file>` | Execute all triggers (sequential) |
| `gsc rules execute --context <ctx> --rules <rules>` | Execute matched rules (parallel) |
| `gsc rules list --type tool-trigger [--scope <all\|repo\|personal>]` | List trigger rules only |
| `gsc rules tree` | Visualize rule scoping across the repository |
| `gsc rules tree --rule-id <id>` | Show specific rule in tree |
| `gsc rules tree --topic <slug>` | Filter by topic |
| `gsc rules tree --type tool-trigger` | Filter by rule type |
| `gsc rules tree --action edit` | Filter by action |

---

## 1. Rule Types

GitSense supports two types of rules:

### Instruction Rules (Default)

Static instructions that agents should follow when matching files/actions. These are advisory and query-based. The agent queries rules before making changes and follows the returned instructions.

**Use case:** "When editing files in `internal/cli/`, don't run `gofmt -w`."

### Tool-Trigger Rules

Executable triggers that evaluate runtime context before tool calls. They can inject knowledge or block actions based on the situation.

**Key insight:** Tool-trigger rules separate **when** (trigger code), **what** (GitSense knowledge), and **how often** (pi-brains delivery state).

**Use case:** "Before editing `README.md`, run `gsc rules get --file README.md` and apply the returned rules."

---

## 2. Lifecycle Events

GitSense rules bind to canonical lifecycle events. Runtime-specific event names (like Pi's `tool_call` or Claude's `PreToolUse`) are mapped into these canonical names.

### Canonical Events

| Event | Description |
| :--- | :--- |
| `session_start` | Session initialization |
| `before_agent_start` | Before agent loop begins (context injection) |
| `user_prompt_submit` | User prompt received |
| `agent_start` | Agent loop begins (notification only) |
| `pre_tool_use` | Before tool execution (default) |
| `post_tool_use` | After tool execution |
| `post_tool_batch` | After batch of tools |
| `context` | Before each LLM call (message injection only) |
| `session_before_compact` | Before compaction (cancel/customize only) |
| `session_compact` | After compaction (notification only) |
| `agent_end` | Agent loop ends |
| `session_end` | Session cleanup |

### Runtime Aliases

| GitSense event | Pi support | Claude alias |
| :--- | :--- | :--- |
| `session_start` | future | `SessionStart` |
| `before_agent_start` | future | `BeforeAgentStart` |
| `user_prompt_submit` | future | `UserPromptSubmit` |
| `agent_start` | future | `AgentStart` |
| `pre_tool_use` | `tool_call` | `PreToolUse` |
| `post_tool_use` | future | `PostToolUse` |
| `post_tool_batch` | future | `PostToolBatch` |
| `context` | future | `Context` |
| `session_before_compact` | future | `SessionBeforeCompact` |
| `session_compact` | future | `SessionCompact` |
| `agent_end` | future | `AgentEnd` |
| `session_end` | future | `SessionEnd` |

**Current status:** Pi currently supports `pre_tool_use` through `tool_call`. Other events are planned for future Pi extension support.

### Backwards Compatibility

Existing rules without an `event` field default to `pre_tool_use`. This preserves backwards compatibility with all existing rules.

### Querying by Event

```bash
# Query for pre_tool_use rules (default)
gsc rules get --file src/foo.ts --action edit

# Explicit event query
gsc rules get --event pre_tool_use --file src/foo.ts --action edit

# Query for agent-end rules
gsc rules get --event agent_end

# Query for session_start rules
gsc rules get --event session_start --format json
```

### Creating Event-Bound Rules

```bash
# Create a stop rule
gsc rules new --event stop \
  --summary "Completion gate" \
  --instruction "Run tests before finishing" \
  --action command \
  --topic safety

# Create from JSON with event
cat <<EOF | gsc rules new --stdin
{
  "event": "pre_tool_use",
  "type": "tool-trigger",
  "summary": "Require policy lookup before risky edits",
  "topic": "safety",
  "actions": ["edit", "write"],
  "glob_patterns": ["**/*.ts"],
  "trigger": {
    "runtime": "node",
    "entry": "policy-check.mjs"
  },
  "frequency": {
    "mode": "once-per-context"
  }
}
EOF
```

### Reusable Trigger Scripts

Trigger scripts can be reused across events by inspecting `ctx.event.name`:

```javascript
// .gitsense/rules/triggers/lifecycle-aware.mjs
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

switch (ctx.event.name) {
  case "pre_tool_use": {
    const toolCall = ctx.payload.toolCall;
    console.log(JSON.stringify({
      matched: true,
      block: false,
      notice: `Checked ${toolCall.action} on ${toolCall.file || toolCall.command}`
    }));
    break;
  }

  case "stop": {
    console.log(JSON.stringify({
      matched: true,
      block: false,
      notice: "Stop checks passed."
    }));
    break;
  }

  default:
    console.log(JSON.stringify({ matched: false, block: false }));
}
```

### Important Notes

- Not all canonical events are wired in every runtime yet.
- Pi currently supports `pre_tool_use` through `tool_call`.
- Future Pi/Claude adapters should reuse the same canonical event names.
- Trigger scripts should inspect `ctx.event.name` instead of assuming tool-call context.

---

## 3. Creating Instruction Rules

### From Flags

```bash
gsc rules new --glob "internal/cli/**" \
  --summary "CLI file conventions" \
  --instruction "Do not run gofmt -w" \
  --instruction "Bump the Version field" \
  --action edit --action write \
  --importance high \
  --owner "team-name" \
  --contact "email@example.com"
```

### From a Template

```bash
# Preferred symmetric form
gsc rules template > /tmp/rule.json

# Backward-compatible create-command form
gsc rules new --template > /tmp/rule.json

# Executable rule template
gsc rules template --type executable > /tmp/executable-rule.json

# Edit the template
vim /tmp/rule.json

# Create the rule
gsc rules new --from-file /tmp/rule.json
```

### From Stdin

```bash
cat rule.json | gsc rules new --stdin
```

### Bash Action Requirements

When using `--action bash`, the `--matches` flag is required. This enforces explicit intent and prevents accidental broad matching.

```bash
# Specific pattern - matches commands containing "rm -rf"
gsc rules new --event pre_tool_use --action bash --matches "rm -rf" \
  --summary "Block rm -rf" --instruction "Do not run rm -rf" --topic safety

# Match all commands (explicit wildcard)
gsc rules new --event pre_tool_use --action bash --matches ".*" \
  --summary "Log all commands" --instruction "Log command for audit" --topic audit

# Case-insensitive matching
gsc rules new --event pre_tool_use --action bash --matches "RM -RF" \
  --case-insensitive --summary "Block rm -rf" --instruction "..." --topic safety
```

**Error when missing `--matches`:**
```
Error: bash action requires --matches flag (use .* to match all commands)
```

### Prompt Action Requirements

When using `--action prompt`, the `--matches` flag is required. This enables declarative rules that filter based on prompt content, including API key detection.

```bash
# Security rule - block AWS keys
gsc rules new --event user_prompt_submit --action prompt \
  --matches "AKIA[0-9A-Z]{16}" \
  --summary "Block AWS keys in prompts" \
  --instruction "Do not share AWS keys in prompts" \
  --topic security

# Security rule - block multiple key patterns
gsc rules new --event user_prompt_submit --action prompt \
  --matches "AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{36,}|sk-[a-zA-Z0-9]{48}" \
  --summary "Block common API keys" \
  --instruction "Do not share API keys in prompts" \
  --topic security

# Audit rule - log all prompts
gsc rules new --event user_prompt_submit --action prompt \
  --matches ".*" \
  --summary "Log all prompts for audit" \
  --instruction "Log prompt content for security audit" \
  --topic audit

# Case-insensitive matching
gsc rules new --event user_prompt_submit --action prompt \
  --matches "PRODUCTION CREDENTIALS" \
  --case-insensitive \
  --summary "Block prompts about production credentials" \
  --instruction "Do not ask about production credentials" \
  --topic security
```

**Querying prompt rules:**
```bash
# Query rules for a specific prompt
gsc rules get --event user_prompt_submit --action prompt --prompt "AKIA1234567890ABCDEF" --format json

# Query rules matching a pattern
gsc rules get --event user_prompt_submit --action prompt --prompt "How do I use production credentials?" --format json
```

**Error when missing `--matches`:**
```
Error: prompt action requires --matches flag (use .* to match all prompts)
```

---

## 4. Creating Tool-Trigger Rules

### Quick Start

```bash
# Human-friendly flag path
gsc rules trigger new \
  --target repo \
  --title "README edit guidance" \
  --runtime node \
  --entry readme-edit-guidance.mjs \
  --instruction "Verify command tables match gsc --help output" \
  --frequency once-per-context \
  --topic documentation
```

This creates:
1. A rule record in `.gitsense/rules/records.jsonl`
2. A reference to a trigger file in `.gitsense/rules/triggers/readme-edit-guidance.mjs`

### Agent-Safe JSON Path

Preflight before writing JSON:
1. Run `gsc topics list` and choose an existing topic, or run `gsc topics add <slug> --description "..."` to create one. Topic commands do not take `--target`.
2. Choose the lifecycle event precisely. Use `post_tool_use` when the trigger checks the result after an edit.
3. Include every action/file/glob from the rule in `creatorChecklist.matching`.

```bash
gsc rules trigger new --creator agent --target personal --from-file trigger-def.json
```

Example `trigger-def.json` for an agent-created trigger:
```json
{
  "type": "executable",
  "summary": "Require TypeScript compilation checks for .ts edits",
  "topic": "typescript",
  "event": "post_tool_use",
  "actions": ["edit"],
  "glob_patterns": ["**/*.ts", "**/*.tsx"],
  "trigger": {
    "runtime": "node",
    "entry": "ts-compile-check.mjs",
    "timeoutMs": 5000
  },
  "instruction": {
    "mode": "inline",
    "text": "Run `tsc --noEmit` and fix all TypeScript compilation errors before continuing."
  },
  "frequency": {
    "mode": "once-per-file"
  },
  "priority": 100,
  "enabled": true,
  "creatorChecklist": {
    "creator": "agent",
    "intent": "Ensure agents verify TypeScript compilation after .ts edits.",
    "scope": "personal",
    "ruleKind": "executable-trigger",
    "topic": {
      "slug": "typescript",
      "source": "existing",
      "verifiedFrom": "gsc topics list"
    },
    "matching": {
      "event": "post_tool_use",
      "actions": ["edit"],
      "globs": ["**/*.ts", "**/*.tsx"]
    },
    "delivery": {
      "mode": "steer",
      "blocks": true,
      "messageShownToAgent": "Run `tsc --noEmit` and fix all TypeScript compilation errors before continuing."
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
}
```

### Trigger Template

Print a template to customize:

```bash
# Minimal template
gsc rules trigger template > .gitsense/rules/triggers/my-trigger.mjs

# Full template with all context fields
gsc rules trigger template --full > .gitsense/rules/triggers/my-trigger.mjs
```

---

## 4. Tool-Trigger Architecture

### The Separation

```
trigger code = when (evaluates context)
GitSense knowledge = what to tell the agent (instruction)
pi-brains = when/how often to inject it (frequency state)
```

This is what makes tool-triggers different from generic hooks. Hooks can run scripts. GitSense makes the knowledge searchable, reviewable, durable, and shareable.

### V1 Executable Trigger Contract

**Input** (JSON on stdin):
```json
{
  "version": "1",
  "session": {
    "id": "019eeaab-9332-731b-9b97-9eb30c11dcac",
    "path": "/Users/agent/.pi/sessions/example.jsonl",
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
    "file": "/repo/data/accounting/q1.ledger",
    "command": null,
    "input": { "path": "~/repo/data/accounting/q1.ledger" }
  },
  "repo": {
    "root": "/repo",
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

**For bash commands:**
```json
{
  "toolCall": {
    "id": "call-002",
    "toolName": "bash",
    "action": "command",
    "file": null,
    "command": "rg REV data/accounting/q1.ledger",
    "input": { "command": "rg REV data/accounting/q1.ledger" }
  },
  "repo": {
    "root": "/repo",
    "normalizedFile": null
  }
}
```

**Required fields:** `version`, `session.id`, `session.path`, `session.cwd`, `conversation.leafId`, `conversation.messageIds`, `toolCall.id`, `toolCall.toolName`, `toolCall.action`, `toolCall.input`, `rule.id`, `rule.summary`, `rule.type`, `rule.ruleHash`, `rule.triggerHash`

**Nullable fields:** `model`, `repo`, `toolCall.file`, `toolCall.command`, `repo.normalizedFile`

**Action values:**
- `read` — built-in or mapped file read
- `edit` — built-in or mapped file edit
- `write` — built-in or mapped file write
- `command` — bash/shell command
- `custom` — unmapped custom tool

### Output (V1 Schema)

**Minimum allow:**
```json
{
  "matched": false,
  "block": false
}
```

**Matched but allow:**
```json
{
  "matched": true,
  "block": false,
  "notice": "No prior matching error was found, so the command can continue."
}
```

**Block:**
```json
{
  "matched": true,
  "block": true,
  "message": "Before retrying, read docs/accounting-format.md and apply the ledger parsing rules.",
  "notice": "Blocked ledger read because accounting guidance was required."
}
```

### Output Fields

| Field | Required | Description |
| :--- | :--- | :--- |
| `matched` | Yes | Does this rule apply to this event? Controls inclusion in aggregate results. |
| `block` | Yes | Should this tool call stop? `true` = block and inject `message`. |
| `message` | When block=true | Model-facing block reason. May be injected into LLM context. |
| `notice` | Optional | User/operator-facing. Never sent to LLM. Shown in UI, persisted in logs. |
| `deliveryMode` | Optional | Delivery hint: `"steer"` (immediate), `"followUp"` (after current turn), `"passiveSteer"` (only if agent is active). Passed through to `gsc rules execute` output. |

### Execution Rules

- Context is passed on stdin
- stdout must be exactly one JSON object
- stderr is captured for status/errors only
- `cwd` = `repo.root` when repo is known, otherwise `session.cwd`
- nonzero exit = trigger failed
- timeout = trigger failed (default: 5000ms)
- invalid JSON = trigger failed
- default failure behavior: **fail-open** (visible diagnostics)

### Invalid Outputs

- non-JSON stdout
- missing `matched`
- missing `block`
- non-boolean `matched`/`block`
- `block=true` with missing or empty `message`
- non-zero exit
- timeout

### Frequency Modes

| Mode | Behavior |
| :--- | :--- |
| `always` | Inject every time |
| `once-per-turn` | Once per agent turn |
| `once-per-context` | Once per context window |
| `once-per-session` | Once per session |
| `once-per-branch` | Once per branch |
| `once-per-file` | Once per file per scope |

---

## When a GitSense Rule Blocks a Lifecycle Event

If pi-brains blocks a lifecycle event with a GitSense rule message, treat it as **required repository context**, not as a tool failure.

### The Matched-Rule Packet

When GitSense blocks a lifecycle event, it delivers a **complete matched-rule packet** that includes:

1. **All matched deterministic instruction rules** — Static instructions that apply to this file/action
2. **All matched executable trigger rules** — Runtime triggers that evaluated this event
3. **Trigger results** — Whether each trigger blocked, allowed, or errored
4. **Required next steps** — What to do before retrying

### Example Block Message

```text
GitSense matched repository rules before this lifecycle event.

Event: pre_tool_use
Runtime: pi
Runtime event: tool_call
Decision: blocked

Original tool call:
- Tool: edit
- Action: edit
- File: /path/to/foo.bar

Matched rules:

1. Exact foo.bar edit policy [instruction]
   Rule: rule_019ef27f-debc-7321-a4a5-ed3ffab5c85c
   Match: file: foo.bar
   Instructions:
   - Preserve the foo.bar key format.
   - Rule hash refresh marker: deterministic blocking should fire again after this update.

2. Stored instruction fallback for foo.bar writes [tool-trigger]
   Rule: rule_019ef23f-2070-73ed-ad7f-35aa07392ab9
   Match: file: foo.bar
   Trigger result:
   - BLOCKED: Run `gsc knowledge search foo.bar write policy` before overwriting foo.bar.
   - Notice: Blocked foo.bar write using the rule's stored instruction.

Required next steps:
- Apply all deterministic instructions above.
- Address all blocking trigger results above.
- Run any requested `gsc` commands, loading the required `gsc experts guide ...` first.
- Retry the original tool call only after satisfying the rule packet.
```

### How to Handle a Blocked Rule Message

1. **Read the full matched-rule packet.** Do not skip any matched rules.
2. **Apply every deterministic instruction.** These are repository policies that must be followed.
3. **Address every blocking trigger result.** These are runtime checks that must be satisfied.
4. **Run any requested `gsc` commands.** Load the relevant `gsc experts guide ...` before using that command category.
5. **Retry the original tool call** only after satisfying the rule packet.
6. **Do not bypass, disable, or ignore rules** unless the user explicitly instructs you to.

### Important Notes

- **Triggers do not hide instructions.** Even if a trigger blocks, all matched deterministic instructions are still included in the packet.
- **Multiple triggers can block.** All blocking trigger messages are included, not just the first.
- **Fail-open on errors.** If a trigger fails to execute, it does not block the tool call, but the error is reported.
- **Notices are user-facing.** Trigger notices are shown to the user but not included in the block message.

---

## 5. Validating Triggers

### Schema Validation

```bash
# Validate a specific rule
gsc rules trigger validate rule_abc123

# Validate all tool-trigger rules
gsc rules trigger validate --all
```

### With Fixture Context

```bash
# Validate with a fixture (for CI)
gsc rules trigger validate rule_abc123 --context .gitsense/rules/fixtures/edit-foo.json
```

### Validation Checks

- Rule schema is valid
- Trigger file exists
- Trigger runtime is supported (`node`)
- Trigger source parses
- Trigger exits within timeout
- stdout is one JSON object
- Result matches schema
- `block: true` has either returned `message` or stored instruction
- Frequency mode is known
- Referenced query/instruction exists if required

---

## 6. Running Triggers

### Single Trigger

```bash
gsc rules trigger run rule_abc123 --context context.json
```

### All Triggers

```bash
gsc rules trigger run --all --context context.json
```

### Aggregate Output

```json
{
  "schemaVersion": 1,
  "matched": [
    {
      "ruleId": "rule_abc",
      "block": true,
      "message": "Run `gsc knowledge search foo.bar edit policy` first.",
      "frequency": {
        "mode": "once-per-context",
        "key": "foo.bar"
      },
      "priority": 100,
      "ruleHash": "sha256:..."
    }
  ],
  "errors": []
}
```

---

## 6b. Executing Rules (gsc rules execute)

`gsc rules execute` is the primary command for agent integrations. It takes pre-queried rules and an execution context, runs triggers in parallel, and returns a complete ExecutionResult.

### Basic Usage

```bash
# Query rules and execute in one pipeline
gsc rules get --event pre_tool_use --action bash --command "rm -rf" --format rules-json | \
  gsc rules execute --context ctx.json --rules -

# With separate files
gsc rules get --file README.md --action edit --format rules-json > /tmp/rules.json
gsc rules execute --context ctx.json --rules /tmp/rules.json
```

### Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--context` | | required | V1ExecutionContext JSON file |
| `--rules` | | required | Rules JSON file (use `-` for stdin) |
| `--format` | `-o` | `json` | Output format |
| `--concurrency` | `-j` | `8` | Max parallel trigger executions |
| `--timeout` | | `0` (no limit) | Total execution budget (e.g., `10s`, `500ms`) |

### ExecutionResult Output

```json
{
  "schemaVersion": 1,
  "block": true,
  "reason": "GitSense matched repository rules before this lifecycle event.\n\nEvent: pre_tool_use\n...",
  "notices": ["Notice 1", "Notice 2"],
  "matchedRules": [
    {
      "ruleId": "rule_001",
      "ruleHash": "sha256:...",
      "type": "declarative",
      "summary": "Block rm -rf commands",
      "instructions": ["Do not run rm -rf commands"],
      "match": { "kind": "command", "value": "rm -rf" }
    },
    {
      "ruleId": "rule_002",
      "ruleHash": "sha256:...",
      "triggerHash": "sha256:...",
      "type": "executable",
      "summary": "Check command safety",
      "match": { "kind": "command", "value": "rm -rf" }
    }
  ],
  "triggerResults": [
    {
      "ruleId": "rule_002",
      "matched": true,
      "block": false,
      "notice": "Command is safe"
    }
  ],
  "errors": [],
  "subagentTasks": []
}
```

### Event-Specific Behavior

| Event | Declarative Rules | Executable Triggers |
| :--- | :--- | :--- |
| `pre_tool_use` | Block with matched-rule packet | Block/allow based on trigger logic |
| `user_prompt_submit` | Advisory (injected as context) | Block/allow based on trigger logic |
| `agent_end` | Advisory (injected as context) | Send messages/notices |
| `session_start` | Advisory (injected as context) | Send messages/notices |
| Other events | Advisory (injected as context) | Block/allow based on trigger logic |

### Capabilities Enforcement

If `canBlock=false` in the execution context:
- Trigger results are captured
- Block is forced to `false`
- Notice is added: "Block ignored: canBlock=false in context"

### Exit Codes

- `0` — Evaluation completed successfully (block true/false is in JSON)
- `1` — Invalid input, runtime failure, or internal error

### Example: Full Pipeline

```bash
# Create context file
cat > /tmp/ctx.json << 'EOF'
{
  "version": "1",
  "event": { "name": "pre_tool_use", "runtime": "pi" },
  "capabilities": { "canBlock": true },
  "session": { "id": "test", "path": "/tmp/test.jsonl", "cwd": "/repo" },
  "conversation": { "leafId": "entry-1", "messageIds": ["entry-1"] },
  "payload": {
    "toolCall": {
      "id": "call-1",
      "toolName": "bash",
      "action": "bash",
      "command": "rm -rf /tmp/test",
      "input": { "command": "rm -rf /tmp/test" }
    }
  },
  "repo": { "root": "/repo" }
}
EOF

# Query and execute
gsc rules get --event pre_tool_use --action bash --command "rm -rf" --format rules-json | \
  gsc rules execute --context /tmp/ctx.json --rules - --timeout 10s
```

---

## 7. Testing Rules Against Sessions

### Replay Testing

Test whether a static instruction rule is surgical enough by replaying tool calls from a Pi session JSONL file.

```bash
# Test a rule against a session (uses latest leaf)
gsc rules test <rule-id> --session /path/to/session.jsonl --format json

# Test with explicit leaf
gsc rules test <rule-id> --session /path/to/session.jsonl --leaf entry-123 --format json
```

### What It Does

1. Loads the rule by ID
2. Verifies it's an instruction rule (not tool-trigger)
3. Extracts tool calls from the active branch
4. Evaluates each file operation (read/edit/write) against the rule
5. Reports matches and non-matches with structured provenance

### Output Format

```json
{
  "rule": {
    "id": "rule_abc",
    "summary": "Accounting record format guidance",
    "type": "instruction",
    "ruleHash": "sha256:..."
  },
  "session": {
    "path": "/abs/session.jsonl",
    "leaf": "entry_leaf",
    "latestLeafUsed": true
  },
  "evaluated": 8,
  "matched": [
    {
      "toolCallId": "call_1",
      "toolName": "read",
      "action": "read",
      "path": "data/accounting/q1.ledger",
      "match": {
        "kind": "glob",
        "value": "**/accounting/**",
        "file": "data/accounting/q1.ledger",
        "action": "read"
      },
      "instructions": ["These files use the custom ledger format..."],
      "wouldBlock": true,
      "deliveryKeyPreview": "static-rule:rule_abc:sha256:...:read:glob:**/accounting/**:<context>"
    }
  ],
  "notMatched": [
    {
      "toolCallId": "call_2",
      "toolName": "read",
      "action": "read",
      "path": "README.md",
      "reason": "file/action did not match rule"
    }
  ]
}
```

### Key Fields

| Field | Description |
| :--- | :--- |
| `wouldBlock` | Whether pi-brains would deliver this rule (ignoring frequency state) |
| `deliveryKeyPreview` | Illustrative delivery key for frequency tracking |
| `match` | Structured match provenance (kind, value, file, action) |
| `ruleHash` | Canonical semantic hash of the rule |

### Limitations

- Only evaluates instruction rules, not tool-trigger rules
- Only considers read/edit/write tool calls (not bash)
- Does not apply frequency state
- V1: JSON output only

---

## 8. Querying Rules

### Before Editing a File

```bash
# Query by file path
gsc rules get --file internal/cli/root.go

# Query by file path when editing (filters by action)
gsc rules get --file internal/cli/root.go --action edit

# Query by glob pattern
gsc rules get --glob "internal/cli/**"

# Query by tag
gsc rules get --tag formatting

# JSON output for agents
gsc rules get --file internal/cli/root.go --format json
```

### List by Type

```bash
# List only instruction rules across repo and personal scopes
gsc rules list --type instruction

# List only repo-scoped tool-trigger rules
gsc rules list --type tool-trigger --scope repo
```

### Output Format

```json
{
  "query": {
    "file": "internal/cli/root.go"
  },
  "git_root": "/path/to/repo",
  "rules": [
    {
      "rule": {
        "id": "rule_019e...",
        "summary": "CLI file conventions",
        "instructions": [
          "Do not run gofmt -w",
          "Bump the Version field"
        ],
        "actions": ["edit", "write"],
        "importance": "high"
      },
      "match_reason": "glob: internal/cli/**",
      "match": {
        "kind": "glob",
        "value": "internal/cli/**",
        "file": "internal/cli/root.go",
        "action": "edit"
      }
    }
  ],
  "summary": {
    "rules_matched": 1,
    "high": 1,
    "medium": 0,
    "low": 0
  }
}
```

### Match Provenance

Each matched rule includes a `match` object with structured provenance for frequency tracking:

| Field | Description |
| :--- | :--- |
| `match.kind` | Match type: `file`, `glob`, `tag`, `topic`, `command`, `unknown` |
| `match.value` | The anchor that caused the match (exact path, glob pattern, tag slug, etc.) |
| `match.file` | The queried file (when applicable) |
| `match.action` | The queried action (when `--action` is provided) |

This enables pi-brains to compute delivery keys:
```
static-rule:<ruleId>:<ruleHash>:<action>:<match.kind>:<match.value>:<contextEpoch>
```

**Selection priority:**
- Exact file match > glob match
- More specific glob > broader glob (by path segment count and wildcard type)
- Ties broken by lexical order

---

## 9. Managing Rules

### Update a Rule

```bash
# Update specific fields
gsc rules update --id <id> --summary "New summary" --changelog "Updated summary"

# Update instructions
gsc rules update --id <id> \
  --instruction "New instruction 1" \
  --instruction "New instruction 2" \
  --action edit --action write \
  --changelog "Updated instructions"

# Update from file
gsc rules update --id <id> --from-file /tmp/rule.json --changelog "Updated from file"
```

### Delete a Rule

```bash
gsc rules delete <id>
```

### List and Search

```bash
# List all rules from repo and personal scopes
gsc rules list

# List personal rules only
gsc rules list --scope personal

# Filter by tag
gsc rules list --tag formatting

# Filter by importance
gsc rules list --importance high

# Filter by type
gsc rules list --type tool-trigger

# Full-text search
gsc rules search "gofmt"

# Show a repo rule by ID or unique prefix
gsc rules show <id> --scope repo
```

---

## 10. Instruction Rule Schema

### Required Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `summary` | string | Short description (≤240 chars) |
| `instructions` | array | At least one instruction (simple strings) |
| `actions` | array | At least one action: `read`, `write`, `edit` |
| `glob_patterns` | array | At least one anchor (glob, file, command, topic, or tag) |

### Optional Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `details` | string | Longer description (≤4000 chars) |
| `importance` | string | `low`, `medium`, or `high` (default: `medium`) |
| `owner` | string | Who owns this rule |
| `contact` | array | Who to contact when matched |
| `exclude_globs` | array | Exclusion patterns |
| `tags` | array | Categorization tags |
| `applies_to.files` | array | Specific file paths |
| `applies_to.linked_files` | array | Related files |
| `applies_to.commands` | array | Related commands |
| `applies_to.topics` | array | Topic slugs |

### Actions

Actions specify when a rule applies:

| Action | When |
| :--- | :--- |
| `read` | When reading a file |
| `write` | When writing a file |
| `edit` | When editing a file |

Example: `"actions": ["edit", "write"]` means the rule applies when editing or writing, but not when reading.

---

## 11. Tool-Trigger Rule Schema

### Required Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `type` | string | Must be `"tool-trigger"` |
| `summary` | string | Short description (≤240 chars) |
| `trigger` | object | Trigger configuration |
| `trigger.runtime` | string | Must be `"node"` |
| `trigger.entry` | string | Path to trigger file (relative to `.gitsense/rules/triggers/`) |
| `instruction` | object | Instruction configuration |
| `instruction.mode` | string | `"inline"` or `"query"` |
| `instruction.text` | string | Required if mode is `"inline"` |
| `instruction.query` | string | Required if mode is `"query"` |
| `frequency` | object | Frequency configuration |
| `frequency.mode` | string | One of: `always`, `once-per-turn`, `once-per-context`, `once-per-session`, `once-per-branch`, `once-per-file` |

### Optional Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `trigger.timeoutMs` | number | Timeout in milliseconds (default: 5000) |
| `frequency.key` | string | Optional key for scoping (e.g., file path) |
| `priority` | number | Higher = executed first (default: 0) |
| `enabled` | boolean | Default: `true` |
| `details` | string | Longer description (≤4000 chars) |
| `importance` | string | `low`, `medium`, or `high` (default: `medium`) |
| `tags` | array | Categorization tags |

---

## 12. Agent Integration Pattern

### Session Start

When `gsc experts init` is used:

1. Agent reads the expert context
2. Agent knows rules are available for enforcement
3. Agent queries rules before edits/writes using `--action` flag
4. Instruction rules are advisory
5. Tool-trigger rules are evaluated by pi-brains or `gsc rules execute`

### Before Each Edit (Instruction Rules)

```bash
gsc rules get --file <file-path> --action edit --format json
```

If rules are found, the agent should:
1. Display the matching rules
2. Follow the instructions (unless they conflict with user intent)
3. Not treat instructions as executable commands

### Tool-Trigger Integration (pi-brains)

Tool-trigger rules are evaluated automatically by pi-brains:

```
Pi emits tool_call
  -> pi-brains builds trigger context
  -> pi-brains queries: gsc rules get --event pre_tool_use --format rules-json
  -> pi-brains executes: gsc rules execute --context <tmp.json> --rules -
  -> gsc returns ExecutionResult with matched rules and trigger results
  -> pi-brains applies frequency state
  -> if result.blocks:
       block tool call with result.reason
     else:
       allow tool call
```

### Pipeline Composition

For direct execution without pi-brains:

```bash
# Query and execute in one pipeline
gsc rules get --event pre_tool_use --action bash --command "rm -rf" --format rules-json | \
  gsc rules execute --context ctx.json --rules -

# With custom concurrency and timeout
gsc rules get --file README.md --action edit --format rules-json | \
  gsc rules execute --context ctx.json --rules - -j 4 --timeout 10s
```

### gsc rules execute vs gsc rules trigger run

| Aspect | `gsc rules execute` | `gsc rules trigger run` |
| :--- | :--- | :--- |
| Input | Pre-queried rules (rules-json) | Rule ID or --all |
| Parallelism | Configurable (-j flag) | Sequential |
| Timeout | Global --timeout budget | Per-trigger only |
| Use case | Agent integration, pipelines | Single trigger testing |

### Injected Message Format

When a tool-trigger blocks, the agent receives:

```
Repository knowledge trigger matched before this tool call.

Rule: Require foo.bar edit guidance
Instruction:
Run `gsc knowledge search foo.bar edit policy` before editing foo.bar.

After doing that, retry the original tool call if still appropriate.
```

---

## 13. Security Model

### Repository-Owned Policy

- Tool triggers are **repository-owned executable policy**
- They run locally when an agent integration chooses to evaluate them
- They should be **reviewed like build scripts, test scripts, or Git hooks**
- `gsc` validates the contract, not the intent
- Integrations should run with timeout/output caps

### Fail-Open Default

V1 is **fail-open** by default:
- Trigger timeout → allow with diagnostics
- Trigger crash → allow with diagnostics
- Invalid output → allow with diagnostics

This ensures agents can always make progress, even if triggers fail.

### Reviewability

Triggers are designed for review:
- Trigger files are committed to the repository
- `gsc rules trigger validate` checks schema and execution
- Fixtures enable CI testing: `gsc rules trigger validate --all --context fixture.json`

---

## 14. Glob Pattern Syntax

| Pattern | Matches |
| :--- | :--- |
| `internal/cli/**` | All files under internal/cli/ |
| `**/*.go` | All Go files |
| `internal/cli/*.go` | Go files directly in internal/cli/ |
| `!**/*_test.go` | Exclude test files |

### Exclusion Precedence

Exclusion patterns override inclusion patterns within the same rule:

```json
{
  "glob_patterns": ["internal/cli/**"],
  "exclude_globs": ["internal/cli/legacy/**"]
}
```

This matches everything in `internal/cli/` except `internal/cli/legacy/`.

---

## 15. Best Practices

### Instruction Rules

1. **Use specific globs** — Avoid `**` when possible; prefer `internal/cli/**`
2. **Set actions** — Specify when the rule applies (read, write, edit)
3. **Set importance** — Helps agents prioritize conflicting rules
4. **Add owner/contact** — Knows who to ask about the rule
5. **Keep instructions actionable** — "Do X" not "X should be done"
6. **Use exclusion patterns** — More precise than broad globs

### Tool-Trigger Rules

1. **Keep triggers simple** — They should recognize situations, not contain knowledge
2. **Store knowledge in GitSense** — Use lessons, notes, or rules for the "what"
3. **Use appropriate frequency** — `once-per-context` is usually sufficient
4. **Set priorities** — Higher priority triggers run first
5. **Test with fixtures** — Validate triggers in CI with `--context`
6. **Use query mode** — For dynamic knowledge, use `instruction.mode: query`

---

## 16. Common Mistakes

| Mistake | Correction |
| :--- | :--- |
| Forgetting to rebuild Brain | `gsc rules new/delete/update` auto-rebuilds |
| Using absolute paths in globs | Use repo-relative paths |
| Treating instructions as commands | Instructions are advisory, not executable |
| Not specifying actions | Always specify when the rule applies |
| Using old instruction format | Instructions are now simple strings, not objects |
| Embedding knowledge in trigger code | Store knowledge in GitSense, use triggers for "when" |
| Using `always` frequency unnecessarily | Use `once-per-context` or `once-per-file` to reduce noise |
| Not validating triggers | Run `gsc rules trigger validate` before committing |

---

## 17. Examples

### Example 1: File-Specific Edit Guidance

```bash
# Create trigger
gsc rules trigger new \
  --title "README edit guidance" \
  --tool edit \
  --file README.md \
  --instruction "Verify command tables match gsc --help output" \
  --topic documentation

# Result:
# - Rule in records.jsonl
# - Trigger in .gitsense/rules/triggers/readme-edit-guidance.mjs
```

### Example 2: Glob-Based Knowledge Query

```bash
# Create trigger with knowledge query
gsc rules trigger new \
  --title "API conventions" \
  --tool edit \
  --glob "api/**" \
  --query "gsc knowledge search api conventions" \
  --topic api

# When triggered, runs the query and injects the result
```

### Example 3: High-Priority Safety Rule

```bash
# Create safety trigger
gsc rules trigger new \
  --title "Protected files" \
  --tool write \
  --file config/production.yml \
  --instruction "DO NOT modify production config without approval" \
  --frequency always \
  --priority 1000 \
  --topic safety
```

### Example 4: Custom Trigger Logic (V1 Contract)

```javascript
// .gitsense/rules/triggers/complex-rule.mjs
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

// V1 context fields:
// - ctx.version = "1"
// - ctx.toolCall.action = "read" | "edit" | "write" | "command" | "custom"
// - ctx.toolCall.toolName = "read" | "edit" | "write" | "bash"
// - ctx.toolCall.file = absolute path (null for bash)
// - ctx.toolCall.command = raw command (null for file tools)
// - ctx.repo?.normalizedFile = repo-relative path

const isEdit = ctx.toolCall?.action === 'edit';
const isProtectedFile = ctx.repo?.normalizedFile?.startsWith('config/production.');
const applies = isEdit && isProtectedFile;

console.log(JSON.stringify({
    matched: applies,
    block: applies,
    message: applies
        ? "DO NOT modify production config without approval."
        : undefined,
    notice: applies
        ? "Blocked: production config requires approval."
        : undefined
}));
```

### Example 5: Non-Blocking Diagnostic (matched=false)

```javascript
// Trigger that logs diagnostics without blocking
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

// Check if this is a bash command with potential issues
const isCommand = ctx.toolCall?.action === 'command';
const hasRiskyCommand = ctx.toolCall?.command?.includes('rm -rf');
const applies = isCommand && hasRiskyCommand;

console.log(JSON.stringify({
    matched: applies,
    block: false, // Don't block, just log
    notice: applies
        ? "Warning: rm -rf detected in command."
        : undefined
}));
```
