<!--
Component: Discovery Response Format Specification
Block-UUID: 3a8b9c1d-2e3f-4a5b-6c7d-8e9f0a1b2c3d
Parent-UUID: fc928c41-8f8e-41d6-817f-1945d4252f99
Version: 1.4.0
Description: Exact JSON schema and validation rules for discovery turn responses. Read by the correction turn AI to map a malformed response to the expected format. Added total_found field to Required JSON Structure and Field Specifications. Added cross-reference warning for schema authority and labeled schema examples as documentation only to prevent AI from including code fences in output. Added discovery_mode and brain_effectiveness fields to support hybrid discovery strategy. Added succinct_natural_language_response field to support AI-generated natural language summaries that directly answer user intents.
Language: Markdown
Created-at: 2026-04-26T18:54:09.609Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
-->


⚠️ **SCHEMA AUTHORITY WARNING**: This document is the single source of truth for JSON structure and field types. If any instruction in other documents (e.g., `discovery.md`) appears to conflict with this specification, this specification takes precedence.

# Discovery Response Format Specification

This document defines the exact JSON schema that MUST be returned by all discovery turns.

## Required JSON Structure

**[DOCUMENTATION ONLY - The examples below use code fences for readability. Your actual output must be raw JSON without any markdown fences or explanatory text.]**

```json
{
  "status": "complete",
  "succinct_natural_language_response": "The variable MAX_FILE_SIZE is declared on line 42 of internal/claude/config.go",
  "total_found": 12,
  "discovery_mode": "experts",
  "brain_effectiveness": {
    "overall_score": 0.85,
    "brains": [
      {
        "name": "code-intent",
        "score": 0.85,
        "fields_used": ["keywords", "purpose"],
        "fields_missing": [],
        "feedback": "Brain provided excellent keyword matches and purpose descriptions"
      }
    ]
  },
  "candidates": [
    {
      "workdir_id": 1,
      "workdir_name": "string",
      "file_path": "string",
      "score": 0.95,
      "reasoning": "string",
      "metadata": {
        "purpose": "string",
        "keywords": ["string", "string"],
        "parent_keywords": ["string", "string"]
      },
      "code_validation": {
        "confirmed_patterns": ["string", "string"],
        "implementation_details": "string"
      }
    }
  ],
  "missing_files": [
    {
      "file_path": "string",
      "score": 0.95,
      "reasoning": "string",
      "code_validation": {
        "confirmed_patterns": ["string"],
        "implementation_details": "string"
      }
    }
  ],
  "keyword_assessment": {
    "discovery_keywords": ["string", "string"],
    "effectiveness": {
      "keyword_name": {
        "rating": "HIGH",
        "explanation": "string",
        "matches": ["file1.go", "file2.go"]
      }
    }
  },
  "discovery_log": {
    "intent_keywords": ["string", "string"],
    "pivot_checks": ["string", "string"],
    "methodology": "string",
    "total_candidates_found": 10,
    "top_candidates_returned": 5,
    "validation_method": "string"
  },
  "coverage": "string"
}
```

## Field Specifications

### Root Level
- `status` (string, required): `"complete"`, `"out_of_scope"`, or `"failed"`
- `succinct_natural_language_response` (string, optional): A one-to-three sentence direct answer to the user's intent in plain language. Should name specific variables, line numbers, file paths, or values discovered. Omit if no concise answer is possible.
- `total_found` (integer, required): Total number of candidates found before top-N filtering (equivalent to `discovery_log.total_candidates_found`)
- `discovery_mode` (string, required): `"experts"` or `"generic"`
  - `"experts"`: Used when `.gitsense/experts-context.md` exists and was read
  - `"generic"`: Used when no experts context is available (traditional tools only)
- `brain_effectiveness` (object, optional): Required when `discovery_mode == "experts"`, must be absent when `discovery_mode == "generic"`
  - `overall_score` (float, required): Overall effectiveness score `0.0`-`1.0`
  - `brains` (array, required): Array of brain effectiveness entries
    - `name` (string, required): Brain database name
    - `score` (float, required): Effectiveness score `0.0`-`1.0`
    - `fields_used` (array of strings, required): Fields that were useful
    - `fields_missing` (array of strings, required): Fields that were missing
    - `feedback` (string, required): Qualitative feedback
- `candidates` (array, required): Array of candidate objects; may be empty
- `missing_files` (array, optional): Array of missing file objects
- `keyword_assessment` (object, optional): Keyword effectiveness analysis
- `discovery_log` (object, optional): Discovery methodology log
- `coverage` (string, optional): Coverage description

