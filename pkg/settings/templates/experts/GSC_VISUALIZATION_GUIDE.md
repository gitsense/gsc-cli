<!--
Component: GSC Visualization Guide
Block-UUID: 28ceaa67-2a78-4da8-8801-d45b692c6df3
Parent-UUID: 4e25d3d3-c1c5-4d29-b92e-2a910622178f
Version: 1.6.0
Description: Updated documentation to promote 'gsc rg' as the primary command for better AI alignment. Added fail-fast validation note for POSIX alternation (\|). Updated all examples to use 'gsc rg' instead of 'gsc grep'. Kept 'gsc grep' as legacy alias with syntax warning.
Language: Markdown
Created-at: 2026-05-02T12:24:46.869Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
-->


# GSC Visualization Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc tree --db <db> --fields <list>` | View enriched file tree with metadata |
| `gsc tree --filter "<expr>"` | Prune tree to matching files only |
| `gsc tree --focus "<path>"` | Restrict tree to a specific subtree |
| `gsc tree --no-prune` | Heat map: show all files, highlight matches |
| `gsc rg <pattern> --db <db>` | Search code + enrich matches with metadata (recommended) |
| `gsc grep <pattern> --db <db>` | Legacy alias (uses ripgrep syntax) |
| `gsc rg <pattern> --summary` | Aggregate match counts (no code snippets) |

---

## 1. Visualizing Structure: `gsc tree`

`gsc tree` builds the repository hierarchy from Git-tracked files and enriches
each node with metadata from a Brain.

### Visualization Modes

| Mode | How to Activate | What It Shows |
| :--- | :--- | :--- |
| **Pruned (Default)** | Apply `--filter` | Only files matching the filter; irrelevant paths hidden |
| **Heat Map** | Add `--no-prune` | All files shown; matching files highlighted |
| **Raw Heat Map** | Add `--no-prune --no-compact` | Same as above, but non-matching filenames visible |

### Filtering the Tree

```bash
# By metadata attribute
gsc tree --db code-intent --filter "layer=backend"

# By directory path
gsc tree --db code-intent --focus "internal/claude/**"

# By file extension
gsc tree --db code-intent --glob "**/*.go"
```

Filters can be combined: `--filter`, `--focus`, and `--glob` are AND-joined.

### Output Formats

| Format | Flag | Best For |
| :--- | :--- | :--- |
| `human` | `--format human` | Terminal display with color and ASCII art |
| `json` | `--format json` | Programmatic consumption; full node metadata |
| `ai-portable` | `--format ai-portable` | **Preferred for AI.** Simplified JSON; removes internal flags that waste tokens. |

### How Enrichment Works (Pipeline)

1. **Build Tree** - Constructs hierarchy from Git-tracked files.
2. **Fetch Metadata** - Batch-queries the Brain for all file paths.
3. **Enrich Nodes** - Attaches metadata; evaluates filters to determine match status.
4. **Propagate Visibility** - A matching child makes its parent directory visible.
5. **Prune** - Removes non-matching nodes (unless `--no-prune`).

---

## 2. Searching Code: `gsc rg` (Recommended)

`gsc rg` eliminates blind searching. Standard `grep` returns a line of code.
`gsc rg` returns the line of code **plus the metadata context of every
matched file** - letting you immediately discard irrelevant matches.

```bash
# Find a symbol and see the purpose of every file that contains it
gsc rg DEFAULT_TTL --db code-intent --fields purpose
```

### ⚠️ `gsc rg` Uses Ripgrep Syntax (NOT POSIX grep)

`gsc rg` uses **ripgrep** as its search engine, not POSIX `grep`. This means
two critical rules apply:

**Rule 1: Do not pass POSIX `grep` flags to `gsc grep`.**

The following POSIX `grep` flags are invalid and will cause errors or unexpected
behaviour with `gsc rg`:

| Invalid `grep` Flag | What to do instead |
| :--- | :--- |
| `-r`, `--recursive` | Not needed; `gsc rg` searches the whole repo by default |
| `-E` (extended regex) | Extended regex is on by default in ripgrep |
| `-P` (Perl regex) | Not supported; backreferences and lookaheads are unavailable |
| `-l` (list files only) | Use `--summary` to get file-level aggregation |
| `-n` (show line numbers) | Line numbers are shown by default |
| `--include="*.go"` | Use `gsc rg <pattern> --db <db> -g "**/*.go"` |
| `--exclude=<pattern>` | Not directly supported; use `--filter` on metadata instead |

Only use the `gsc rg` flags documented in the Output Flags table below.

**Rule 2: Patterns must follow ripgrep (Rust regex) syntax, not POSIX syntax.**

Ripgrep uses the Rust regex engine (RE2-like). Several POSIX `grep` constructs
are not supported:

| Feature | POSIX `grep` | ripgrep (used by `gsc rg`) |
| :--- | :--- | :--- |
| Backreferences (`\1`, `\2`) | ✅ Supported | ❌ **Not supported** |
| Lookahead / Lookbehind | Requires `-P` | ❌ **Not supported** |
| POSIX character classes (`[:digit:]`, `[:alpha:]`) | ✅ Supported | ❌ Use `\d`, `\w`, `\s` instead |
| `\b` word boundary | ✅ Supported | ✅ Supported |
| `.` matches newline | Requires `-z` | ❌ Does not match newline by default |
| Alternation a\|b (BRE) | ✅ Supported | ❌ **Not supported (use | instead)** |
| Alternation a|b (ERE) | Requires -E | ✅ **Supported by default** |

