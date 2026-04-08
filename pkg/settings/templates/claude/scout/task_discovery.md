# Scout Discovery Task

## Your Intent

{{.Intent}}

## Working Directories

{{.Workdirs}}

## Reference Files

{{.RefFiles}}

{{if .TurnHistoryExists}}
## Previous Turns Context

The following previous turns provide context for this discovery session:

```json
{{.TurnHistoryJSON}}
```

**How to use this context:**
- Review how the user's intent has evolved across turns
- Learn from previous keyword selections (what worked, what didn't)
- Build upon successful strategies from previous turns
- Avoid repeating work that was already done
- Identify patterns in what was found vs. what was missed

**Key insights to extract:**
- Which keywords were most effective?
- Did previous turns miss obvious candidates?
- Are there new keywords discovered in previous results?
- How has the user refined their intent over time?
{{else}}
## Previous Turns Context

**This is the first turn** - there are no previous turns to provide context.

Proceed with fresh discovery using the Intelligence Funnel strategy.
{{end}}

## Your Task

Use the Intelligence Funnel strategy to discover relevant files:

1. **Curate Intent Keywords**: Extract 2-7 keywords from the intent
2. **The Pivot**: Use `gsc insights` to check keyword volume
3. **Metadata Filtering**: Use `gsc query` to understand purpose
4. **Score Candidates**: Rate relevance based on metadata

{{if .TurnHistoryExists}}
**Note**: Consider previous turn results when selecting keywords and scoring candidates.
{{end}}

## Output Format

Return ONLY valid JSON (no additional text):
