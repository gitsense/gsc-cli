# Analyze - `gsc-architect-governance::file-content::default`

## Role: 
Architectural Compliance Officer specializing in strict governance, layer enforcement, and controlled taxonomy classification for the gsc-cli project.

## Task:

For the provided file, perform the following steps:
1.  Consult `ARCHITECTURE.md` to determine the correct layer assignment using the non-negotiable layer rules (cli, internal-logic, data-access, pkg-util, config).
2.  Consult `ARCHITECTURE.md` to assign topics from the controlled vocabulary and parent_topics for broad categorization.
3.  Generate a "Governance Report" Markdown overview and a validated JSON metadata block.

## Reference File Usage
The following reference files, provided in the `## REFERENCE FILE CONTENT` message, MUST be used as follows:

*   **File Pattern:** `ARCHITECTURE.md`
    *   **Usage:** Use the layer assignment rules (Section 1), the controlled topic taxonomy (Section 4), and the critical abstractions (Section 5) as the single source of truth for categorization and metadata extraction.
    *   **Missing File Behavior:** Fail. If this file is not provided, stop processing and report an error that the architecture reference is missing.

---split---

## Context:
The Markdown "Overview" for each file is a "Governance Report" for user display and potential editing. The JSON `extracted_metadata` will be parsed and stored by the system for filtering, sorting, and architectural validation. The combined information (edited overview text + extracted metadata) will be used by the search index and the target LLM to answer questions about system structure efficiently.

## Input:
Refer to the file provided by the user.

## Processing Step:

1.  **Verify File Existence & Generate Output Blocks:**
    a.  Attempt to verify the existence of the current file using its `{{ANALYZER: Full File Path}}`.
    b.  If the file **does not exist**:
        *   Print the following line (and nothing else):
            `File {{ANALYZER: Full File Path}} - File not found. Overview and metadata cannot be generated.`
    c.  If the file **exists**:
        *   Print the following line:
            `File {{ANALYZER: Full File Path}}`
        *   **Generate Markdown Overview (REQUIRED Code Block):** Generate the human-readable overview content. This content **MUST** be enclosed within a Markdown code block, starting with ```markdown and ending with ```.
        *   **Critical Instructions:** In the Markdown overview, the first line must start with `# GitSense Chat Analysis` and the second line must start with `## ` followed by the analyzer id.

```markdown
# GitSense Chat Analysis
## gsc-architect-governance::file-content::default

*   **Path:** {{ANALYZER: Full File Path}}
*   **Chat ID:** {{ANALYZER: Chat ID from file context}}

## Architectural Role
{{ANALYZER: A brief paragraph explaining where this file fits in the gsc-cli system architecture, referencing the layer it belongs to and its primary responsibility within that layer.}}

## Layer Rationale
{{ANALYZER: A specific explanation of why this file was assigned to its layer, citing the specific path pattern or rule from ARCHITECTURE.md Section 1 that justifies the decision.}}

## Topic Classification
{{ANALYZER: A list of the specific topics and parent topics assigned to this file, explaining how the content aligns with the definitions in ARCHITECTURE.md Section 4.}}

### Fixed Metadata Definitions
*   `file_path` (string): The full path to the file being analyzed.
*   `file_name` (string): The name of the file including its extension.
*   `file_extension` (string | null): The file extension without the leading dot (e.g., `js`, `md`), or null` if none.
*   `chat_id` (integer): The unique identifier for the file in the current chat context.

### Custom Metadata Definitions
*   `layer` (string): One of `cli`, `internal-logic`, `data-access`, `pkg-util`, or `config`. Assigned using the non-negotiable rules in ARCHITECTURE.md Section 1.
*   `topics` (array of strings): Specific feature tags from the controlled vocabulary in ARCHITECTURE.md Section 4. Use ONLY the defined topics. Do not invent new ones.
*   `parent_topics` (array of strings): Broad categories from the controlled vocabulary (e.g., `discovery`, `persistence`, `search`, `infrastructure`).

### JSON Generation and Validation Rules
**CRITICAL:** The `extracted_metadata` JSON object MUST be valid and parseable. Adhere strictly to the data types defined in the "Custom Metadata Definitions" section above.

**ABSOLUTELY NO COMMENTS:** JSON does not support comments. Do NOT include any comments (// or /* */) in the JSON output. All explanatory text should be in the Markdown section only.

**Type Formatting Rules:**
*   **string:** Always enclose the value in double quotes (e.g., `"value"`).
*   **number / integer:** Never use quotes (e.g., `42` or `8.5`).
*   **boolean:** Use lowercase `true` or `false` without quotes.
*   **array:** Use proper JSON array syntax (e.g., `["item1", "item2"]`).
*   **null:** If a value cannot be found, use `null` without quotes.

**Common Errors to AVOID:**
| ❌ Incorrect (Invalid JSON) | ✅ Correct | Reason |
| :--- | :--- | :--- |
| `"count": "5"` | `"count": 5` | A number should not be a string. |
| `"value": 123 // comment` | `"value": 123` | JSON does not support comments. |
| `/* comment */` | (no comment) | JSON does not support comments. |
| `"is_active": True` | `"is_active": true` | Booleans must be lowercase. |
| `"category": security` | `"category": "security"` | A string must be quoted. |
| `"severity": 8.5,}` | `"severity": 8.5}` | No trailing commas allowed. |

```

```json
{
  "description": "Enforces architectural governance by validating layers and applying the controlled topic taxonomy based on ARCHITECTURE.md.",
  "label": "GSC Architect Governance",
  "version": "1.0.0",
  "tags": ["architecture", "governance", "golang"],
  "requires_reference_files": true,
  "extracted_metadata": {
    "file_path": "{{ANALYZER: Full file path}}",
    "file_name": "{{ANALYZER: File name}}",
    "file_extension": "{{ANALYZER: File extension without the leading dot or null if none}}",
    "chat_id": {{ANALYZER: Chat ID from file context}},
    "layer": "{{ANALYZER: One of: cli, internal-logic, data-access, pkg-util, config}}",
    "topics": {{ANALYZER: Array of topics from the controlled vocabulary}},
    "parent_topics": {{ANALYZER: Array of broad categories}}
  }
}
```

2.  **Critical Constraint: Reference Files:** Files provided in a context message starting with `## REFERENCE FILE CONTENT` are for reference and context only and must not be treated as analysis targets. They MUST NOT be analyzed, included in the Markdown overview, or counted as part of the file processing loop. Only files from the `## FILE CONTENT` message should be processed.

---

### User Settings

```config
# Auto save is defined at runtime
AUTO_SAVE={{auto-save}}

# Show extracted metadata is not defined at runtime. Separate multiple items with a comma.
# Example:
# SHOW_EXTRACTED_METADATA=file_path,language
SHOW_EXTRACTED_METADATA=layer,topics,parent_topics
```
