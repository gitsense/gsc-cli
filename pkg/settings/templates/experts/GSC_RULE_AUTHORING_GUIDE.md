<!--
Component: GSC Rule Authoring Guide
Block-UUID: (generated)
Parent-UUID: N/A
Version: 1.0.0
Description: On-demand reference guide for safely authoring rules and triggers. Covers decision framework, scope/target selection, safety checklist, and post-write reporting contract.
Language: Markdown (Go Template)
Created-at: 2026-06-27T00:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0)
-->

# Rule & Trigger Authoring Guide

This guide helps agents safely decide when to create rules/triggers, how to scope them, and how to explain the resulting behavior to users.

---

## Agent-Created Rules

When an AI agent creates or updates rules, it MUST use the agent lane:

```bash
gsc rules new --creator agent --target <repo|personal> --from-file rule.json
gsc rules trigger new --creator agent --target <repo|personal> --from-file trigger.json
gsc rules trigger new --creator agent --target <repo|personal> --stdin < trigger.json
gsc rules update --creator agent --target <repo|personal> --id <id> --from-file rule.json --changelog "..."
```

Human users may omit `--creator`. Agents must not omit it.

For `--creator agent`, the JSON must include `creatorChecklist`. The command rejects the write if:
- `creatorChecklist.unresolved` is not empty
- `creatorChecklist.scope` does not match `--target`
- `creatorChecklist.topic` does not match the rule topic or lacks verification
- `creatorChecklist.matching` omits any rule action, file, or glob
- `creatorChecklist.verification.syntaxVerifiedFrom` is missing
- executable triggers omit side effects, validation plan, or lifecycle verification
- executable triggers lack explicit confirmation (regardless of stated risk level)
- blocking rules lack explicit confirmation

**Important:** All executable triggers are treated as high-risk by validation, even if `risk.level` is set to "low" or "medium". This is because executable triggers run code. If the rule is executable, `confirmation.required`, `confirmation.userConfirmed`, and `confirmation.confirmedText` must all be set.

Scope selection alone is not confirmation. If a confirmation prompt asked for both scope and confirmation, and the user answered only scope, stop and ask for the missing confirmation.

### Required Preflight

Before writing any rule or trigger file:

1. Pick scope: `repo` or `personal`.
2. Pick topic:
   - Prefer an existing topic from `gsc topics list`.
   - If no topic fits, create one with exactly `gsc topics add <slug> --description "..."`.
   - Do not pass `--target` to `gsc topics add`; topics are shared registry entries.
3. Pick lifecycle event:
   - `pre_tool_use` means before the tool call runs.
   - `post_tool_use` means after the tool call runs and its result can be checked.
   - For "after editing .ts files, verify compilation", use `post_tool_use`.
4. Build the JSON with a complete `creatorChecklist`.
5. For executable triggers, create/verify the trigger entry file, then run `gsc rules trigger new --creator agent ...`.
6. After creation, run the validation command from `creatorChecklist.verification.validationPlan`.

### Minimal Checklist Shape

```json
{
  "creatorChecklist": {
    "creator": "agent",
    "intent": "What future agent behavior should change.",
    "scope": "personal",
    "ruleKind": "declarative",
    "topic": {
      "slug": "agent-lifecycle-rules",
      "source": "existing",
      "verifiedFrom": "gsc topics list"
    },
    "matching": {
      "event": "pre_tool_use",
      "actions": ["edit"],
      "globs": ["**/*.ts", "**/*.tsx"]
    },
    "delivery": {
      "mode": "steer",
      "blocks": true,
      "messageShownToAgent": "Exact message shown to the agent."
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

Before writing a rule, present a concise checklist to the user. If everything is clear and low-risk, proceed. If the rule is executable, blocking, repo-scoped, broad, or ambiguous, ask for explicit confirmation after showing the checklist.

---

## Decision Framework

Use this ladder to choose the right knowledge type:

```text
Use a NOTE when the information is passive context.
  Example: "This module uses the XYZ pattern for error handling."

Use a LESSON when the information is learned guidance from prior work.
  Example: "When refactoring this module, we learned to avoid ABC because it caused DEF."

Use a RULE when future agents should actively change behavior.
  Example: "Do not run gofmt -w on files in internal/cli/."

Use a TRIGGER only when behavior must execute code automatically.
  Example: "Before editing config/production.yml, verify the change has approval."