**When in doubt, keep patterns simple.** A literal string or a basic `\w+`
pattern is always safe. Avoid complex regex constructs that rely on POSIX
extensions or Perl features.

### 🛡️ Fail-Fast Validation

`gsc rg` includes automatic validation to catch common POSIX syntax errors:

- **`\|` (BRE alternation)**: Will immediately error with a helpful message
  
```
  Error: invalid regex syntax: found '\|' (POSIX BRE). gsc uses ripgrep (ERE) syntax.
  Use '|' for alternation instead. Example: "pattern1|pattern2"
  ```

This prevents silent failures and guides you to the correct syntax immediately.

### The Dual-Pass Workflow

1. **Discovery Pass** - Runs `ripgrep` to find all files containing the pattern.
2. **Enrichment Pass** - Looks up each matched file in the Brain; applies
   metadata filters; attaches requested fields.

### Filtering Search Results

Combine text search with metadata filters to eliminate noise:

```bash
# Find "password" only in high-risk files
gsc rg "password" --db code-intent --filter "risk_level=high"

# Find "TODO" only in the backend layer
gsc rg "TODO" --db code-intent --filter "layer=backend"
```

### Output Flags

| Flag | Description |
| :--- | :--- |
| `--summary` | **Token-saving option.** Returns aggregated statistics only - no code snippets. Use when you need to know *if* something exists or *how many* files match. |
| `--context <N>` | Show N lines of surrounding code context per match |
| `--limit <N>` | Cap the number of files in results (default: 50) |
| `--fields <list>` | Select specific metadata fields to display |
| `--no-fields` | Suppress metadata; show code matches only |
| `-i`, `--ignore-case` | Case-insensitive search |
| `-v`, `--invert-match` | Show non-matching lines |
| `-w`, `--word-regexp` | Match whole words only |
| `-F`, `--fixed-strings` | Treat pattern as literal string (not regex) |
| `-g <glob>` | Filter files by glob pattern |
| `-m <N>`, `--max-count <N>` | Limit matches per file |
| `--hidden` | Include hidden files and directories |
| `--no-ignore` | Don't respect .gitignore |
| `-e <pattern>` | Add multiple search patterns (OR logic) |

### Examples with New Flags

```bash
# Case-insensitive search for "error"
gsc rg -i "error" --db code-intent --fields purpose

# Find whole word "TODO" only in Go files
gsc rg -w "TODO" -g "**/*.go" --db code-intent

# Search for literal string "func(" (not regex)
gsc rg -F "func(" --db code-intent

# Search for multiple patterns using -e flags (recommended for many patterns)
gsc rg "TODO" -e "FIXME" -e "HACK" --db code-intent

# Search for multiple patterns using | alternation (simpler for 2-3 patterns)
gsc rg "ttl|expir" --db code-intent --fields purpose

# Include hidden files and ignore .gitignore
gsc rg "secret" --hidden --no-ignore --db code-intent
```

---

## 3. Tree vs. Grep - Decision Matrix

| Scenario | Use |
| :--- | :--- |
| "Show me the structure of the payment module" | `gsc tree --focus "internal/payment/**"` |
| "What files handle authentication?" | `gsc tree --filter "purpose~auth"` |
| "Where is `DEFAULT_TTL` defined?" | `gsc rg DEFAULT_TTL --fields purpose` |
| "Find all uses of a deprecated function" | `gsc rg "oldFunc()" --filter "layer!=test"` |
| "How is the codebase organized by layer?" | `gsc tree --db code-intent --fields layer` |
| "Are there any TODO comments in the backend?" | `gsc rg "TODO" --filter "layer=backend" --summary` |
| "What does this unfamiliar directory contain?" | `gsc tree --db code-intent --fields purpose --focus "<path>"` |

**Guiding principle:** Use `gsc tree` to understand *structure and intent*.
Use `gsc rg` to locate *specific symbols, strings, or patterns*.

---

## 4. Best Practices for the AI

1. **Use `--format ai-portable` for tree output.** It strips internal flags,
   giving you clean JSON with lower token cost.
2. **Use `--summary` before full grep.** If you only need to know whether
   a symbol exists, run with `--summary` first to avoid loading code snippets.
3. **Combine text + metadata.** Always add `--fields purpose` to `gsc rg`
   calls - it tells you *why* the matched file exists, so you can filter out
   irrelevant hits without reading the code.
4. **Prefer `gsc tree --focus` over a top-level tree.** Large repos can
   generate very large trees. Restrict with `--focus` when you know the area.
5. **Empty grep results may mean low coverage.** If `gsc rg` returns nothing,
   check `gsc coverage --db <brain>` to confirm the file is actually indexed
   before concluding the symbol doesn't exist.
6. **Use ripgrep-safe patterns only.** Never use backreferences, lookaheads,
   or POSIX character classes in `gsc grep` patterns. Stick to literal strings,
   `\w+`, `\d+`, `\s`, and standard alternation `(a|b)`. If a pattern works
   with standard `grep` but produces unexpected results with `gsc grep`, the
   regex syntax is the likely cause.
7. **Leverage new ripgrep flags for precision.** Use `-w` for whole-word
   matches, `-i` for case-insensitive searches, `-v` to exclude lines,
   and `-e` for multi-pattern searches. These flags are fully compatible with
   `gsc rg` and can significantly reduce false positives.
8. **Prefer `gsc rg` over `gsc grep`.** The `gsc rg` command name aligns with
   ripgrep syntax, reducing AI hallucinations and syntax errors. `gsc grep`
   is retained as a legacy alias but may trigger POSIX syntax assumptions.
