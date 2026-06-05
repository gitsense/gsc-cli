<!--
Component: Change Resume Task Prompt
Block-UUID: 0c3bd124-1243-4666-93a5-c1bb61f2a440
Parent-UUID: d4a4cc6b-8ab0-4ac5-a53b-792e3116018c
Version: 1.3.0
Description: Task prompt for Change Resume turns. Injects Git provenance metadata (SHAs) and instructs the AI to generate missing .change-meta.json files for a previously failed change turn. Updated to support conditional versioning based on EnableCodeProvenance flag. When enabled, AI must inspect existing headers and declare semantic versions.
Language: Markdown
Created-at: 2026-04-23T17:12:40.762Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0)
-->


# Change Resume Task

## Context

You are resuming a **failed change turn**. The previous turn successfully modified files in the codebase, but it failed because it did not create the required `.change-meta.json` files for some or all of the modified files.

## Your Task

Your **only** task is to generate the missing `.change-meta.json` files for the modified files listed below.

**Do NOT make any code changes.** The files have already been modified. You are only creating metadata documentation.

## Modified Files

The following files were modified in the previous turn. For each file, you have the Git blob SHAs before and after the change.

{{.ModifiedFiles}}

## Instructions

For each file in the list above:

1.  **Inspect the Change:** Use the provided `old_blob_sha` and `new_blob_sha` to see what changed.
    *   Run: `git diff <old_blob_sha> <new_blob_sha> -- <file_path>`
    *   Analyze the diff to understand the nature of the change.

2.  **Create Metadata File:** Create a `.change-meta.json` file in the same directory as the modified file.
    *   **Filename:** `.<filename>.change-meta.json`
    *   **Content:** Valid JSON with the following fields:
        *   `absolute_path`: Full absolute path to the file.
        *   `description`: Clear description of what changed and why (based on your analysis of the git diff).
        {{if .EnableCodeProvenance}}
        *   `version`: Semantic version (MAJOR.MINOR.PATCH) based on the impact of the change.
        {{end}}

**Note:** The CLI will automatically enrich these files with technical details (SHAs, change_type, language) after you finish. You only need to provide the absolute_path, description{{if .EnableCodeProvenance}}, and version{{end}}.

{{if .EnableCodeProvenance}}
### Versioning Rules

Before declaring the `version` for a file, inspect the file's Code Block Header (if one exists):
- **Header present:** Read the current `Version`. Declare the next version based on change impact: patch (bug fix / minor tweak), minor (new feature / additive), major (breaking / complete rewrite).
- **No header:** Declare `1.0.0`.
{{end}}

## Example Workflow

**File:** `pkg/settings/settings.go`
**Old SHA:** `abc123...`
**New SHA:** `def456...`

1.  **Inspect:**
```bash
git diff abc123... def456... -- pkg/settings/settings.go
# Output shows: const DefaultContractTTL = 4 → const DefaultContractTTL = 48
```

2.  **Create Metadata:**
{{if .EnableCodeProvenance}}
```bash
cat > pkg/settings/.settings.go.change-meta.json << 'EOF'
{
  "absolute_path": "/home/user/project/pkg/settings/settings.go",
  "description": "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time",
  "version": "1.1.0"
}
EOF
```
{{else}}
```bash
cat > pkg/settings/.settings.go.change-meta.json << 'EOF'
{
  "absolute_path": "/home/user/project/pkg/settings/settings.go",
  "description": "Updated DefaultContractTTL from 4 to 48 hours to extend default contract expiration time"
}
EOF
```
{{end}}

## Important Constraints

- **No Code Changes:** Do NOT modify any source code files.
- **Metadata Only:** Your output is the creation of `.change-meta.json` files.
- **Accuracy:** The `description` must clearly explain the rationale for the change.
{{if .EnableCodeProvenance}}
- **Version Accuracy:** The `version` must accurately reflect the semantic impact of the change.
{{end}}
- **Completeness:** You must create a metadata file for **every** file listed in the Modified Files section.
- **Required Fields:** Only include `absolute_path`, `description`{{if .EnableCodeProvenance}}, and `version`{{end}} in your JSON. Do not include `file_sha`, `change_type`, or `language` - these will be added by the CLI.

## Output Format

Once you have created all the files, confirm completion by returning ONLY the following JSON block:

```json
{
  "status": "complete"
}
```

Do not include any other text after this block.

Once you have created all the files, you are done.
