<!--
Component: Scout Verification Task Prompt
Block-UUID: 6ca20d04-52c4-4c7d-b812-eac5486fa1b8
Parent-UUID: N/A
Version: 1.0.0
Description: Task prompt for Scout verification turns. Includes turn-history context for reviewing discovery results.
Language: Markdown
Created-at: 2026-04-08T16:41:00.000Z
Authors: GLM-4.7 (v1.0.0)
-->


# Scout Verification Task

## Your Intent

{{.Intent}}

{{if .TurnHistoryExists}}
## Previous Discovery Context

The following discovery turn provides the candidates to verify:

```json
{{.TurnHistoryJSON}}
```

**Your task:**
- Review each candidate from the last discovery turn
- Read their code to verify relevance to the original intent
- Re-score based on actual implementation (0.0-1.0)
- Provide detailed reasoning for score changes
- Identify false positives (score = 0.0)
- Extract keyword effectiveness assessment

**Keyword Assessment:**
For each keyword from the discovery intent:
- Rate effectiveness (High/Medium/Low)
- Explain why it worked/didn't work
- List example matches
- Identify new keywords discovered in verified files
- Provide recommendations for future discovery turns
{{else}}
## Previous Discovery Context

**No previous discovery context available** - cannot proceed with verification.

Please run a discovery turn first to generate candidates for verification.
{{end}}

## Your Task

Verify the candidates by:
1. Reading their code
2. Assessing relevance to the original intent
3. Re-scoring based on actual implementation
4. Providing detailed reasoning

## Output Format

Return ONLY valid JSON (no additional text):
