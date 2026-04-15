<!--
Component: Scout Discovery Methodology
Block-UUID: 184e7c80-72da-45d7-a1aa-1d5b421f40bf
Parent-UUID: 53b73eb2-4bb2-4c2c-82df-63f535caa570
Version: 5.1.0
Description: Detailed discovery methodology for Scout discovery. Removed codebase overview step, improved fail-fast logic to evaluate semantic alignment, updated keyword extraction to 2-5 keywords, added wildcard requirement, mandated gsc grep over Unix grep, relabeled Step 4 to 'Understand Purpose', and clarified workflow to prevent infinite loops.
Language: Markdown
Created-at: 2026-04-05T03:40:15.123Z
Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v3.0.0), GLM-4.7 (v4.0.0), GLM-4.7 (v5.0.0), GLM-4.7 (v5.1.0)
-->


## Discovery Execution: Step-by-Step

### Step 1: Seed Keywords from Context
If REFERENCE_FILES are provided (README, ARCHITECTURE, etc.):
- Extract technical terms, library names, or architectural patterns mentioned
- These become your initial search anchors

### Step 2: Curate Intent Keywords
Based on the user's intent:
- Extract **2-5 keywords** aligned with the repository's actual taxonomy
- Focus on **quality and relevance** over quantity
- **IMPORTANT**: Always wrap keywords with wildcards (`*keyword*`) when searching
  - This catches compound terms like `contract-renewal` when searching for `contract`
  - Example: `*contract*` matches `contract`, `contract-renewal`, `contract-expiration`
- **Common terms are useful**: Keywords like `time`, `create`, `default` can be highly relevant
  - The metadata (purpose, keywords, parent_keywords) will help filter relevance
  - Example: `*time*` will match `default-time`, `expiration-time`, `build-time`
  - Use the metadata to determine which matches are relevant to the intent
- Example: Intent "Find contract renewal files" → Keywords: `*contract*`, `*renewal*`, `*lifecycle*`
- Example: Intent "Change default contract expiration time" → Keywords: `*contract*`, `*expiration*`, `*time*`

**Important**: If you can't find 5 relevant keywords, don't force it. 2-3 highly relevant keywords are better than 5 mixed keywords.

### Step 3: The Pivot - Evaluate Keyword Alignment (Fail-Fast Decision)

Run an initial `gsc insights` query to discover what keywords and parent_keywords match:

```bash
gsc insights --db code-intent --fields keywords,parent_keywords \
  --filter "keywords in (*contract*,*renewal*)" \
  --filter "parent_keywords in (*contract*,*renewal*)" \
  --limit 100 --format json
```

**Note the wildcard syntax**: `*contract*` matches `contract`, `contract-renewal`, `contract-expiration`, etc.

**Evaluate the RESULTS (not just the count):**

**SUCCESS - Keywords are Promising:**
- You found keywords/parent_keywords that **semantically align** with the user's intent
- Examples:
  - Intent: "contract expiration" → Found `contract`, `expiration`, `default-time` ✓
  - Intent: "session management" → Found `session`, `session-storage`, `session-timeout` ✓
- **Action**: Proceed to Step 4 (Understand Purpose)
- **Note**: Even if some keywords are broad (e.g., `time`), if you found relevant variations (e.g., `default-time`), the keywords are promising

**FAILURE - Keywords are Not Promising:**
- The matched keywords/parent_keywords **do not align** with the user's intent
- Examples:
  - Intent: "contract expiration" → Found only `build-time`, `compile-time`, `run-time` ✗
  - Intent: "session management" → Found only `time`, `date`, `timestamp` ✗
- **Action**: Return fail-fast response with suggestions

**Fail-Fast Response Format:**
```json
{
  "status": "failed",
  "reason": "Keywords not promising",
  "details": "The keywords 'contract', 'expiration', and 'time' returned matches, but none align with the intent. Found keywords: 'build-time', 'compile-time', 'run-time' (all related to build process, not contract expiration).",
  "suggestions": [
    "Can you provide more details about what you're trying to accomplish?",
    "What specific functionality are you looking for?",
    "Can you describe the problem you're trying to solve in more detail?",
    "What is the context or use case for this request?",
    "Are there any specific files, functions, or components you're aware of?"
  ]
}
```

### Step 4: Understand Purpose

Once you have promising keywords, use `gsc query` to get detailed metadata (including purpose):

```bash
gsc query --db code-intent --filter "keywords in (contract,renewal)" \
  --fields purpose,keywords,parent_keywords --format json
```

**Why this step matters:**
- **Purpose reveals true relevance**: A semi-promising keyword might be a perfect match if the purpose talks about creating contracts
- **Sibling keywords**: The purpose might mention related keywords we didn't think of (e.g., "lifecycle", "creation")
- **Unique lingo**: The purpose might use domain-specific terminology that better describes the intent
- **Efficiency**: We only get purpose for files that passed the keyword alignment check, not for 1000+ matches

**Review each result:**
- Look at the **purpose** field - does it match the intent semantically?
- Check **keywords** - are they relevant to the search?
- Consider **parent keywords** - what domain does this file belong to?
- **Discard false positives** where the text matches but semantic purpose doesn't

**If stronger keywords are discovered in the purpose:**
- Use them to **score the candidates you already have** (do NOT repeat Step 3)
- Example: Purpose mentions "contract lifecycle" → Use `lifecycle` as a scoring factor
- Do NOT go back to Step 3 to search for new files with these keywords

### Step 5: Score Candidates

**CRITICAL: Score based on metadata ONLY. DO NOT read code.**

