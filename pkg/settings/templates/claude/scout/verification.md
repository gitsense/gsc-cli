<!--
Component: Scout Verification Methodology
Block-UUID: b8fab98f-bc2d-4c71-95d6-e0e7263a15b5
Parent-UUID: 60a8f724-9261-4cd7-b2d3-7526614eb19f
Version: 2.1.0
Description: Detailed verification methodology for Scout verification. Renamed from turn_2_verification.md to remove turn-specific naming.
Language: Markdown
Created-at: 2026-04-03T03:45:00.000Z
Authors: GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
-->


## Verification Execution: Step-by-Step

### Step 1: Review Discovery Results
Review the candidates discovered in the last discovery turn:
- Read the candidates from the discovery phase
- Understand the original scores and reasoning
- Identify which candidates need verification

### Step 2: Read Code for Verification
For each candidate that needs verification:
- Read the actual code content
- Understand the implementation details
- Verify the semantic fit with the user's intent

### Step 3: Re-score Candidates
Based on code inspection:
- **Promote**: Increase score if code confirms relevance
- **Demote**: Decrease score if code reveals false positive
- **Remove**: Set score to 0.0 if code shows no relevance

### Step 4: Update Reasoning
Provide detailed reasoning for each score change:
- Explain what you found in the code
- Why the score was adjusted
- Any implementation details that support the decision

---

## Output Format

Return ONLY valid JSON (no additional text):

```json
{
  "verified_candidates": [
    {
      "file_path": "internal/scout/manager.go",
      "workdir_id": 1,
      "original_score": 0.95,
      "verified_score": 0.98,
      "reason": "Code inspection confirms this is the core Scout manager. Implements session lifecycle, subprocess spawning, and stream processing. Directly relevant to intent."
    }
  ],
  "summary": {
    "total_verified": 10,
    "candidates_promoted": 2,
    "candidates_demoted": 1,
    "candidates_removed": 1,
    "average_verified_score": 0.85,
    "top_candidates_count": 5
  }
}
```
