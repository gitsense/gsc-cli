<!--
Component: GSC Notes Guide
Block-UUID: d2e3f4a5-b6c7-8901-defa-901234567890
Parent-UUID: N/A
Version: 2.0.0
Description: On-demand reference guide for gsc notes commands. Covers note creation, querying, and management for searchable scratchpad notes. Topics are required.
Language: Markdown (Go Template)
Created-at: 2026-06-21T00:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
-->

# GSC Notes Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc notes add --target <repo\|personal>` | Create a note from flags, file, or stdin |
| `gsc notes add --template` | Print a note template and exit |
| `gsc notes get --file <path> [--scope <all\|repo\|personal>]` | Query notes for a specific file |
| `gsc notes get --glob <pattern> [--scope <all\|repo\|personal>]` | Query notes matching a glob pattern |
| `gsc notes get --tag <tag> [--scope <all\|repo\|personal>]` | Query notes by tag |
| `gsc notes update --target <repo\|personal> --id <id>` | Update an existing note |
| `gsc notes delete <id> --target <repo\|personal>` | Delete a note |
| `gsc notes list [--scope <all\|repo\|personal>]` | List all notes |
| `gsc notes show <id> [--scope <all\|repo\|personal>]` | Show a note in detail |
| `gsc notes search <query> [--scope <all\|repo\|personal>]` | Full-text search |
| `gsc notes tags [--scope <all\|repo\|personal>]` | List note tags with counts |
| `gsc notes overview [--scope <all\|repo\|personal>]` | Summary digest for notes |
| `gsc notes build --target <repo\|personal>` | Rebuild the gsc-notes Brain for repo target, or manifest for personal target |

---

## 1. Creating Notes

### From Flags

```bash
# Topic must be registered first
gsc topics add cli-architecture --description "CLI architecture and design patterns"

# Create a repo-scoped note
gsc notes add --target repo --glob "internal/cli/**" \
  --topic cli-architecture \
  --summary "CLI architecture notes" \
  --content "The CLI uses cobra for command handling. Each command is in its own file under internal/cli/." \
  --importance medium \
  --tag architecture

# Create a personal note
gsc notes add --target personal \
  --topic workflow \
  --summary "My workflow preferences" \
  --content "I prefer table output format for gsc commands"
```

### From a Template

```bash
# Print template
gsc notes add --template > /tmp/note.json

# Edit the template (ensure topic is included)
vim /tmp/note.json

# Create the note
gsc notes add --from-file /tmp/note.json
```

### From Stdin

```bash
cat note.json | gsc notes add --stdin
```

---

## 2. Querying Notes (Agent-Facing)

### Before Working on a File

```bash
# Query by file path
gsc notes get --file internal/cli/root.go

# Query by glob pattern
gsc notes get --glob "internal/cli/**"

# Query by tag
gsc notes get --tag architecture

# JSON output for agents
gsc notes get --file internal/cli/root.go --format json
```

### Output Format

```json
{
  "query": {
    "file": "internal/cli/root.go"
  },
  "scope": "all",
  "sources": ["repo", "personal"],
  "git_root": "/path/to/repo",
  "notes": [
    {
      "source": "repo",
      "note": {
        "id": "note_019e...",
        "summary": "CLI architecture notes",
        "content": "The CLI uses cobra for command handling...",
        "topic": "cli-architecture",
        "importance": "medium"
      },
      "match_reason": "glob: internal/cli/**"
    }
  ],
  "summary": {
    "notes_matched": 1,
    "high": 0,
    "medium": 1,
    "low": 0
  }
}
```

---

## 3. Managing Notes

### Update a Note

```bash
# Update specific fields
gsc notes update --id <id> --summary "New summary"

# Update content
gsc notes update --id <id> --content "Updated content"

# Update from file
gsc notes update --id <id> --from-file /tmp/note.json
```

### Delete a Note

```bash
gsc notes delete <id>
```

### List and Search

