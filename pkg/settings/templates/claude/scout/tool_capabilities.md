<!--
Component: Scout Tool Capabilities
Block-UUID: f418ab5e-1343-4f8b-b7ca-84efdadf1283
Parent-UUID: N/A
Version: 1.0.0
Description: Practical reference guide for gsc tools with discovery-focused examples.
Language: Markdown
Created-at: 2026-04-03T02:02:47.526Z
Authors: Gemini 3 Flash (v1.0.0)
-->


# Scout Tool Capabilities Reference

Lead with `gsc insights` to build your mental map, then use `gsc query` and `gsc grep` to discover candidates.

---

## Foundation: gsc insights
**What it does**: Returns frequency statistics for keywords and file extensions. Use this to "Pivot"-checking if a search term is too broad or too narrow.

**Constraints**: Max `--limit` is 1000.

**Examples**:
1. **Map the Repo**: `gsc insights --db code-intent --fields keywords --limit 50 --format json`
2. **Find Latest Activity**: `gsc insights --db code-intent --fields dates | sort | tail -1` (Note: No `--format json` when piping to sort).
3. **Discover Keyword Clusters**: `gsc insights --db code-intent --fields keywords --filter "keywords in (*auth*,*token*)" --format json`
4. **Explore Available Fields**: `gsc brains code-intent --format json` (Shows all metadata fields available for search)

---

## Primary: gsc query
**What it does**: Strictly metadata-only search. Fast and token-efficient. Use this to find files by property (keyword, date, purpose) without reading code.

**Examples**:
1. **Find by Domain**: `gsc query --db code-intent --filter "keywords in (authentication)" --fields purpose,keywords --format json`
2. **Temporal Query**: `gsc query --db code-intent --filter "dates in (2026-03-15_*)" --fields purpose --format json` (Matches any date type for that day).
3. **Narrow with AND Logic**: `gsc query --filter "keywords in (blog)" --filter "dates in (2026-03-15_*)" --format json`

---

## Secondary: gsc grep
**What it does**: Searches code content and enriches with metadata. Use when the intent involves specific code patterns (e.g., function names).

**Examples**:
1. **Discovery Grep (Token Saver)**: `gsc grep --summary --fields purpose,keywords --db code-intent --format json "buildIntelligence"` (Returns metadata only, no code snippets).
2. **Filtered Grep**: `gsc grep --filter "keywords in (auth)" --format json "validateToken"` (Searches code only within the auth domain).

---

## Important: JSON vs. Piping
- Use `--format json` when you need structured data for your final answer.
- Do NOT use `--format json` if you are piping to `sort`, `tail`, `head`, or `wc`, as the JSON structure will be destroyed.
