<!--
Component: Scout Tool Capabilities
Block-UUID: 06ea4428-76f9-47ac-a167-d6825c612b6b
Parent-UUID: f418ab5e-1343-4f8b-b7ca-84efdadf1283
Version: 1.1.0
Description: Practical reference guide for gsc tools with discovery-focused examples.
Language: Markdown
Created-at: 2026-04-19T15:23:45.295Z
Authors: Gemini 3 Flash (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0)
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

**Examples** (ordered by complexity):
1. **Simple identifier**: `gsc grep --summary --fields purpose,keywords --db code-intent --format json "buildIntelligence"` (Returns metadata only, no code snippets).
2. **Partial match / wildcard**: `gsc grep --summary --fields purpose,keywords --db code-intent --format json "contract.*TTL"` (Regex wildcard for compound terms).
3. **Domain-scoped search**: `gsc grep --filter "keywords in (auth)" --format json "validateToken"` (Searches code only within the auth domain).
4. **Multi-concept - use sequential commands** (do NOT combine with `\|`):
   - `gsc grep --summary --fields purpose,keywords --db code-intent --format json "DefaultContractTTL"`
   - `gsc grep --summary --fields purpose,keywords --db code-intent --format json "contract.*expir"`

### ⚠️ Multi-Pattern Pitfalls

| Anti-pattern | Why it fails | Correct alternative |
|---|---|---|
| `"foo\|bar"` | `\|` is not supported as OR | Use two sequential `gsc grep` commands |
| `"foo OR bar"` | Literal string match, not boolean | Use `--filter` with `in (...)` |

### When to use `gsc query` instead of `gsc grep`
- Use `gsc query` when you know a keyword domain but not a specific code pattern
- Use `gsc grep` only when you need to match literal code content or identifiers
- For multi-concept searches, prefer: `gsc query --filter "keywords in (contract, expiry, ttl)" --format json`

---

## Important: JSON vs. Piping
- Use `--format json` when you need structured data for your final answer.
- Do NOT use `--format json` if you are piping to `sort`, `head`, or `tail`, as the JSON structure will be destroyed.
