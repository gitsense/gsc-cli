<!--
Component: Scout Verification Methodology
Block-UUID: b8fab98f-bc2d-4c71-95d6-e0e7263a15b5
Parent-UUID: 60a8f724-9261-4cd7-b2d3-7526614eb19f
Version: 3.0.0
Description: Detailed verification methodology for Scout verification. Updated to reflect rich verification format with critical missing files, keyword effectiveness assessment, and actionable recommendations.
Language: Markdown
Created-at: 2026-04-03T03:45:00.000Z
Authors: GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v3.0.0)
-->


## Verification Execution: Step-by-Step

### Step 1: Review Discovery Results
Review the candidates discovered in the last discovery turn:
- Read the candidates from the discovery phase
- Understand the original scores and reasoning
- Identify which candidates need verification
- Note the keywords used in discovery

### Step 2: Read Code for Verification
For each candidate that needs verification:
- Read the actual code content
- Understand the implementation details
- Verify the semantic fit with the user's intent
- Look for specific patterns, functions, or constants
- Identify line numbers where relevant code exists

### Step 3: Re-score Candidates
Based on code inspection:
- **Promote**: Increase score if code confirms relevance (0.7 → 0.9)
- **Demote**: Decrease score if code reveals false positive (0.8 → 0.3)
- **Remove**: Set score to 0.0 if code shows no relevance

### Step 4: Identify Critical Missing Files
Look for files that discovery missed but are essential to the intent:
- Configuration files (settings.go, config.go)
- Constants or default values
- Source of truth for the behavior
- Files that control the core functionality

### Step 5: Assess Keyword Effectiveness
For each keyword from the discovery intent:
- **Effectiveness Rating**: High/Medium/Low
- **Explanation**: Why it worked or didn't work
- **Example Matches**: Which files it actually found
- **Issues**: What went wrong (if effectiveness is Low)
- **New Keywords Discovered**: Terms found in code that weren't in original intent

### Step 6: Provide Actionable Recommendations
Give the user concrete next steps:
- Which files to modify (with priority levels)
- What changes to make (with line numbers)
- Why these changes are needed
- How to improve future discovery turns

### Step 7: Update Reasoning
Provide detailed reasoning for each score change:
- Explain what you found in the code
- Why the score was adjusted
- Any implementation details that support the decision
- Confirmed patterns and missing patterns

---

## Output Format

Return ONLY valid JSON (no additional text):

