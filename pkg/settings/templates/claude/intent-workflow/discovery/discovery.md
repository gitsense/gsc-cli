<!--
Component: Scout Discovery Methodology
Block-UUID: ae8b5990-fb0b-41a4-bb20-2577172b12fa
Parent-UUID: 33f35136-ea1f-4dbd-a7a9-9c1576e55036
Version: 10.0.0
Description: Discovery methodology for Smart Discovery (find + validate). Merged validation guidelines into discovery methodology. Includes relevance levels, critical missing file guidelines, keyword assessment, and actionable recommendations. Fixed ripgrep syntax - removed incorrect wildcard usage and added proper ripgrep/regex guidance. MAJOR REFACTOR: Removed Relevance Level Guidelines (conflicted with schema), added Scoring Guidelines mapped to numeric score field, added type reminder table, enforced rating enum values, replaced informal terminology with exact JSON key names, fixed confirmed_pattern singular/plural inconsistency, added pre-output validation checklist, added cross-reference warning for schema authority. REMOVED Actionable Recommendations Guidelines (ghost field trap - no schema field exists). FIXED scoring range inconsistency to match system_prompt.md (0.9-1.0, 0.7-0.8, 0.4-0.6).
Language: Markdown
Created-at: 2026-04-19T01:11:00.000Z
Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v3.0.0), GLM-4.7 (v4.0.0), GLM-4.7 (v5.0.0), GLM-4.7 (v5.1.0), GLM-4.7 (v6.0.0), GLM-4.7 (v7.0.0), GLM-4.7 (v8.0.0), GLM-4.7 (v9.0.0), GLM-4.7 (v10.0.0)
-->


⚠️ **SCHEMA AUTHORITY WARNING**: If any instruction in this document appears to conflict with the OUTPUT FORMAT section in `response_format.md`, the OUTPUT FORMAT section takes precedence. The schema specification is the single source of truth for JSON structure and field types.

## Discovery Best Practices

### Ripgrep Syntax Guidelines
- **gsc grep uses ripgrep**: All patterns must be valid ripgrep/regex syntax
- **Use simple keywords**: `contract`, `expiration`, `default` work well
- **Avoid glob wildcards**: Do NOT use `*` at the start/end of patterns (e.g., `*contract*` is invalid)
- **For partial matches**: Use regex patterns like `.*contract.*` or `contract.*`
- **Example**: `gsc grep "contract"` is better than `gsc grep "*contract*"`

### Keyword Selection
- **Common terms are useful**: `time`, `create`, `default` can be highly relevant
- **Let metadata filter**: Use purpose, keywords, and parent_keywords to determine relevance
- **Don't over-complicate**: Simple keywords are often better than forced compound terms
- 2-3 highly relevant keywords are better than 5-10 mixed keywords
- **Don't force it**: If you can't find 5 relevant keywords, 2-3 is fine

### Fail-Fast Evaluation
- **Evaluate semantic alignment first**: Do the matched keywords relate to the intent?
- **File count is secondary**: 100 relevant keywords are better than 5 irrelevant ones
- **Ask for context**: If keywords don't align, ask the user for more details about their problem

### Workflow Discipline
- **Don't repeat keyword discovery**: Once you've passed the fail-fast decision, don't go back to keyword discovery
- **Use discovered keywords for scoring**: If the purpose reveals stronger keywords, use them to score existing candidates
- **Avoid infinite loops**: The workflow is linear - don't revisit earlier steps
- **Stop when confident**: If you find a file with score > 0.9 that directly addresses the intent, you may stop validation early

## ⚠️ CRITICAL: Ripgrep Syntax Requirements

**gsc grep uses ripgrep under the hood**, which means all search patterns must be valid ripgrep/regex syntax.

### Common Mistakes to Avoid

❌ **WRONG**: `gsc grep "*contract*"`
- Error: `rg: regex parse error: (?:*contract*) ^ error: repetition operator missing expression`
- Reason: `*` is a regex quantifier, not a wildcard

✅ **CORRECT**: `gsc grep "contract"`
- Matches: "contract", "contracts", "contract-renewal", etc.
- Reason: Simple keyword matching works well

✅ **CORRECT**: `gsc grep "contract.*"`
- Matches: "contract-renewal", "contract-expiration", etc.
- Reason: Valid regex pattern for "contract" followed by anything

### When to Use Regex Patterns

Use regex patterns only when you need specific matching:
- `contract.*` - matches "contract" followed by anything
- `.*contract` - matches anything ending with "contract"
- `.*contract.*` - matches "contract" anywhere in the text
- `contract-(renewal|expiration)` - matches "contract-renewal" or "contract-expiration"

### Best Practice

**Start with simple keywords**. If simple keywords don't work, then consider regex patterns. Most of the time, simple keywords like `contract`, `expiration`, `default` are sufficient and more reliable.

## Smart Discovery Protocol

### Phase 1: Metadata Search (Fast, Cheap)
1. **Curate Intent Keywords**: Extract 2-10 keywords from the user's intent, aligned with the repository's actual taxonomy from the codebase overview.