```

### When to Use Each Type

| Type | Use When | Example |
| :--- | :--- | :--- |
| **Note** | Passive context agents can query | "This API uses snake_case for field names" |
| **Lesson** | Learned guidance from prior work | "We tried X but it caused Y; use Z instead" |
| **Rule** | Active behavior change required | "Do not run rm -rf without confirmation" |
| **Trigger** | Code must evaluate runtime context | "Check if user approved before editing prod config" |

### Anti-Patterns

| Don't | Use Instead |
| :--- | :--- |
| Create a rule for passive information | Use a note |
| Create a trigger for a static reminder | Use a declarative rule |
| Create a rule for one-time guidance | Use a lesson |
| Create a trigger when a rule suffices | Use a declarative rule with instructions |

---

## Scope Selection: Repo vs Personal

Before creating any rule, note, or lesson, ask:

```text
Should this be saved to repo scope or personal scope?
```

### Repo Scope (`--target repo`)

Use for:
- Project-specific conventions
- Team-shared policies
- Repository coding standards
- File-specific edit guidance
- Build/test/lint requirements

Examples:
- "Do not run gofmt -w on internal/cli/ files"
- "Always bump the Version field in component headers"
- "Block rm -rf commands in this repository"

### Personal Scope (`--target personal`)

Use for:
- User preferences across repositories
- Individual workflow habits
- Cross-repo standing instructions
- Personal productivity rules

Examples:
- "Always use --verbose flag when running tests"
- "Prefer table output format for gsc commands"
- "Log all bash commands for personal audit"

### Scope Rules

| Rule | Repo | Personal |
| :--- | :--- | :--- |
| Project conventions | ✅ | ❌ |
| Team policies | ✅ | ❌ |
| User preferences | ❌ | ✅ |
| Cross-repo habits | ❌ | ✅ |
| Private credentials | ❌ | ✅ |
| Repository paths | ✅ | ❌ |

**Never:**
- Create personal rules that encode repo-specific paths
- Create repo rules that encode private user preferences

---

## Read Scope: Default Behavior

When querying rules, the default scope is `all` (repo + personal):

```bash
# Queries both repo and personal rules
gsc rules get --file foo.go

# Explicit scope
gsc rules get --file foo.go --scope repo
gsc notes search "api" --scope personal
```

This means:
- Users always see the combined view by default
- Personal rules overlay repo rules
- Source is tracked in JSON output

---

## Rule Safety Checklist

Before creating a rule, verify:

### Matching Conditions

Prefer narrow matching:
- `--glob "internal/cli/**"` over `--glob "**"`
- `--action edit` over all actions
- `--event pre_tool_use` over all events
- `--tag safety` for categorization
- `--tool github.*` for specific tools
- `--matches "rm -rf"` for specific commands

Avoid:
- Broad "always" rules without confirmation
- `--glob "**"` combined with high importance
- No action/event filter on blocking rules

### Rule Type Selection

Prefer this order:
1. **Declarative instruction** — Advisory, no code execution
2. **Executable trigger** — Only when code must evaluate context

Ask before creating a trigger:

```text
This will create an executable trigger that runs code automatically.
Confirm the target scope, runtime, entry path, and intended side effects.
```

### Importance Level

| Level | Use When |
| :--- | :--- |
| `low` | Nice-to-have guidance |
| `medium` | Standard conventions (default) |
| `high` | Safety-critical, must-follow |

### Summary Quality

Good summaries describe behavior and blast radius:
- ✅ "Block rm -rf commands in internal/cli/"
- ✅ "Require approval before editing production config"
- ❌ "Safety rule"
- ❌ "Important"

---

## Trigger Safety Requirements

Before creating a trigger, state:

```text
This trigger will execute code.
- Runtime: [node|python|bash]
- Entry: [path to trigger file]
- When it runs: [lifecycle event and conditions]
- What it may touch: [files, network, system]
- How to disable: gsc rules delete <id> --target <repo|personal>
```

### Trigger Checklist

- [ ] Runtime is supported (node, python, bash)
- [ ] Entry path is relative (no leading `/`)
- [ ] Entry path has no `..` traversal
- [ ] Trigger file lives under correct target's `rules/triggers/` (see below)
- [ ] Side effects are documented
- [ ] Rollback command is provided
- [ ] Timeout is reasonable (default 5000ms)
- [ ] Frequency is appropriate (not `always` unless necessary)

### Trigger File Locations

Trigger files must be placed in the correct directory based on scope:

| Scope | Trigger Directory |
| :--- | :--- |
| `repo` | `.gitsense/rules/triggers/` (in the repository) |
| `personal` | `~/.gitsense/rules/triggers/` (in user home, or `$GSC_HOME/rules/triggers/`) |

When creating a trigger with `--target personal`, the trigger file must exist in the personal triggers directory before running `gsc rules trigger new`. The error message will show the expected path if the file is missing.

Example workflow for personal triggers:
```bash
# 1. Create the trigger file in the personal triggers directory
mkdir -p ~/.gitsense/rules/triggers
cat > ~/.gitsense/rules/triggers/my-trigger.mjs << 'EOF'
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));
console.log(JSON.stringify({ matched: false, block: false }));
EOF