\```json
{
  "verification_summary": {
    "session_intent": "The original intent for this session",
    "turn_number": 2,
    "total_candidates_reviewed": 7,
    "verified_candidates_count": 7,
    "critical_finding": "The most important discovery (e.g., missing critical file)"
  },
  "verified_candidates": [
    {
      "file_path": "internal/contract/manager.go",
      "original_score": 0.95,
      "verified_score": 0.95,
      "relevance": "HIGHLY RELEVANT - Core lifecycle operations",
      "reasoning": "This file contains the CreateContract function which sets the default expiration time at line 73. Also contains RenewContract function for extending existing contracts.",
      "code_verification": {
        "confirmed_patterns": [
          "CreateContract function with TTL calculation",
          "RenewContract function for extending contracts",
          "Uses settings.DefaultContractTTL constant"
        ],
        "missing_patterns": [
          "No custom TTL parameter support"
        ],
        "implementation_details": "Line 73 directly uses settings.DefaultContractTTL to calculate ExpiresAt = now + 4 hours",
        "issues": []
      },
      "action_required": "Modify this file to accept custom TTL parameter or change the constant reference"
    }
  ],
  "critical_missing_candidate": {
    "file_path": "pkg/settings/settings.go",
    "score": 0.99,
    "relevance": "CRITICAL - Source of truth",
    "reasoning": "This file contains 'const DefaultContractTTL = 4' (line 91) which is THE PRIMARY CONSTANT that controls the default contract expiration time. To change the default from 4 hours, you would modify this constant.",
    "code_verification": {
      "confirmed_pattern": "const DefaultContractTTL = 4"
    },
    "action_required": "Change 'const DefaultContractTTL = 4' to desired hours (e.g., 8 for 8 hours)"
  },
  "keyword_assessment": {
    "discovery_intent_keywords": ["*contract*", "*expiration*", "*default*", "*time*"],
    "keyword_effectiveness": {
      "*contract*": {
        "effectiveness": "HIGH",
        "explanation": "Correctly identified all contract-related files",
        "matched_files": 7,
        "example_matches": [
          "internal/cli/contract/lifecycle.go (manage-contract-lifecycle keyword)",
          "internal/contract/manager.go (create-contract keyword)"
        ],
        "issue": ""
      },
      "*expiration*": {
        "effectiveness": "MEDIUM",
        "explanation": "Found files that CHECK expiration but missed the file that SETS defaults",
        "matched_files": 2,
        "example_matches": [
          "internal/cli/contract/dump.go (prune-expired-workspaces keyword)",
          "internal/contract/manager.go (mentions expires_at)"
        ],
        "issue": "Keyword search found operational code but not configuration constants"
      },
      "*default*": {
        "effectiveness": "LOW",
        "explanation": "Did not effectively identify the settings.go file where DefaultContractTTL is defined",
        "matched_files": 0,
        "example_matches": [],
        "issue": "Discovery metadata didn't associate settings.go with 'default' keyword"
      },
      "*time*": {
        "effectiveness": "MEDIUM",
        "explanation": "Generic keyword that found many time-related code but low signal",
        "matched_files": 3,
        "example_matches": [
          "internal/contract/manager.go (time.Hour usage)",
          "internal/cli/contract/lifecycle.go (time parsing)"
        ],
        "issue": "Too broad, found related files but not the root configuration"
      }
    },
    "new_keywords_discovered_in_code": {
      "settings": {
        "found_in": "pkg/settings/settings.go",
        "pattern": "DefaultContractTTL constant",
        "relevance": "CRITICAL - should be primary search term"
      },
      "TTL": {
        "found_in": "manager.go, settings.go",
        "pattern": "Time-to-live abbreviation for contract expiration",
        "relevance": "HIGH - more specific than 'expiration'"
      },
      "settings.DefaultContractTTL": {
        "found_in": "manager.go line 73",
        "pattern": "Direct reference to configuration constant",
        "relevance": "CRITICAL - exact location of default"
      }
    },
    "keyword_recommendations": {
      "should_add": [
        "DefaultContractTTL - the exact constant name",
        "TTL - time-to-live is standard terminology",
        "settings.go - the configuration file itself",
        "const Default - for configuration constants"
      ],
      "should_refine": [
        "'*expiration*' - too broad, generates false positives (dump.go, chats.go)",
        "'*default*' - failed to find settings file, should index settings-related files more heavily",
        "'*time*' - too generic, matches many unrelated time operations"
      ],
      "future_discovery_strategy": "For configuration-based intent (like 'change the default X'), explicitly search for 'const', 'settings', and 'config' patterns. The current discovery favored operational code over configuration constants."
    }
  },
  "summary_and_recommendations": {
    "files_to_modify_to_change_default_expiration": [
      {
        "priority": "PRIMARY",
        "file": "pkg/settings/settings.go",
        "line": 91,
        "change": "Change 'const DefaultContractTTL = 4' to desired value (e.g., 8 for 8 hours)",
        "reason": "This is the source of truth constant used by all contract creation code"
      },
      {
        "priority": "SECONDARY",
        "file": "internal/contract/manager.go",
        "line": 73,
        "change": "Optionally: Add a parameter to CreateContract to accept custom TTL and override the constant",
        "reason": "Enables programmatic control of TTL instead of just changing the constant"
      },
      {
        "priority": "TERTIARY",
        "file": "internal/cli/contract/create.go",
        "line": 112,
        "change": "Optionally: Add --ttl or --hours flag to CLI",
        "reason": "Allows users to specify custom TTL when creating contracts via CLI"
      }
    ],
    "false_positives_identified": [
      "internal/db/models.go - Wrong file (score was 0.75, should be ~0.0)",
      "internal/db/chats.go - Wrong file (score was 0.6, should be ~0.0)",
      "internal/cli/contract/dump.go - Wrong intent (score was 0.65, should be ~0.2)"
    ],
    "discovery_quality_assessment": "The discovery turn found several relevant files (manager.go, create.go) but with a critical miss: it did not identify pkg/settings/settings.go where the DefaultContractTTL constant is defined. This is a significant gap since settings files are typically where configuration defaults live. The keyword strategy of using wildcards like '*expiration*' and '*default*' failed to distinguish between operational code (checking expiration) and configuration code (setting defaults).",
    "verdict": "MIXED - Moderate success with medium-to-high false positive rate. The top candidate (manager.go 0.95) is correct, but several files (db/models.go, chats.go) are false positives. Most critically, pkg/settings/settings.go was missing entirely."
  }
}
\```

---

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

---

## Critical Missing File Guidelines

When identifying critical missing files:

1. **Look for Configuration Files**: Settings, config, constants files
2. **Look for Source of Truth**: Where the default values are defined
3. **Look for Core Constants**: Named constants that control behavior
4. **Look for Initialization Code**: Where defaults are set at startup
5. **Look for Documentation**: README, ARCHITECTURE files that might mention the file

If you find a critical missing file:
- Include it in the `critical_missing_candidate` section
- Give it a high score (0.9-1.0)
- Explain why it's critical
- Provide the exact location (file path and line number)
- Tell the user what to do with it

---

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

---

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
