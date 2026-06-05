<!--
Component: GSC Query Guide
Block-UUID: 78aae5cf-349f-4432-b44d-fc174d9c6111
Parent-UUID: N/A
Version: 1.0.0
Description: Detail guide for querying, filtering, field discovery, insights, and coverage in the gsc Intelligence Layer. Load when the user asks about querying, filtering, or data analysis.
Language: Markdown
Created-at: 2026-05-01T00:20:05.230Z
Authors: Gemini 2.5 Flash Lite (v1.0.0)
-->


# GSC Query Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc brains` | List all registered Brains |
| `gsc brains <name>` | Inspect fields available in a Brain |
| `gsc fields <db>` | List all fields and their types in a Brain |
| `gsc values <db> <field>` | List all unique values for a specific field |
| `gsc query --db <db> --filter "<expr>"` | Find files matching metadata criteria |
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

---

## 3. Query vs. Enriched Search - When to Use Which

| Use `gsc query` when… | Use `gsc grep` when… |
| :--- | :--- |
| Looking for **concepts or intent** ("authentication files") | Looking for a **specific symbol** (`DEFAULT_TTL`, `handlePayment`) |
| The answer is fully described by metadata | The exact code location matters |
| You want a clean file list without code snippets | You need code context alongside metadata |

`gsc grep` **eliminates blind searching**. Unlike standard `grep`, it enriches
every match with metadata from the Brain:

```bash
gsc grep DEFAULT_TTL --db code-intent --fields purpose
```

Returns the line match **plus** the purpose of every matched file, letting
you immediately discard irrelevant matches (e.g., a test fixture vs.
the actual config module).

---

## 4. Analysis: Understanding the Codebase

### Insights - Distribution Analysis
```bash
gsc insights --db <brain> --fields <field1,field2> --limit 10
```
Shows how many files fall under each value of a field. Use this to understand
the vocabulary of the repo ("what are the most common layers?") or to confirm
a filter will return meaningful results before writing a query.

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

## 6. Common Mistakes

| Mistake | Correction |
| :--- | :--- |
| Using `=` expecting strict equality | `=` is a substring match. Use `gsc values` first to see exact field values. |
| Trusting an empty query result as "not found" | Always run `gsc coverage` to check for blind spots before concluding. |
| Using multiple `--filter` flags expecting OR | Multiple flags are AND. Use `in (a,b)` for OR logic. |
| Querying without knowing field names | Run `gsc brains <name>` or `gsc fields <name>` first. |
| Grepping for a concept | Concepts belong in `gsc query --filter`. Save `grep` for symbols and strings. |
