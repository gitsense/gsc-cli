<!--
Component: Change Meta File Format Specification
Block-UUID: f257505e-87ca-4c94-8347-6e1529c313b2
Parent-UUID: 400c24af-4bf7-4da8-93dd-779831ba1cc1
Version: 1.6.0
Description: Format specification for .change-meta.json files. Updated to support conditional versioning based on EnableCodeProvenance flag. When enabled, AI must provide a semantic version; when disabled, only absolute_path and description are required.
Language: Markdown
Created-at: 2026-04-20T03:04:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
-->


# Change Meta File Format

## Purpose

Every file you modify during a change turn MUST have a corresponding `.change-meta.json` file.

This file tracks the **rationale** for the change. You are responsible for providing the `description` (why you changed it) and the `absolute_path` (where the file is).

{{if .EnableCodeProvenance}}
When code provenance is enabled, you are also responsible for declaring the semantic `version` of the file based on the impact of your changes.
{{end}}

## File Naming Convention

**Pattern:** `.<filename>.change-meta.json`

**Examples:**
- Modify `settings.go` → Create `.settings.go.change-meta.json`
- Modify `internal/auth/middleware.go` → Create `.middleware.go.change-meta.json`
- Add `new_file.py` → Create `.new_file.py.change-meta.json`

**Important:** The `.change-meta.json` file must be in the **same directory** as the file it describes.

## JSON Format

{{if .EnableCodeProvenance}}
You need to create a JSON file with the following three fields.

```json
{
  "absolute_path": "/absolute/path/to/file.go",
  "description": "Clear description of what changed and why",
  "version": "1.0.0"
}
```
{{else}}
You need to create a JSON file with the following two fields.

```json
{
  "absolute_path": "/absolute/path/to/file.go",
  "description": "Clear description of what changed and why"
}
```
{{end}}

## Required Fields

### `absolute_path` (required)
- **Type:** string
- **Description:** Full absolute path to the file you modified.
- **Example:** `/home/user/project/pkg/settings/settings.go`

### `description` (required)
- **Type:** string
- **Description:** Clear, concise description of what changed and why.
- **Example:** "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time"
- **Guidelines:**
  - **Structure:** Use a "What + Why + Scope" format.
  - **What:** Briefly state the change (e.g., "Updated X from A to B").
  - **Why:** Link to the user's intent (e.g., "...to fulfill the one-week expiration requirement").
  - **Scope/Impact:** Explicitly state if related files were checked and deemed safe (e.g., "Downstream consumers read this constant and require no modification").
  - **Conciseness:** Keep it to 1-2 sentences.

{{if .EnableCodeProvenance}}
### `version` (required)
- **Type:** string
- **Format:** Semantic version (MAJOR.MINOR.PATCH)
- **Rules:**
  - **If the file has a Code Block Header:** Read the current `Version`. Declare the next version based on impact: patch (bug fix / minor tweak), minor (new feature / additive), major (breaking / complete rewrite).
  - **If the file has NO header:** Declare `1.0.0`.
{{end}}

## Example

**File:** `pkg/settings/settings.go`

**Create:** `.settings.go.change-meta.json` with the following content:

{{if .EnableCodeProvenance}}
```json
{
  "absolute_path": "/home/user/project/pkg/settings/settings.go",
  "description": "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time",
  "version": "1.1.0"
}
```
{{else}}
```json
{
  "absolute_path": "/home/user/project/pkg/settings/settings.go",
  "description": "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time"
}
```
{{end}}

## Critical Requirements

### 1. One File Per Change
- **Every** modified file MUST have a corresponding `.change-meta.json` file.
- **Fatal Error:** If you modify a file but don't create its `.change-meta.json` file, the turn will fail.
- **No Exceptions:** This applies to all file types (code, config, documentation, etc.)

### 2. Description Quality
- The `description` is the primary artifact you are responsible for.
- It must clearly explain the **rationale** behind the change.
- **Fatal Error:** Vague or missing descriptions will cause the turn to fail validation.

### 3. Correct File Naming
- Use the exact pattern: `.<filename>.change-meta.json`.
- Place the file in the same directory as the file it describes.
- **Fatal Error:** Incorrectly named or misplaced files will cause the turn to fail.

{{if .EnableCodeProvenance}}
### 4. Version Accuracy
- The `version` must accurately reflect the semantic impact of the change.
- **Fatal Error:** Incorrect versioning (e.g., declaring a patch for a breaking change) will cause the turn to fail validation.
{{end}}

## Workflow

1. **Edit the file** - Make your changes to the target file.
2. **Create .change-meta.json** - Create the file in the same directory with:
   - `absolute_path`: The full path to the file.
   - `description`: Your rationale for the change.
   {{if .EnableCodeProvenance}}
   - `version`: The semantic version for this change.
   {{end}}
3. **Repeat** - Do this for every file you modify.

## Common Mistakes to Avoid

### ❌ Missing Description
```json
{
  "absolute_path": "/path/to/file.go"
  // Missing description!
}
```
**Result:** Turn fails with error "Missing description in .settings.go.change-meta.json"

### ❌ Vague Description
```json
{
  "description": "Updated the file"  // Too vague
}
```
**Result:** Turn fails validation. The description must explain *why*.

### ❌ Wrong Location
You create `.settings.go.change-meta.json` in the wrong directory.
**Result:** Turn fails with error "Cannot find .change-meta.json for settings.go"

{{if .EnableCodeProvenance}}
### ❌ Incorrect Version
```json
{
  "version": "1.0.0"  // File already exists and is v1.2.0
}
```
**Result:** Turn fails validation. You must inspect the existing header and bump the version correctly.
{{end}}

## Special Cases

### Files Without Extensions
- Use the full filename in the `.change-meta.json` name.
- Example: Modify `Makefile` → Create `.Makefile.change-meta.json`.

### Multiple Changes to Same File
- Only create **one** `.change-meta.json` file.
- Describe all changes in the single `description` field.

## Debugging

If you encounter issues with `.change-meta.json` files:

1. **Check file exists:** Verify the `.change-meta.json` file is in the correct directory.
2. **Validate JSON:** Use `jq . .settings.go.change-meta.json` to validate JSON syntax.
3. **Check description:** Ensure the `description` field is present and meaningful.
4. **Check naming:** Ensure the filename matches the pattern `.<filename>.change-meta.json`.
{{if .EnableCodeProvenance}}
5. **Check version:** Ensure the `version` field is present and follows semantic versioning rules.
{{end}}