# 2. Create the rule
gsc rules trigger new --creator agent --target personal --from-file trigger.json
```

### Trigger Confirmation Prompt

Before creating a trigger, the agent should ask:

```text
This trigger will execute code automatically when [condition].
- Runtime: [runtime]
- Entry: [entry path]
- Target: [repo|personal]
- Side effects: [what it may touch]

Confirm creation? (y/n)
```

### What Triggers Can Do

| Capability | Declarative Rule | Executable Trigger |
| :--- | :--- | :--- |
| Block actions | ✅ (via pi-brains) | ✅ |
| Inject instructions | ✅ | ✅ |
| Run code | ❌ | ✅ |
| Query external systems | ❌ | ✅ |
| Modify files | ❌ | ⚠️ (with caution) |
| Network access | ❌ | ⚠️ (with caution) |

---

## Creating Rules Safely

### Step 1: Determine Knowledge Type

Ask: "Does this require active behavior change, or is it passive context?"

- Passive → note
- Learned guidance → lesson
- Active behavior → rule
- Code evaluation → trigger

### Step 2: Determine Scope

Ask: "Is this project-specific or user-specific?"

- Project/team → `--target repo`
- User preference → `--target personal`

### Step 3: Narrow Matching

Ask: "What's the narrowest condition that captures this rule?"

```bash
# Good: narrow
gsc rules new --target repo \
  --glob "internal/cli/**" \
  --action edit \
  --summary "CLI file conventions" \
  --instruction "Do not run gofmt -w"

# Bad: too broad
gsc rules new --target repo \
  --glob "**" \
  --summary "Safety rule" \
  --instruction "Be careful"
```

### Step 4: Choose Rule Type

Prefer declarative over executable:

```bash
# Declarative (preferred)
gsc rules new --target repo \
  --glob "config/production.yml" \
  --action edit \
  --summary "Production config protection" \
  --instruction "Do not modify without approval"

# Only if code evaluation is needed
gsc rules trigger new --target repo \
  --title "Production config approval check" \
  --runtime node \
  --entry check-approval.mjs \
  --event pre_tool_use
```

### Step 5: Verify and Report

After creating a rule, report:

```text
Scope: repo|personal
Rule ID: <id>
Applies to: <files/actions/events/tools>
Behavior: <short behavior summary>
Executable trigger: yes|no
Disable: gsc rules delete <id> --target <repo|personal>
```

---

## Creating Notes Safely

```bash
# Create a note with narrow scope
gsc notes add --target repo \
  --glob "internal/cli/**" \
  --summary "CLI architecture notes" \
  --content "The CLI uses cobra for command handling..."

# Create a personal note
gsc notes add --target personal \
  --summary "My workflow preferences" \
  --content "I prefer table output format"
```

After creating a note, report:

```text
Scope: repo|personal
Note ID: <id>
Applies to: <files/topics>
Summary: <summary>
Disable: gsc notes delete <id> --target <repo|personal>
```

---

## Creating Lessons Safely

```bash
# Stage a lesson with narrow scope, then commit it to repo scope
gsc lessons add \
  --summary "Refactoring lesson" \
  --details "When refactoring module X, we learned to avoid Y because it caused Z." \
  --file internal/module.go \
  --tag refactoring
gsc lessons draft commit --target repo

# Stage a personal lesson, then commit it to personal scope
gsc lessons add \
  --summary "My workflow preference" \
  --details "I prefer table output format for gsc commands"
gsc lessons draft commit --target personal
```

After creating a lesson, report:

```text
Scope: repo|personal
Lesson ID: <id>
Applies to: <files/topics>
Summary: <summary>
Disable: gsc lessons delete <id> --target <repo|personal>
```

---

## Updating and Deleting

### Update a Rule

```bash
# Update with changelog
gsc rules update --target repo --id <id> \
  --summary "Updated summary" \
  --changelog "Updated for clarity"

# Update personal rule
gsc rules update --target personal --id <id> \
  --instruction "New instruction" \
  --changelog "Added new instruction"
