<!--
Component: GSC Query Guide
Block-UUID: 9ee392b0-a2d1-466e-896f-07e3083ac2fe
Parent-UUID: a296ca2e-a615-4b55-952e-55703cd0891a
Version: 1.5.0
Description: Documented glob-only metadata projection for gsc query. Clarified that --filter is for metadata predicates while --glob may stand alone when retrieving selected fields for known files or path scopes.
Language: Markdown
Created-at: 2026-05-01T00:20:05.230Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), MiMo-v2.5-Pro (v1.1.0), claude-sonnet-4-6 (v1.2.0), claude-sonnet-4-6 (v1.3.0), claude-sonnet-4-6 (v1.4.0), Codex GPT-5 (v1.5.0)
-->


# GSC Query Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc brains` | Human-friendly active Brain detail plus condensed inactive Brain manifests |
| `gsc brains --json` | Rich structured Brain discovery for coding agents |
| `gsc brains <name>` | Inspect fields available in a Brain |
| `gsc fields <db>` | List all fields and their types in a Brain |
| `gsc values <db> <field>` | List all unique values for a specific field |
| `gsc query --db <db> --filter "<expr>" --limit N` | Find files matching metadata criteria |
| `gsc query --db <db> --glob "<path-or-pattern>" --fields <list> --limit N` | Retrieve selected metadata for known files or path scopes |
| `gsc insights --db <db> --fields <list>` | Analyze value distributions across the codebase |
| `gsc coverage --db <db>` | Show analyzed vs. unanalyzed file coverage |

---

## 1. Discovery: What's Available?

Always run discovery before querying. An unknown Brain is a missed tool.

### List Brains
```bash
gsc brains
```

Returns all registered Brains with name, description, and file count.
For coding agents, prefer:

```bash
gsc brains --json
```

The JSON output preserves active Brain fields and descriptions and may include
`inactive_databases` for manifests found in `.gitsense/manifests` that are not
yet imported.

If this returns no Brains, metadata-backed commands such as `gsc query`,
`gsc insights`, `gsc coverage`, `gsc fields`, and `gsc values` are not
available yet. Tell the user there are no active Brains and continue with
text/path search (`gsc rg` without `--db`, or standard `rg`) when possible.
If inactive Brains are listed, suggest the shown import command to activate
one. Otherwise suggest `gsc manifest import <uri>` only as the path to enable
structured metadata querying.

### Inspect a Brain's Fields
```bash
gsc brains <brain-name>
# or
gsc fields <brain-name>
```

Returns field names, types (scalar vs. array), and descriptions.

### List Unique Field Values
```bash
gsc values <db> <field>
```
Shows all distinct values for a field. Use this to understand the "vocabulary"
before writing a filter (e.g., what are the valid `risk_level` values?).

---

## 2. Querying: Finding Files by Metadata

### Filter Operators

| Operator | Behavior | Example |
| :--- | :--- | :--- |
| `=` | Contains (substring, not exact) | `--filter "purpose=auth"` |
| `!=` | Does not contain | `--filter "status!=deprecated"` |
| `~` | Contains (explicit alias for `=`) | `--filter "purpose~payment"` |
| `!~` | Not contains | `--filter "layer!~test"` |
| `in` | List membership (OR logic) | `--filter "risk in (high,critical)"` |
| `not in` | Exclude list | `--filter "layer not in (test,docs)"` |
| `>` | Greater than (numeric) | `--filter "complexity>10"` |
| `<` | Less than (numeric) | `--filter "complexity<5"` |
| `>=` | Greater than or equal | `--filter "score>=80"` |
| `<=` | Less than or equal | `--filter "score<=20"` |
| `exists` / `!exists` | Field presence check | `--filter "owner exists"` |

**Range shorthand:**
```bash
--filter "complexity=5..20"   # inclusive numeric range
```

**Semicolon separator** (AND logic within one flag):
```bash
--filter "risk=high;layer=backend"
```

