<!--
Component: Scout Validation Task Prompt
Block-UUID: ab9a9a5c-e1c3-43cf-9e96-e4da8548ad86
Parent-UUID: 6ca20d04-52c4-4c7d-b812-eac5486fa1b8
Version: 2.0.0
Description: Task prompt for Scout validation turns. Updated to request rich validation format with critical missing files, keyword effectiveness assessment, and actionable recommendations.
Language: Markdown
Created-at: 2026-04-12T03:21:06.830Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v2.0.0)
-->


# Scout Validation Task

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
The user has selected the following files for validation review:

\```json
{{.ReviewFilesJSON}}
\```

**Your task:**
- Review ONLY the files listed in "User Selection" above (full file paths)
- Use the discovery context to understand the original intent and methodology
- Read the code for each selected file to validate relevance
- Re-score based on actual implementation (0.0-1.0)
- Provide detailed reasoning for score changes
- Identify false positives (score = 0.0)
- Identify critical missing files that discovery missed
- Extract keyword effectiveness assessment
- Provide actionable recommendations
{{else}}
**Your task:**
- Review each candidate from the last discovery turn
- Read their code to validate relevance to the original intent
- Re-score based on actual implementation (0.0-1.0)
- Provide detailed reasoning for score changes
- Identify false positives (score = 0.0)
- Identify critical missing files that discovery missed
- Extract keyword effectiveness assessment
- Provide actionable recommendations
{{end}}
{{else}}
## Previous Discovery Context

**No previous discovery context available** - cannot proceed with validation.

Please run a discovery turn first to generate candidates for validation.
{{end}}

## Your Task

Validate the candidates by:
1. Reading their code
2. Assessing relevance to the original intent
3. Re-scoring based on actual implementation
4. Providing detailed reasoning
5. Identifying critical missing files
6. Assessing keyword effectiveness
7. Providing actionable recommendations

## Output Format

Return ONLY valid JSON (no additional text) with the following structure:

\```json
{
  "validation_summary": {
    "session_intent": "The original intent for this session",
    "turn_number": 2,
    "total_candidates_reviewed": 7,
    "validated_candidates_count": 7,
    "critical_finding": "The most important discovery (e.g., missing critical file)"
  },
  "validated_candidates": [
    {
      "file_path": "path/to/file.go",
      "original_score": 0.95,
      "validated_score": 0.95,
      "relevance": "HIGHLY RELEVANT - Core lifecycle operations",
      "reasoning": "Detailed explanation of why this score was assigned",
      "code_validation": {
        "confirmed_patterns": [
          "Pattern 1 found in code",
          "Pattern 2 found in code"
        ],
        "missing_patterns": [
          "Pattern expected but not found"
        ],
        "implementation_details": "Specific line numbers and code snippets",
        "issues": [
          "Any problems or concerns"
        ]
      },
      "action_required": "What the user should do with this file"
    }
  ],
  "critical_missing_candidate": {
    "file_path": "path/to/missing/file.go",
    "score": 0.99,
    "relevance": "CRITICAL - Source of truth",
    "reasoning": "Why this file is critical and was missed",
    "code_validation": {
      "confirmed_pattern": "The key pattern found in this file"
    },
    "action_required": "What the user should do with this file"
  },
  "keyword_assessment": {
    "discovery_intent_keywords": ["*keyword1*", "*keyword2*"],
    "keyword_effectiveness": {
      "*keyword1*": {
        "effectiveness": "HIGH",
        "explanation": "Why this keyword worked well",
        "matched_files": 5,
        "example_matches": [
          "file1.go (keyword description)",
          "file2.go (keyword description)"
        ],
        "issue": "What went wrong (if effectiveness is LOW)"
      }
    },
    "new_keywords_discovered_in_code": {
      "new_keyword": {
        "found_in": "Where this keyword was found",
        "pattern": "What pattern represents this keyword",
        "relevance": "How relevant this keyword is"
      }
    },
    "keyword_recommendations": {
      "should_add": [
        "Keyword to add to future searches"
      ],
      "should_refine": [
        "Keyword to refine or remove"
      ],
      "future_discovery_strategy": "Strategic advice for future discovery"
    }
  },
  "summary_and_recommendations": {
    "files_to_modify_to_change_default_expiration": [
      {
        "priority": "PRIMARY",
        "file": "path/to/file.go",
        "line": 91,
        "change": "What change to make",
        "reason": "Why this change is needed"
      }
    ],
    "false_positives_identified": [
      "file1.go - Why this was a false positive",
      "file2.go - Why this was a false positive"
    ],
    "discovery_quality_assessment": "Overall evaluation of discovery performance",
    "verdict": "SUCCESS, MIXED, or FAILURE"
  }
}
\```

## Important Notes

- **Relevance Levels**: Use HIGHLY RELEVANT, PARTIALLY RELEVANT, WEAKLY RELEVANT, FALSE POSITIVE, or IRRELEVANT
- **Critical Missing Files**: If discovery missed a critical file, include it in the `critical_missing_candidate` section
- **Keyword Assessment**: Be specific about what worked and what didn't
- **Actionable Recommendations**: Provide concrete next steps for the user
- **No Additional Text**: Return ONLY the JSON, no explanations or markdown formatting