### Candidate Object
- `workdir_id` (integer, required): Working directory ID
- `workdir_name` (string, required): Working directory name
- `file_path` (string, required): File path relative to workdir
- `score` (float, required): Relevance score `0.0`-`1.0`
- `reasoning` (string, required): Explanation of relevance
- `metadata` (object, required): Brain metadata
  - `purpose` (string, required): File purpose description
  - `keywords` (array of strings, required): **MUST be an array, not a string**
  - `parent_keywords` (array of strings, required): **MUST be an array, not a string**
- `code_validation` (object, optional): Code inspection results
  - `confirmed_patterns` (array of strings, optional): Patterns confirmed in source
  - `implementation_details` (string, optional): Specific implementation notes

### Missing File Object
- `file_path` (string, required)
- `score` (float, required)
- `reasoning` (string, required)
- `code_validation` (object, optional): Same structure as candidate `code_validation`

### Keyword Assessment Object
- `discovery_keywords` (array of strings, required)
- `effectiveness` (object, required): Map of keyword name → effectiveness
  - `rating` (string, required): `"HIGH"`, `"MEDIUM"`, or `"LOW"`
  - `explanation` (string, required)
  - `matches` (array of strings, required): File paths - **MUST be an array, not a count**

### Discovery Log Object
- `intent_keywords` (array of strings, required)
- `pivot_checks` (array of strings, required)
- `methodology` (string, required)
- `total_candidates_found` (integer, required)
- `top_candidates_returned` (integer, required)
- `validation_method` (string, required)

## Common Format Errors to Avoid

### ❌ WRONG - String representation of an array
```json
"keywords": "[\"keyword1\", \"keyword2\"]"
```

### ✅ CORRECT - Actual array
```json
"keywords": ["keyword1", "keyword2"]
```

---

### ❌ WRONG - Integer count instead of array
```json
"effectiveness": {
  "contract": {
    "rating": "HIGH",
    "matched_files": 54,
    "relevance": "Excellent"
  }
}
```

### ✅ CORRECT - Array of file paths with correct field name
```json
"effectiveness": {
  "contract": {
    "rating": "HIGH",
    "explanation": "Found all contract-related files",
    "matches": ["pkg/settings/settings.go", "internal/contract/manager.go"]
  }
}
```

---

### ❌ WRONG - Extra fields not in schema
```json
{
  "status": "complete",
  "relevance_level": "HIGH",
  "confidence_level": "VERY HIGH",
  "actionable_recommendations": { ... }
}
```

### ✅ CORRECT - Only schema-defined fields
```json
{
  "status": "complete",
  "candidates": [ ... ]
}
```

---

### ❌ WRONG - brain_effectiveness in generic mode
```json
{
  "discovery_mode": "generic",
  "brain_effectiveness": { ... }
}
```

### ✅ CORRECT - brain_effectiveness only in experts mode
```json
{
  "discovery_mode": "experts",
  "brain_effectiveness": {
    "overall_score": 0.85,
    "brains": [ ... ]
  }
}
```

---

### ❌ WRONG - missing brain_effectiveness in experts mode
```json
{
  "discovery_mode": "experts"
}
```

### ✅ CORRECT - brain_effectiveness required in experts mode
```json
{
  "discovery_mode": "experts",
  "brain_effectiveness": {
    "overall_score": 0.85,
    "brains": [ ... ]
  }
}
```

---

### ❌ WRONG - Empty string for succinct_natural_language_response
```json
{
  "succinct_natural_language_response": ""
}
```

### ✅ CORRECT - Omit the field if no concise answer is possible
```json
{
  "status": "complete",
  "candidates": [ ... ]
}
```

### ✅ CORRECT - Provide a real answer when available
```json
{
  "succinct_natural_language_response": "The variable MAX_FILE_SIZE is declared on line 42 of internal/claude/config.go"
}
```

## Validation Checklist

Before returning your response, verify:

- [ ] All required fields are present
- [ ] `total_found` is included and matches `discovery_log.total_candidates_found`
- [ ] `discovery_mode` is either `"experts"` or `"generic"`
- [ ] `brain_effectiveness` is present when `discovery_mode == "experts"`
- [ ] `brain_effectiveness` is absent when `discovery_mode == "generic"`
- [ ] `brain_effectiveness.overall_score` is in range `[0.0, 1.0]`
- [ ] `brain_effectiveness.brains` contains at least one entry
- [ ] Each brain entry has `name`, `score`, `fields_used`, `fields_missing`, and `feedback`
- [ ] `keywords` and `parent_keywords` are JSON arrays, not strings
- [ ] `matches` in `effectiveness` is a JSON array of strings, not a numeric count
- [ ] No extra fields outside the schema are present
- [ ] `status` is one of: `"complete"`, `"out_of_scope"`, `"failed"` (informational, not validated by correction loop)
- [ ] `rating` in each effectiveness entry is one of: `"HIGH"`, `"MEDIUM"`, `"LOW"`
- [ ] `score` values are floats in the range `0.0`-`1.0`
- [ ] JSON is syntactically valid and fully parseable
- [ ] `succinct_natural_language_response`, if present, is a non-empty string and directly answers the intent