### Combining Filters (AND logic across flags)
```bash
gsc query --db code-intent --filter "language=go" --filter "risk_level=high"
```
Returns files that are **both** Go **and** high risk.

### Selecting Additional Fields
```bash
gsc query --db code-intent --filter "purpose~auth" --fields "owner,risk_level"
```

`--filter` is only required when you are applying a metadata predicate. If you already know the files or directory pattern, use `--glob` by itself with `--fields` to project metadata for that path scope.

### Limiting Results

Use `--limit` to cap the number of files returned. This is especially important for
coding agents working within token budgets.

```bash
# Return at most 20 matching files
gsc query --db code-intent --filter "layer=backend" --limit 20
```

Default: 0 (unlimited). When results are truncated, the output includes a notice.

### Scoping by Path — Prefer `--glob`

When you want to restrict results to a directory or file pattern, use `--glob` rather than `--filter "file_path=..."`. The `file_path=` filter requires an exact full path and does not support partial matches or wildcards. `--glob` accepts standard glob patterns. It can be used alone to retrieve metadata for known files, or combined with `--filter` for metadata conditions.

```bash
# Retrieve metadata for a known file without a metadata predicate
gsc query --db code-intent --glob "src/api/router.go" --fields file_path,purpose,keywords --limit 20 --format json

# Retrieve metadata for an explicit file set
gsc query --db code-intent --glob "src/api/router.go" --glob "src/api/handlers.go" --fields file_path,purpose --limit 20 --format json

# Scope to a directory
gsc query --db code-intent --filter "layer=backend" --glob "src/api/**/*.go"

# Scope to a file pattern across the repo
gsc query --db code-intent --filter "risk_level=high" --glob "**/*_handler.go"
```

Prefer `--glob "<path>"` for single-file metadata projection. Reserve `--filter "file_path=<path>"` only when you specifically need to treat `file_path` as a metadata field in a predicate.

---

## 3. Query vs. Enriched Search - When to Use Which

| Use `gsc query` when… | Use `gsc rg` when… |
| :--- | :--- |
| Looking for **concepts or intent** ("authentication files") | Looking for a **specific symbol** (`DEFAULT_TTL`, `handlePayment`) |
| The answer is fully described by metadata | The exact code location matters |
| You want a clean file list without code snippets | You need code context alongside metadata |

`gsc rg` enriches every match with metadata from the active Brain:

```bash
gsc rg DEFAULT_TTL --db code-intent --fields purpose
```

Returns the line match **plus** the purpose of every matched file, letting
you immediately discard irrelevant matches (e.g., a test fixture vs.
the actual config module).

---

## 4. Analysis: Understanding the Codebase

### Insights - Distribution Analysis
```bash
gsc insights --db <brain> --fields <field1,field2> --limit 250 --format json
```
Shows how many files fall under each value of a field. Use this to understand
the vocabulary of the repo ("what are the most common layers?") or to confirm
a filter will return meaningful results before writing a query.

For AI agents, prefer `--format json` so each value is explicit:
`value` is the metadata value, `count` is how many in-scope files map to it,
and `percentage` is that count divided by the current in-scope file count.

**Important:** `percentage` is a distribution metric, not a relevance,
confidence, or quality score. A low percentage often means the value is narrow
and precise. A high percentage may be less useful for discovery because it maps
to many files and leaves more ambiguity.

### Coverage - The Map of the Unknown
```bash
gsc coverage --db <brain>
```
Compares Git-tracked files against the Brain's analyzed files.

**Critical AI duty:** If `gsc query` returns an empty result, do **not**
assume the feature doesn't exist. Run `gsc coverage` first. The Brain is a
**curated subset** - binary files, files over 25,000 tokens, and
user-excluded directories (migrations, archives) are intentionally omitted.
If the file is in a blind spot, surface that to the user before concluding
an answer.

**Output fields:**
- **Focus Coverage**: % of in-scope files analyzed
- **Total Coverage**: % of all tracked files analyzed
- **Blind Spots**: Directories with unanalyzed files

---

## 5. Scope: Restricting the Search