```bash
# List all notes
gsc notes list

# Filter by tag
gsc notes list --tag architecture

# Filter by topic
gsc notes list --topic cli-architecture

# Filter by importance
gsc notes list --importance high

# Full-text search
gsc notes search "cobra"
```

---

## 4. Note Schema

### Required Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `summary` | string | Short description (≤240 chars) |
| `topic` | string | Primary topic slug (must be registered via `gsc topics add`) |

### Optional Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `content` | string | Detailed note content (≤10000 chars) |
| `glob_patterns` | array | Glob patterns for file matching |
| `linked_files` | array | Specific file paths |
| `tags` | array | Categorization tags |
| `importance` | string | `low`, `medium`, or `high` (default: `medium`) |
| `related_topics` | array | Up to 2 related topic slugs |

### Note JSON Example

```json
{
  "summary": "CLI architecture notes",
  "content": "The CLI uses cobra for command handling. Each command is in its own file under internal/cli/.",
  "topic": "cli-architecture",
  "glob_patterns": ["internal/cli/**"],
  "tags": ["architecture", "cli"],
  "importance": "medium"
}
```

### Topics

Every knowledge item (rule, note, lesson) must reference exactly one primary topic. Topics organize repository knowledge across all knowledge types.

```bash
# Register a topic before creating notes
gsc topics add cli-architecture --description "CLI architecture and design patterns"

# List registered topics
gsc topics list

# Search for a topic
gsc topics search "architecture"
```

If a topic is not registered, note creation will fail with:
```
topic "cli-architecture" not registered; add with: gsc topics add cli-architecture --description "..."
```

---

## 5. Agent Integration Pattern

### Session Start

When `gsc experts init` is used:

1. Agent reads the expert context
2. Agent knows notes are available for context and reference
3. Agent can query notes before working on files
4. Notes are reference material, not enforcement

### Before Working on a File

```bash
gsc notes get --file <file-path> --format json
```

If notes are found, the agent should:
1. Read the notes for context
2. Use the information to understand the codebase
3. Not treat notes as instructions to follow

### Unified Knowledge Search

Notes can be discovered alongside rules and lessons:

```bash
# Search all knowledge types
gsc knowledge search "cli architecture"

# Browse by topic
gsc knowledge list --topic cli-architecture
```

---

## 6. Glob Pattern Syntax

| Pattern | Matches |
| :--- | :--- |
| `internal/cli/**` | All files under internal/cli/ |
| `**/*.go` | All Go files |
| `internal/cli/*.go` | Go files directly in internal/cli/ |

---

## 7. Best Practices

1. **Register topics first** — Use `gsc topics add` before creating notes
2. **Use specific globs** — Avoid `**` when possible; prefer `internal/cli/**`
3. **Tag notes** — Enables hierarchical querying
4. **Set importance** — Helps agents prioritize when multiple notes match
5. **Keep content focused** — One topic per note
6. **Use linked files** — For specific files the note is about
7. **Use topic hierarchy** — Use `related_topics` for cross-references

---

## 8. Notes vs Rules vs Lessons

| Type | Purpose | Example |
| :--- | :--- | :--- |
| **Rules** | Safe guards (guardrails) - things agents MUST follow | "Do not run gofmt -w" |
| **Lessons** | What we learn to ensure we don't need safe guards | "The skip happens in walk.rs, not core" |
| **Notes** | Things we want to keep track of - a scratchpad | "TODO: refactor this module" |

Notes are for research, context, and observations. They help agents understand the codebase but don't enforce behavior.

---

## 9. Common Mistakes

| Mistake | Correction |
| :--- | :--- |
| Using absolute paths in globs | Use repo-relative paths |
| Treating notes as instructions | Notes are reference, not enforcement |
| Not rebuilding Brain | `gsc notes add/delete/update` auto-rebuilds |
| Using notes for rules | Use `gsc rules` for guardrails |
| Using notes for lessons | Use `gsc lessons` for learned constraints |
| Topic not registered | Run `gsc topics add <slug> --description "..."` first |