2. **The Pivot (Volume Check)**: Use `gsc insights` to check file counts for your keywords.
   - **If >100 files**: Too broad. Add more specific keywords or use AND filters to narrow the cluster.
   - **If 5-50 files**: Sweet spot. Proceed to metadata filtering.
   - **If <5 files**: Too narrow. Broaden keywords or try wildcards.

3. **Metadata Filtering**: Use `gsc query` or `gsc grep --summary` to surface file purposes and keywords. Discard false positives where text matches but semantic purpose does not.

4. **Identify Top Candidates**: Select the top 10-20 candidates based on metadata relevance for code validation.

### Phase 2: Code Validation (Targeted, Precise)
1. **Read Code for Top Candidates Only**: Read actual code for the top 10-20 candidates identified in Phase 1.
2. **Re-Score Based on Implementation**: Adjust scores based on actual code inspection.
3. **Identify Missing Files**: Discover files not found in metadata search but needed to address the intent.
4. **Filter Out False Positives**: Remove candidates with score < 0.7 after code validation.

### Phase 3: Return Validated Results
- Only include candidates with score > 0.7
- Provide reasoning from code inspection
- Include missing files identified
- Provide keyword effectiveness assessment
- Document validation methodology

## Scoring Guidelines

When assigning scores to candidates, use these numeric ranges:

- **0.9-1.0**: Confirmed source of truth (constant/config defined here)
  - Contains the primary logic or constants
  - Directly implements the feature
  - Is the "source of truth" for the intent

- **0.7-0.8**: Direct usage confirmed in code
  - Calls the core implementation
  - Provides CLI or API interface
  - Displays or observes the behavior

- **0.4-0.6**: Indirect reference or partial match
  - Only reads or displays data
  - Performs cleanup or maintenance
  - Indirectly related to the intent

- **<0.4**: False positive or tangential
  - Wrong file type (e.g., database models instead of business logic)
  - Unrelated functionality
  - No semantic alignment with intent

## Critical Missing File Guidelines

When identifying critical missing files:

1. **Look for Configuration Files**: Settings, config, constants files
2. **Look for Source of Truth**: Where the default values are defined
3. **Look for Core Constants**: Named constants that control behavior
4. **Look for Initialization Code**: Where defaults are set at startup
5. **Look for Documentation**: README, ARCHITECTURE files that might mention the file

If you find a critical missing file:
- Include it in the `missing_files` section
- Give it a high score (0.9-1.0)
- Explain why it's critical
- Provide the exact location (file path and line number)
- Tell the user what to do with it

## Keyword Assessment Guidelines

When assessing keyword effectiveness:

1. **High Effectiveness**: Keyword found relevant files with high precision
   - Most matches were truly relevant
   - Few false positives
   - Good signal-to-noise ratio

2. **Medium Effectiveness**: Keyword found some relevant files but also noise
   - Mix of relevant and irrelevant matches
   - Moderate false positive rate
   - Some signal but also noise

3. **Low Effectiveness**: Keyword failed to find relevant files or found mostly noise
   - Many false positives
   - Missed critical files
   - Poor signal-to-noise ratio

For each keyword, provide:
- Why it worked or didn't work
- Example matches (with file paths and keyword descriptions)
- What went wrong (if effectiveness is Low)
- How to improve it

## Intent Anchoring

- Your entire analysis must be anchored to the original intent
- Do not expand scope beyond what the intent explicitly requests
- If you discover related but out-of-scope files, note them but do not include them
- Changes must be traceable back to validated discovery

## Type Reminder Table

Use this table to verify field types before output:

| Field | Type | Example |
|-------|------|---------|
| `score` | `number` (0.0-1.0) | `0.95` |
| `matches` | `string[]` (array of file paths) | `["pkg/settings/settings.go"]` |
| `rating` | `string` (enum) | `"HIGH"`, `"MEDIUM"`, `"LOW"` |
| `total_candidates_found` | `number` | `23` |
| `top_candidates_returned` | `number` | `8` |
| `confirmed_patterns` | `string[]` (array) | `["const DefaultContractTTL = 4"]` |
| `keywords` | `string[]` (array) | `["settings", "configuration"]` |

⚠️ **CRITICAL**: `matches` must be an array of file paths, NOT a count. `rating` must be exactly "HIGH", "MEDIUM", or "LOW" - no other values are valid.

## Pre-Output Validation Checklist

Before outputting JSON, verify:

- [ ] No `relevance_level` field exists in any candidate object
- [ ] Every `matches` field is a string array (not an integer, not a string)
- [ ] Every `rating` value is exactly "HIGH", "MEDIUM", or "LOW"
- [ ] `discovery_log` contains both `total_candidates_found` AND `top_candidates_returned`
- [ ] `confirmed_patterns` is a plural array (not `confirmed_pattern`)
- [ ] `keywords` and `parent_keywords` are arrays, not strings
- [ ] Output is raw JSON - no ```json fences, no explanatory text
- [ ] All field names match the schema exactly (e.g., `total_candidates_found`, not "total candidates evaluated")