```bash
gsc query --filter "..." --scope-override "include=src/**;exclude=tests/**"
```

Scope precedence (highest to lowest):
1. `--scope-override` flag
2. Active profile (`gsc config use <profile>`)
3. `.gitsense-map` file at repo root
4. Default (all tracked files)

---

## 6. Brain-Specific Notes

### gsc-lessons

The `gsc-lessons` Brain has both scalar and array fields. Use the right field for the right tool:

| Use case | Command |
| :--- | :--- |
| Quick file-level overlay in search | `gsc rg <term> --db gsc-lessons --fields latest_lesson_summary,lesson_count` |
| When was the last lesson recorded | `gsc rg <term> --db gsc-lessons --fields latest_lesson_at` |
| How many lessons exist for a file | `gsc rg <term> --db gsc-lessons --fields lesson_count` |
| Full lesson content for a file | `gsc query --db gsc-lessons --glob "<path>" --fields lesson_summaries,lesson_details --limit 20 --format json` |
| Find all lessons by topic or tag | `gsc query --db gsc-lessons --filter "tags=<tag>" --fields lesson_summaries,lesson_details --format json` |

**Always use `--format json` when requesting array fields** (`lesson_summaries`, `lesson_details`, `review_checks`, etc.) — without it, arrays render as flat text and lose structure.

Scalar fields safe for `gsc rg` overlays: `latest_lesson_summary`, `lesson_count`, `latest_lesson_at`, `purpose`.

`purpose` is reserved for GitSense Chat synthesis and will be empty until Chat writes it. Use `latest_lesson_summary` as the default overlay field.

### Discovering Keywords and Tags Before Filtering

Never guess keyword or tag values. Use `gsc insights` to discover what exists first — it supports substring matching so you can search the vocabulary itself:

```bash
# Find all keywords containing "binary"
gsc insights --db gsc-lessons --fields keywords --filter "keywords=binary" --limit 250 --format json

# Find all tags containing "detection"
gsc insights --db gsc-lessons --fields tags --filter "tags=detection" --limit 250 --format json

# Browse the full keyword and tag vocabulary
gsc insights --db gsc-lessons --fields keywords,tags --limit 250 --format json
```

Use `--limit 250` as the default for discovery — the output is just `value | count` rows so token cost is low, and a smaller limit risks missing the keyword you need. Increase further if the result appears truncated.

When using JSON output, treat `count` as "files mapped to this keyword/tag" and
`percentage` as "share of current in-scope files." Do not discard a value
because its percentage is low. For narrow concepts, a low-count value may be
the most precise value to query. Conversely, a high-percentage value can be
less useful because it maps to many files.

Once you have an exact value, use it in a query:

```bash
gsc query --db gsc-lessons --filter "keywords=binary-detection" --fields lesson_summaries,lesson_details --format json
```

**Never use `gsc values gsc-lessons keywords`** — it returns raw JSON array strings, not individual keyword values.

---

## 7. Common Mistakes

| Mistake | Correction |
| :--- | :--- |
| Using `=` expecting strict equality | `=` is a substring match. Use `gsc values` first to see exact field values. |
| Trusting an empty query result as "not found" | Always run `gsc coverage` to check for blind spots before concluding. |
| Using multiple `--filter` flags expecting OR | Multiple flags are AND. Use `in (a,b)` for OR logic. |
| Querying without knowing field names | Run `gsc brains <name>` or `gsc fields <name>` first. |
| Running metadata commands when no Brains exist | Tell the user no Brains are active; use `gsc rg` without `--db` or standard `rg`, then suggest `gsc manifest import <uri>` if they want metadata intelligence. |
| Grepping for a concept | Concepts belong in `gsc query --filter`. Save `grep` for symbols and strings. |
| Assuming `--filter` is required for every query | Use `--filter` for metadata predicates. Use `--glob` alone when projecting metadata for known files or path scopes. |
| Using `--filter "file_path=<partial>"` to scope by directory | `file_path=` requires an exact full path. Use `--glob "dir/**/*.rs"` instead. |
