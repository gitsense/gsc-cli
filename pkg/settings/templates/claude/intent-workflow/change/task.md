<!--
Component: Change Task Prompt
Block-UUID: 4b8f9c2d-4e5f-4a3b-8c7d-9e0f1a2b3c4f
Parent-UUID: 7a8b9c0d-1e2f-3a4b-5c6d-7e8f9a0b1c2d
Version: 1.14.0
Description: Task prompt for Change turns. Injects discovery turn history, validated files, working directories, and user change request. Updated to support "accountability over restriction" model: allows out-of-scope changes for logical completeness but requires transparency and discovery gap reporting. Added validation checklist, expanded metadata creation instructions, and example workflow for .change-meta.json files. Updated JSON examples to include provenance fields. Updated to support discovery -> change flow by removing references to validation as a separate turn. Simplified metadata creation instructions to rely on the native Write tool instead of shell commands. Removed all references to git_diff generation as it is redundant with files_modified stats and handled by the CLI. Updated to reflect "AI writes rationale, CLI fills metadata" model; removed SHA calculation and complex JSON instructions. Updated to reflect flattened ChangeResult structure where change_summary fields are now at the root level and turn_number has been removed. Added pre-flight cleanup context to inform AI about orphaned metadata files that were removed before starting the turn. Updated to support Active Discovery model with three-way conditional logic (HasActiveDiscovery, IsDiscoverySkipped, no discovery context) to handle skipped discovery turns. Updated to read intent from file instead of inline injection to support large intents consistently with discovery turns. Updated to use absolute paths ({{.TurnDir}}) for all control files to prevent path resolution issues.
Language: Markdown
Created-at: 2026-04-25T15:23:51.832Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0)
-->


# Change Task

{{.PreFlightContext}}

## Your Task

1. Read `{{.TurnDir}}/intent.md` to understand the user's intent
2. Read `{{.TurnDir}}/change_meta_format.md` for the complete format specification and examples
3. Apply the requested changes following the instructions below

{{if .HasActiveDiscovery}}
## Discovery Context

The following files were validated in the most recent discovery turn and are your **Primary Targets**:

```json
{{.DiscoveryContext}}
```

**Your task:**
- Review the discovery results to understand the codebase context.
- Use the **validated files list** as your **Primary Targets** for implementation.
- Read the code for each validated file to understand the current implementation.
- Apply the requested changes, ensuring **logical completeness**.
- Files you modify outside this list are **out_of_scope** and must be reported.
- Provide a summary of what was changed, including scope validation and discovery gaps.

{{else if .IsDiscoverySkipped}}
## Discovery Context

**Discovery was explicitly skipped for this turn.**

There are no pre-validated Primary Targets. Rely on the Intent and your analysis
of the working directories. All modifications you make will be marked as `user_directed`
scope. You are still required to:
- Create a `.change-meta.json` file for every modified file.
- Report all changes in your JSON output with scope `user_directed`.

{{else}}
## Discovery Context

**No valid discovery context found** - cannot proceed with change.

Please run `gsc claude scout start` first to generate validated files for modification.
{{end}}

## Working Directories
{{.Workdirs}}

## Reference Files
{{.RefFiles}}

## Change Metadata Files

For every file you modify, you MUST create a corresponding `.change-meta.json` file.

**See `{{.TurnDir}}/change_meta_format.md` for the complete format specification and examples.**

This file tracks the **rationale** for the change. You are responsible for providing the `description` (why you changed it) and the `absolute_path` (where the file is).

**Critical:** Every modified file MUST have a corresponding `.change-meta.json` file. Missing files will cause the turn to fail.

**Workflow:**
1. Edit the file
2. Create `.change-meta.json` file with `absolute_path` and `description`
3. Repeat for every file you modify

**Validation Checklist:**
After creating all .change-meta.json files, verify:
- [ ] Each modified file has a corresponding `.change-meta.json` file
- [ ] Each `.change-meta.json` file is in the same directory as the modified file
- [ ] Each `.change-meta.json` file contains valid JSON
- [ ] Each file has a meaningful `description` explaining the rationale
- [ ] File naming follows the pattern `.<filename>.change-meta.json`

**Note:** The `.change-meta.json` files will be automatically enriched with technical details (SHAs, type, language) by the CLI after you finish.

## Your Task

Apply the requested changes by:

