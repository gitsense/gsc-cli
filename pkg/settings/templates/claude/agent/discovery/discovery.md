<!--
Component: Scout Discovery Methodology
Block-UUID: 184e7c80-72da-45d7-a1aa-1d5b421f40bf
Parent-UUID: 53b73eb2-4bb2-4c2c-82df-63f535caa570
Version: 7.0.0
Description: Discovery methodology for Smart Discovery (find + validate). Merged validation guidelines into discovery methodology. Includes relevance levels, critical missing file guidelines, keyword assessment, and actionable recommendations.
Language: Markdown
Created-at: 2026-04-19T01:11:00.000Z
Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v3.0.0), GLM-4.7 (v4.0.0), GLM-4.7 (v5.0.0), GLM-4.7 (v5.1.0), GLM-4.7 (v6.0.0), GLM-4.7 (v7.0.0)
-->


## Discovery Best Practices

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
- Provide actionable recommendations
- Document validation methodology

## Relevance Level Guidelines

When assigning relevance levels, use these criteria:

- **HIGHLY RELEVANT**: Core implementation, source of truth, directly controls the behavior
  - Contains the primary logic or constants
  - Directly implements the feature
  - Is the "source of truth" for the intent

- **PARTIALLY RELEVANT**: Related but not central, wrapper code, or secondary functionality
  - Calls the core implementation
  - Provides CLI or API interface
  - Displays or observes the behavior

- **WEAKLY RELEVANT**: Tangentially related, only observes or displays the behavior
  - Only reads or displays data
  - Performs cleanup or maintenance
  - Indirectly related to the intent

- **FALSE POSITIVE**: Not relevant to the intent, wrong file type, or unrelated functionality
  - Wrong file type (e.g., database models instead of business logic)
  - Unrelated functionality
  - Only contains data structures

- **IRRELEVANT**: No connection to the intent whatsoever
  - Completely unrelated code
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

## Actionable Recommendations Guidelines

When providing recommendations:

1. **Prioritize Changes**: Use PRIMARY, SECONDARY, TERTIARY priority levels
2. **Be Specific**: Provide exact file paths and line numbers
3. **Explain Why**: Tell the user why each change is needed
4. **Be Actionable**: Give concrete next steps
5. **Think Strategically**: Provide advice for future discovery turns

Include:
- Files to modify (with priority)
- What changes to make
- Why these changes are needed
- How to improve future searches

## Intent Anchoring

- Your entire analysis must be anchored to the original intent
- Do not expand scope beyond what the intent explicitly requests
- If you discover related but out-of-scope files, note them but do not include them
- Changes must be traceable back to validated discovery
