<!--
Component: Scout Verification Task Prompt
Block-UUID: ab9a9a5c-e1c3-43cf-9e96-e4da8548ad86
Parent-UUID: 6ca20d04-52c4-4c7d-b812-eac5486fa1b8
Version: 1.1.0
Description: Task prompt for Scout verification turns. Includes turn-history context for reviewing discovery results.
Language: Markdown
Created-at: 2026-04-12T03:21:06.830Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
-->


# Scout Verification Task

## Your Intent

{{.Intent}}

{{if .TurnHistoryExists}}
## Previous Discovery Context

The following discovery turn provides context from previous turns:

\```json
{{.TurnHistoryJSON}}
\```

{{if .HasReviewFiles}}
**User Selection:**
The user has selected the following files for verification review:

\```json
{{.ReviewFilesJSON}}
\```

**Your task:**
- Review ONLY the files listed in "User Selection" above (full file paths)
- Use the discovery context to understand the original intent and methodology
- Read the code for each selected file to verify relevance
- Re-score based on actual implementation (0.0-1.0)
- Provide detailed reasoning for score changes
- Identify false positives (score = 0.0)
- Extract keyword effectiveness assessment
{{else}}
**Your task:**
- Review each candidate from the last discovery turn
- Read their code to verify relevance to the original intent
- Re-score based on actual implementation (0.0-1.0)
- Provide detailed reasoning for score changes
- Identify false positives (score = 0.0)
- Extract keyword effectiveness assessment
{{end}}
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
