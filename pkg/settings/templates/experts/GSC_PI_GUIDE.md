<!--
Component: GSC Pi Guide
Block-UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Parent-UUID: 5034a927-3965-400a-9d7a-bdbde3290971
Version: 1.0.0
Description: Reference guide for the gsc pi command group, covering session management, the interactive resume picker, HUD sidebar, and session queries.
Language: Markdown
Created-at: 2026-06-22T00:00:00Z
Authors: MiMo-v2.5-Pro (v1.0.0)
-->

# gsc pi — Command Reference

`gsc pi` manages Pi coding session data: importing session logs, querying past conversations, resuming sessions, and monitoring context usage.

## Quick Reference

| Command / Flag | Purpose |
| :--- | :--- |
| `gsc pi -r`, `--resume` | Interactive session picker with split-pane preview |
| `gsc pi -b`, `--brains` | Show session statistics (tokens, model, files) |
| `gsc pi --hud` | Pick a session and open in tmux split with HUD sidebar |
| `gsc pi sessions sync` | Import Pi session JSONL files into the SQLite mirror |
| `gsc pi sessions list` | List imported sessions |
| `gsc pi sessions query` | Full-text search across sessions |
| `gsc pi sessions show <id>` | Show detailed session information |
| `gsc pi sessions verify` | Verify session import fidelity |

---

## The Resume Picker (`gsc pi -r`)

The resume picker is an interactive TUI for browsing and resuming Pi sessions. It has three focus zones navigated with `Tab`:

### Focus Zones

| Zone | Purpose | Navigation |
| :--- | :--- | :--- |
| **List** (left) | Session rows with relative time and title | `↑/↓` browse, `Enter` resume |
| **Preview** (right) | First/last N messages with markdown rendering | `↑/↓` scroll |
| **Options** (top bar) | Scope, Sort, Range controls | `←/→` pick control, `↑/↓` change value |

### Preview Pane Features

The preview pane renders messages with markdown-like formatting:
- **Role labels** are capitalized (User, Assistant, Tool Result, etc.)
- **Horizontal dividers** separate each message for visual clarity
- **Paragraphs** are separated by blank lines
- **Code blocks** are displayed with distinct background styling
- **Bold text** is rendered with emphasis
- **Inline code** is highlighted
- **Lists** show bullet points (`•`)

This makes it easier to scan session content without opening the full session.

### Options Controls

| Control | Values | Effect |
| :--- | :--- | :--- |
| **Scope** | All / Cwd | Filter to sessions in the current working directory |
| **Sort** | Updated / Created | Order by last message time or creation time |
| **Range** | Last / First | Show last N or first N messages in preview |

### Other Keybindings

| Key | Action |
| :--- | :--- |
| `Tab` / `Shift+Tab` | Cycle focus between zones |
| `Ctrl+O` | Cycle row density (comfortable / compact) |
| `Ctrl+C` | Quit |
| `Esc` | Clear search (if active) or quit |
| Printable characters | Type to search (always active) |

### Search

Search is always on — typing filters sessions by title, repo root, or working directory. `Backspace` edits the query, `Esc` clears it.

---

## Session Statistics (`gsc pi -b`)

Shows token usage, model info, and touched files for a session:

```bash
# Stats for a specific session
gsc pi -b <session-id>

# Stats for all sessions (aggregated)
gsc pi -b
```

The output includes:
- **Context tokens**: input + output + cache read/write
- **Cost**: provider-reported cost total
- **Model**: provider and model ID
- **Files**: number of files touched

---

## HUD Mode (`gsc pi --hud`)

Opens a tmux split pane with:
- **Left**: An interactive Pi session (`pi --session <id>`)
- **Right**: A live HUD sidebar showing context usage and touched files

Requires `tmux` to be installed.

---

## Session Sync (`gsc pi sessions sync`)

Imports Pi session JSONL files from `~/.pi/sessions/` into a SQLite mirror for fast querying.

```bash
# One-time sync
gsc pi sessions sync

# Continuous sync (background daemon)
gsc pi sessions sync --continuous

# Check sync status
gsc pi sessions sync status

# Stop continuous sync
gsc pi sessions sync stop
```

The mirror database lives at `$GSC_HOME/data/pi/pi-sessions.sqlite3` by default.

---