1.  **Analyze the Intent:** Determine the full scope of the change, including potential side effects or dependencies.
2.  **Check Current State:** If pre-flight cleanup was performed, verify each file's current state before making changes. If a change is already present, create the metadata file documenting that it was verified, not re-applied.
3.  **Identify Primary Targets:** Start with the **validated files** from the discovery turn. These are your high-confidence targets.
4.  **Ensure Logical Completeness:** You are authorized to modify **any file** necessary to fulfill the intent and ensure the system remains stable. This includes:
    -   Modifying files not found during discovery if they are required for the change to be logically complete (e.g., updating a validator, fixing a side effect, or resolving a dependency).
    -   Creating new files if the intent requires new functionality.
5.  **Apply Minimal Edits:** If changing a single constant in one file fulfills the intent, do not edit other files that merely reference that constant. Identify the "Source of Truth" and modify it.
6.  **Document the Scope:** For every file modified, determine if it was in the original discovery scope (`in_scope`) or outside it (`out_of_scope`).
7.  **Report Gaps:** If you modified "out_of_scope" files, identify the keywords or patterns that would have allowed the discovery turn to find them.
8.  **Create Metadata:** Create a `.change-meta.json` file for every modified file.
    -   Use the **Write** tool to create the file in the same directory as the modified file.
    -   Read `{{.TurnDir}}/change_meta_format.md` for the complete specification.
    -   The file only requires two fields: `absolute_path` and `description`.
    -   Verify the file naming follows the pattern `.<filename>.change-meta.json`.

## Example Workflow

Here's a complete example of the metadata creation workflow:

1. **Edit the file:**
```bash
# Modify pkg/settings/settings.go
# Change: const DefaultContractTTL = 4 → const DefaultContractTTL = 48
```

2. **Create .change-meta.json:**
Use the Write tool to create `pkg/settings/.settings.go.change-meta.json` with the following content:
```json
{
  "absolute_path": "/home/user/project/pkg/settings/settings.go",
  "description": "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time"
}
```

3. **Verify:**
```bash
# Check file exists
ls -la pkg/settings/.settings.go.change-meta.json
# Validate JSON
jq . pkg/settings/.settings.go.change-meta.json
```

4. **Repeat** for each additional file you modify.

## Important Constraints

- **Edit Files in Place:** Modify files directly in the working directories.
- **No Block-UUID Updates:** Do NOT update Block-UUID information in code block headers.
- **Transparency First:** You are not strictly limited to the validated list, but you **MUST** explicitly track and justify any "out_of_scope" modifications in the JSON output.
- **Accountability for Out-of-Scope Changes:**
    -   If you modify a file that was **not** in the validated discovery list:
        1.  Mark the file as `scope: "out_of_scope"` in your `files_modified` report.
        2.  Provide a specific `reason` why this file was necessary (e.g., "Discovered a hard-coded limit in this file that conflicted with the new 48h TTL").
        3.  Populate the `discovery_gap` section with keywords or patterns that would have allowed the discovery turn to find this file.
- **Discovery Gap Reporting:** Every "out_of_scope" modification must result in an entry in the `discovery_gap` section to improve future discovery results.
- **Create .change-meta.json files:** Every modified file MUST have a corresponding `.change-meta.json` file.
- **Focus on the Change:** Your primary goal is to make the requested changes, not update metadata.

## Output Format

Return ONLY valid JSON (no additional text) with the following structure:

```json
{
  "change_request": "string",
  "files_modified": {
    "total_count": 2,
    "in_scope_count": 1,
    "out_of_scope_count": 1,
    "files": [
      {
        "working_dir": "string",
        "path": "string",
        "status": "modified",
        "scope": "in_scope"
      },
      {
        "working_dir": "string",
        "path": "string",
        "status": "modified",
        "scope": "out_of_scope",
        "reason": "Required dependency for in_scope file"
      }
    ]
  },
  "discovery_gap": {
    "files_added": 1,
    "files": [
      {
        "working_dir": "string",
        "path": "string",
        "reason": "This file was needed but not discovered",
        "suggested_keywords": ["keyword1", "keyword2"]
      }
    ]
  },
  "changelog": [],
  "notes": "Any notes or observations",
  "errors": ""
}
```

## Important Notes

- **No Additional Text**: Return ONLY the JSON, no explanations or markdown formatting
- **Absolute Paths**: Use absolute paths for the `working_dir` field in `files_modified`
- **Relative Paths**: Use relative paths for the `path` field in `files_modified`
- **Status Values**: Use "modified", "added", or "deleted" for file status
- **Scope Values**: Use "in_scope" for files from the validated list, "out_of_scope" for files added during implementation
- **Empty Strings**: Use empty strings for notes and errors if not applicable
- **Change Metadata**: Every modified file MUST have a corresponding `.change-meta.json` file in the same directory
- **Discovery Gap**: If you modify out_of_scope files, you MUST populate the `discovery_gap` section with `suggested_keywords` to help future discovery turns find these files.