```

### Delete a Rule

```bash
# Delete from repo
gsc rules delete <id> --target repo

# Delete from personal
gsc rules delete <id> --target personal
```

### Show Where a Rule Lives

```bash
# Query with source info
gsc rules get --file foo.go --format json | jq '.rules[] | {source, id, summary}'

# Query personal rules only
gsc rules get --scope personal --tag safety

# Query repo rules only
gsc rules get --scope repo --tag safety
```

---

## Post-Write Reporting Contract

After creating or updating any rule, trigger, note, or lesson, the agent MUST report:

### For Rules

```text
✅ Rule created in <scope> scope.

  Rule ID: <id>
  Summary: <summary>
  Applies to: <glob/files/actions/events>
  Importance: <importance>
  Type: <declarative|executable>
  
  To modify: gsc rules update --target <scope> --id <id>
  To delete: gsc rules delete <id> --target <scope>
```

### For Triggers

```text
✅ Trigger created in <scope> scope.

  Rule ID: <id>
  Summary: <summary>
  Runtime: <runtime>
  Entry: <entry path>
  Event: <lifecycle event>
  Frequency: <frequency mode>
  
  Trigger file: <path to trigger file>
  
  ⚠️ This trigger executes code automatically.
  
  To modify: gsc rules update --target <scope> --id <id>
  To delete: gsc rules delete <id> --target <scope>
  To disable: gsc rules update --target <scope> --id <id> --enabled false
```

### For Notes

```text
✅ Note created in <scope> scope.

  Note ID: <id>
  Summary: <summary>
  Applies to: <glob/files/topics>
  
  To modify: gsc notes update --target <scope> --id <id>
  To delete: gsc notes delete <id> --target <scope>
```

---

## Common Mistakes

| Mistake | Why It's Bad | Fix |
| :--- | :--- | :--- |
| Using `--target repo` for personal preferences | Pollutes shared repo | Use `--target personal` |
| Using `--target personal` for project conventions | Not shared with team | Use `--target repo` |
| Broad `--glob "**"` without confirmation | Too wide blast radius | Narrow the glob |
| Creating trigger for static reminder | Unnecessary code execution | Use declarative rule |
| No `--changelog` on update | Loses change history | Always add changelog |
| Forgetting `--target` on write | Command fails | Always specify target |
| Not reporting after creation | User doesn't know where rule landed | Use post-write contract |
| Setting `confirmation.required: false` for executable triggers | Validation rejects: all executable triggers require confirmation | Always set `confirmation.required: true` for executable triggers |
| Placing personal trigger files in repo `.gitsense/rules/triggers/` | Validation fails: personal triggers must be in `~/.gitsense/rules/triggers/` | Use the correct directory for the target scope |

---

## Quick Reference

### Write Commands (Require `--target`)

```bash
gsc rules new --target <repo|personal> ...
gsc rules update --target <repo|personal> --id <id> ...
gsc rules delete <id> --target <repo|personal>
gsc rules build --target <repo|personal>
gsc rules trigger new --target <repo|personal> ...

gsc notes add --target <repo|personal> ...
gsc notes update --target <repo|personal> --id <id> ...
gsc notes delete <id> --target <repo|personal>
gsc notes build --target <repo|personal>

gsc lessons add ...
gsc lessons draft commit --target <repo|personal>
gsc lessons update --target <repo|personal> --id <id> ...
gsc lessons delete <id> --target <repo|personal>
gsc lessons build --target <repo|personal>
```

### Read Commands (Default `--scope all`)

All rules, notes, and lessons read/discovery commands support `--scope`.
Leave it unset for `all`; set it only when the user or task needs repo-only
or personal-only knowledge.

```bash
gsc rules get --file <path> [--scope <all|repo|personal>]
gsc rules list [--scope <all|repo|personal>]
gsc rules search <query> [--scope <all|repo|personal>]
gsc notes get --file <path> [--scope <all|repo|personal>]
gsc notes search <query> [--scope <all|repo|personal>]
gsc notes tags [--scope <all|repo|personal>]
gsc lessons list [--scope <all|repo|personal>]
gsc lessons search <query> [--scope <all|repo|personal>]
gsc lessons show <id> [--scope <all|repo|personal>]
```

### Scope Summary

| Operation | Flag | Default | Values |
| :--- | :--- | :--- | :--- |
| Read | `--scope` | `all` | `all`, `repo`, `personal` |
| Write | `--target` | (required) | `repo`, `personal` |