## Querying Sessions (`gsc pi sessions query`)

Full-text search across all imported sessions:

```bash
# Search messages
gsc pi sessions query "error handling"

# Filter by role
gsc pi sessions query --role user "deploy"

# Filter by tool
gsc pi sessions query --tool bash "git push"

# Filter by file
gsc pi sessions query --file internal/cli/root.go

# Limit results
gsc pi sessions query --limit 10 "TODO"

# JSON output for scripting
gsc pi sessions query --format json "migration"
```

### Query Flags

| Flag | Purpose |
| :--- | :--- |
| `--session-id` | Filter to a specific session |
| `--role` | Filter by message role (user, assistant) |
| `--tool` | Filter by tool name (bash, read, edit, write) |
| `--file` | Filter by repo-relative file path |
| `--abs-file` | Filter by absolute file path |
| `--op` | Filter by file operation (read, edit, write) |
| `--provider` | Filter by AI provider |
| `--model` | Filter by model ID |
| `--since` / `--until` | Timestamp bounds (RFC3339) |
| `--sort` | Order: recent (default), oldest, match-count |
| `--limit` | Max results (default 50) |
| `--format` | Output: human (default), json |

---

## Listing Sessions (`gsc pi sessions list`)

```bash
# List recent sessions
gsc pi sessions list

# Filter by repo
gsc pi sessions list --repo /path/to/repo

# JSON output
gsc pi sessions list --format json
```

---

## Session Details (`gsc pi sessions show`)

```bash
gsc pi sessions show <session-id>
```

Shows: session ID, name, working directory, provider, model, message count, tool call count, file ref count, and the first/last user message.

---

## Verifying Import Fidelity (`gsc pi sessions verify`)

Checks that the SQLite mirror matches the source JSONL files:

```bash
gsc pi sessions verify
```

---

## Read-Only Session Helpers

These commands parse Pi JSONL session files directly without requiring a SQLite mirror. They are useful for triggers and debugging.

### Show Active Branch (`gsc pi sessions branch`)

Display the active branch entries from root to leaf:

```bash
gsc pi sessions branch --session /path/to/session.jsonl --leaf entry-123 --format json
```

### Show Tool Calls (`gsc pi sessions tool-calls`)

Display tool calls and their results from the active branch:

```bash
gsc pi sessions tool-calls --session /path/to/session.jsonl --leaf entry-123 --format json
```

Tool calls are joined with their results by `toolCallId`.

### Show File References (`gsc pi sessions files`)

Display file references (read, edit, write) from the active branch:

```bash
gsc pi sessions files --session /path/to/session.jsonl --leaf entry-123 --format json
```

Files are extracted from:
- Tool calls: `read(path)`, `edit(path)`, `write(path)`
- Branch summary fields: `readFiles[]`, `modifiedFiles[]`

### Show Errors (`gsc pi sessions errors`)

Display failed tool results from the active branch:

```bash
# All errors
gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --format json

# Filter by tool
gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --tool bash --format json

# Filter by error text
gsc pi sessions errors --session /path/to/session.jsonl --leaf entry-123 --contains TS2307 --format json
```

### Flags

| Flag | Purpose |
| :--- | :--- |
| `--session` | Path to session JSONL file (required) |
| `--leaf` | Leaf entry ID (default: latest) |
| `--tool` | Filter errors by tool name |
| `--contains` | Filter errors by substring in result text |
| `--format` | Output format: human (default), json |

---

## Common Workflows

### Resume a recent session
```bash
gsc pi -r
# Use ↑/↓ to browse, Tab to switch zones, Enter to resume
```

### Find sessions that touched a file
```bash
gsc pi sessions query --file internal/cli/pi/resume.go
```

### Check context usage before resuming
```bash
gsc pi -b <session-id>
```

### Monitor context in real-time
```bash
gsc pi --hud
```

---

## Troubleshooting

| Problem | Solution |
| :--- | :--- |
| "No Pi sessions found" | Run `gsc pi sessions sync` first |
| Sync finds no files | Check that `~/.pi/sessions/` contains `.jsonl` files |
| HUD requires tmux | Install tmux: `brew install tmux` (macOS) or `apt install tmux` (Linux) |
| Database not found | Use `--db` flag or set `GSC_HOME` environment variable |