## Example Valid Response (Experts Mode)

**[DOCUMENTATION ONLY - This example uses code fences for readability. Your actual output must be raw JSON without any markdown fences or explanatory text.]**

```json
{
  "status": "complete",
  "succinct_natural_language_response": "The DefaultContractTTL constant is defined on line 91 of pkg/settings/settings.go with a value of 4.",
  "total_found": 23,
  "discovery_mode": "experts",
  "brain_effectiveness": {
    "overall_score": 0.85,
    "brains": [
      {
        "name": "code-intent",
        "score": 0.85,
        "fields_used": ["keywords", "purpose"],
        "fields_missing": [],
        "feedback": "Brain provided excellent keyword matches and purpose descriptions"
      }
    ]
  },
  "candidates": [
    {
      "workdir_id": 1,
      "workdir_name": "gsc-cli",
      "file_path": "pkg/settings/settings.go",
      "score": 1.0,
      "reasoning": "Contains the DefaultContractTTL constant on line 91.",
      "metadata": {
        "purpose": "Application constants and configuration loading logic.",
        "keywords": ["define-constants", "load-configuration"],
        "parent_keywords": ["settings", "configuration"]
      },
      "code_validation": {
        "confirmed_patterns": ["const DefaultContractTTL = 4"],
        "implementation_details": "Line 91 defines the constant referenced by CreateContract in manager.go."
      }
    }
  ],
  "missing_files": [],
  "keyword_assessment": {
    "discovery_keywords": ["contract", "default"],
    "effectiveness": {
      "contract": {
        "rating": "HIGH",
        "explanation": "Matched all contract-related files with good precision.",
        "matches": ["pkg/settings/settings.go", "internal/contract/manager.go"]
      },
      "default": {
        "rating": "MEDIUM",
        "explanation": "Matched settings file but also returned unrelated files.",
        "matches": ["pkg/settings/settings.go"]
      }
    }
  },
  "discovery_log": {
    "intent_keywords": ["contract", "default", "ttl"],
    "pivot_checks": [
      "Initial keyword 'contract' returned 54 files - within acceptable range",
      "Refined to 'contract' AND 'expiration' returned 8 files (good volume)",
      "Proceeded to metadata filtering and code validation"
    ],
    "methodology": "Metadata search followed by targeted code validation of top 8 candidates. Stopped early after finding 0.95 score file.",
    "total_candidates_found": 23,
    "top_candidates_returned": 8,
    "validation_method": "Source code inspection of top-ranked candidates."
  },
  "coverage": "1 working directory scanned (gsc-cli)"
}
```

## Example Valid Response (Generic Mode)

**[DOCUMENTATION ONLY - This example uses code fences for readability. Your actual output must be raw JSON without any markdown fences or explanatory text.]**

```json
{
  "status": "complete",
  "succinct_natural_language_response": "The MAX_FILE_SIZE variable is declared in internal/claude/config.go at line 42 within the defaultConfig struct.",
  "total_found": 5,
  "discovery_mode": "generic",
  "candidates": [
    {
      "workdir_id": 1,
      "workdir_name": "gsc-cli",
      "file_path": "pkg/settings/settings.go",
      "score": 0.95,
      "reasoning": "Found contract-related constants using grep search.",
      "metadata": {
        "purpose": "Application constants and configuration loading logic.",
        "keywords": ["define-constants", "load-configuration"],
        "parent_keywords": ["settings", "configuration"]
      },
      "code_validation": {
        "confirmed_patterns": ["const DefaultContractTTL = 4"],
        "implementation_details": "Line 91 defines the constant."
      }
    }
  ],
  "missing_files": [],
  "keyword_assessment": {
    "discovery_keywords": ["contract", "default"],
    "effectiveness": {
      "contract": {
        "rating": "MEDIUM",
        "explanation": "Grep search found relevant files but with lower precision than brain search.",
        "matches": ["pkg/settings/settings.go"]
      }
    }
  },
  "discovery_log": {
    "intent_keywords": ["contract", "default"],
    "pivot_checks": [
      "Grep search for 'contract' returned 5 files",
      "Proceeded to code validation of all candidates"
    ],
    "methodology": "Traditional grep and find search followed by code validation.",
    "total_candidates_found": 5,
    "top_candidates_returned": 5,
    "validation_method": "Source code inspection of all candidates."
  },
  "coverage": "1 working directory scanned (gsc-cli)"
}
```