- **0.9-1.0**: Direct match on purpose and keywords (ideal result)
- **0.7-0.8**: Strong semantic fit
- **0.4-0.6**: Possibly relevant but supporting/secondary
- **<0.4**: False positive - discard

**Note**: Small result sets (1-5 files) are excellent - this indicates high precision. Do NOT try to expand results unless they are clearly irrelevant.

**If you are still uncertain after reviewing purpose:**
- Use `gsc grep --summary --fields purpose,keywords --db code-intent --format json "search_term"` to search code content
- Example: Found `contract` and `expiration`, but not sure about `default-time` → Use `gsc grep --summary "default-time"` to verify
- This searches code content efficiently without reading entire files

### Step 6: Optional - Temporal Filtering

If the intent involves recency (latest, recent, newest):

```bash
# First discover the latest date
gsc insights --db code-intent --fields dates | sort | tail -1
# Then filter by that date
gsc query --db code-intent --filter "dates in (2026-03-15_*)" \
  --fields purpose --format json
```

### Step 7: Fallback - Explore Available Metadata

If keyword-based discovery yields unsatisfactory results, use `gsc brains` to explore all available metadata fields:

```bash
gsc brains code-intent --format json
```

This shows all available fields (e.g., `key_entities`, `mechanism`, `logic_constraints`) that you can use for alternative search strategies with `gsc query` or `gsc grep`.

---

## Best Practices

### Wildcard Usage
- **ALWAYS use wildcards** when searching keywords: `*keyword*`
- This catches compound terms (e.g., `*contract*` matches `contract-renewal`)
- This catches variations (e.g., `*expiration*` matches `token-expiration`)
- Example: `*contract*` is better than `contract`

### Keyword Selection
- **Common terms are useful**: `time`, `create`, `default` can be highly relevant
- **Let metadata filter**: Use purpose, keywords, and parent_keywords to determine relevance
- **Don't over-complicate**: Simple keywords are often better than forced compound terms
- 2-3 highly relevant keywords are better than 5-10 mixed keywords

### Fail-Fast Evaluation
- **Evaluate semantic alignment first**: Do the matched keywords relate to the intent?
- **File count is secondary**: 100 relevant keywords are better than 5 irrelevant ones
- **Ask for context**: If keywords don't align, ask the user for more details about their problem

### Workflow Discipline
- **Don't repeat Step 3**: Once you've passed the fail-fast decision, don't go back to keyword discovery
- **Use discovered keywords for scoring**: If the purpose reveals stronger keywords, use them to score existing candidates
- **Avoid infinite loops**: The workflow is linear - don't revisit earlier steps

---

## Tool Usage Rules

**FORBIDDEN TOOLS:**
- ❌ **NEVER use Unix `grep`** - it's slow and inefficient
- ❌ **NEVER use `jq`, `uniq`, `wc`, `find`, `ls`, `cat`, `less`, `more`**
- ❌ **NEVER use the Read tool during discovery**

**REQUIRED TOOLS:**
- ✅ **ALWAYS use `gsc grep`** for code content searches
- ✅ **ALWAYS use `gsc query`** for metadata searches
- ✅ **ALWAYS use `gsc insights`** for frequency/statistics queries

**Why `gsc grep` over Unix `grep`:**
- `gsc grep` searches code content AND enriches with metadata
- `gsc grep --summary` returns metadata only (no code snippets) - saves tokens
- `gsc grep` is optimized for the codebase structure
- Unix `grep` is slow and doesn't provide context

---

## Output Format

**Success Case:**
Return ONLY valid JSON (no additional text):

```json
{
  "candidates": [
    {
      "workdir_id": 1,
      "workdir_name": "gsc-cli",
      "file_path": "internal/scout/manager.go",
      "score": 0.95,
      "reasoning": "Directly implements Scout session management and orchestration. Purpose: 'Manages Scout session lifecycle and discovery coordination.' Keywords match intent exactly: scout, session, manager.",
      "metadata": {
        "purpose": "Manages Scout session lifecycle and discovery coordination",
        "keywords": ["scout", "session", "manager", "discovery"],
        "parent_keywords": ["scout", "core"]
      }
    }
  ],
  "discovery_log": {
    "intent_keywords": ["*scout*", "*discovery*", "*session*"],
    "pivot_checks": [
      "Initial keyword '*scout*' returned 23 files (5-50, good volume)",
      "Refined to '*scout*' AND '*session*' returned 3 files (1-4, excellent precision)",
      "Keywords are promising: found 'scout', 'session', 'manager' - all semantically aligned with intent",
      "No temporal filtering needed for this intent"
    ],
    "methodology": "Used gsc insights with wildcards to evaluate keyword alignment, then gsc query to understand purpose and validate semantic fit. Discarded false positives. No code inspection performed.",
    "total_candidates_found": 3,
    "top_candidates_returned": 3
  },
  "coverage": "1 working directory scanned, all keywords cross-referenced"
}
```

**Do NOT include:**
- Headings like "Step 5: Score Candidates"
- Markdown code blocks (```json)
- Explanatory text before or after the JSON
- Any text outside the JSON object

**Your entire response must be parseable as valid JSON.**

**Fail-Fast Case:**
```json
{
  "status": "failed",
  "reason": "Keywords not promising",
  "details": "The keywords 'contract', 'expiration', and 'time' returned matches, but none align with the intent. Found keywords: 'build-time', 'compile-time', 'run-time' (all related to build process, not contract expiration).",
  "suggestions": [
    "Can you provide more details about what you're trying to accomplish?",
    "What specific functionality are you looking for?",
    "Can you describe the problem you're trying to solve in more detail?",
    "What is the context or use case for this request?",
    "Are there any specific files, functions, or components you're aware of?"
  ]
}
```
