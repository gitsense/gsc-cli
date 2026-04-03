<!--
Component: Scout Discovery Methodology
Block-UUID: eaa7c5a5-dd75-493d-b751-8259b0d683c7
Parent-UUID: ad2bdd6e-29ba-4276-99f5-808bbc56a1ef
Version: 2.0.0
Description: Detailed discovery methodology for Scout Turn 1.
Language: Markdown
Created-at: 2026-04-03T03:45:00.000Z
Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v2.0.0)
-->


## Discovery Execution: Step-by-Step

### Step 1: Build the Mental Map
Review the codebase-overview.json provided above:
- **Top keywords**: What are the dominant domains?
- **Parent keywords**: What are the high-level functional categories?
- **File types**: What languages and frameworks are present?

### Step 2: Seed Keywords from Context
If REFERENCE_FILES are provided (README, ARCHITECTURE, etc.):
- Extract technical terms, library names, or architectural patterns mentioned
- These become your initial search anchors

### Step 3: Curate Intent Keywords
Based on the user's intent and the mental map:
- Extract 5-10 keywords aligned with the repository's actual taxonomy
- Use the top keywords from codebase-overview as reference
- Example: Intent "Find contract renewal files" → Keywords: `contract`, `renewal`, `lifecycle`, `management`

### Step 4: The Pivot - Check File Volumes
For each keyword, use gsc insights to check how many files it matches:

```bash
gsc insights --db tiny-overview --fields keywords --limit 1000 \
  --filter "keywords in (*contract*,*renewal*)" --format json
```

Analyze the results:
- **If >100 files**: Too broad. Add more specific keywords or combine with AND filters
- **If 5-50 files**: Sweet spot. Proceed to metadata filtering
- **If <5 files**: Too narrow. Broaden keywords or try wildcards

### Step 5: Metadata-First Filtering
Once you have 5-50 file clusters per keyword, use gsc query:

```bash
gsc query --db tiny-overview --filter "keywords in (contract,renewal)" \
  --fields purpose,keywords,parent_keywords --format json
```

Review each result:
- Look at the **purpose** field-does it match the intent semantically?
- Check **keywords**-are they relevant to the search?
- Consider **parent keywords**-what domain does this file belong to?
- **Discard false positives** where the text matches but semantic purpose doesn't

### Step 6: Score Candidates
Based on metadata alone (DO NOT read code):

- **0.9-1.0**: Direct match on purpose and keywords (e.g., "Handles contract renewal")
- **0.7-0.8**: Strong semantic fit (e.g., "Manages lifecycle events" for renewal query)
- **0.4-0.6**: Possibly relevant but supporting/secondary
- **<0.4**: False positive-discard

Explain your reasoning: Why does this file's purpose, keywords, and parent keywords match the intent?

### Step 7: Optional - Temporal Filtering
If the intent involves recency (latest, recent, newest):

```bash
# First discover the latest date
gsc insights --db tiny-overview --fields dates | sort | tail -1
# Then filter by that date
gsc query --db tiny-overview --filter "dates in (2026-03-15|*)" \
  --fields purpose --format json
```

---

## Output Format

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
    "intent_keywords": ["scout", "discovery", "session"],
    "pivot_checks": [
      "Initial keyword 'scout' returned 150 files (>100, too broad)",
      "Refined to 'scout' AND 'session' returned 23 files (5-50, sweet spot)",
      "No temporal filtering needed for this intent"
    ],
    "methodology": "Used gsc insights with wildcards to find domain clusters, then gsc query to validate semantic fit. Discarded false positives. No code inspection performed.",
    "total_candidates_found": 15,
    "top_candidates_returned": 10
  },
  "coverage": "2 working directories scanned, all keywords cross-referenced"
}
```
