<!--
Component: Change Task Prompt
Block-UUID: a1b2c3d4-5e6f-7g8h-9i0j-1k2l3m4n5o6p
Parent-UUID: N/A
Version: 1.0.0
Description: Task prompt for Change turns. Injects discovery and validation turn history, validated files, working directories, and user change request.
Language: Markdown
Created-at: 2026-04-15T04:11:20.000Z
Authors: GLM-4.7 (v1.0.0)
-->


# Change Task

## Your Intent

{{.Intent}}

{{if .TurnHistoryExists}}
## Previous Discovery and Validation Context

The following discovery and validation turns provide context from previous turns:

```json
{{.TurnHistoryJSON}}
```

**Your task:**
- Review the discovery and validation results to understand the codebase context
- Use the validated files list to know which files you should modify
- Read the code for each validated file to understand the current implementation
- Apply the requested changes to the validated files
- Generate git diffs for all working directories after making changes
- Provide a summary of what was changed
{{else}}
## Previous Discovery and Validation Context

**No previous discovery and validation context available** - cannot proceed with change.

Please run discovery and validation turns first to generate validated files for modification.
{{end}}

## Working Directories
{{.Workdirs}}

## Reference Files
{{.RefFiles}}

## Your Task

Apply the requested changes by:
1. Reading the validated files from the discovery and validation turns
2. Understanding the current implementation
3. Making the requested changes in place (editing files directly)
4. Generating git diffs for each working directory
5. Providing a summary of what was changed

## Important Constraints

- **Edit files in place**: Modify files directly in the working directories
- **Do NOT update Block-UUIDs**: Leave code block headers unchanged
- **Use validated files only**: Only modify files that were validated in previous turns
- **Generate git diffs**: Run `git diff` in each working directory after making changes
- **Focus on the change**: Your primary goal is to make the requested changes, not update metadata

## Output Format

Return ONLY valid JSON (no additional text) with the following structure:

```json
{
  "change_summary": {
    "turn_number": 3,
    "change_request": "The original change request",
    "files_modified_count": 1,
    "files_modified": [
      {
        "working_dir": "/absolute/path/to/workdir",
        "path": "relative/path/to/file.go",
        "status": "modified"
      }
    ]
  },
  "git_diff": {
    "/absolute/path/to/workdir": "diff --git a/file.go b/file.go\n..."
  },
  "notes": "Any notes or observations",
  "errors": ""
}
```

## Important Notes

- **No Additional Text**: Return ONLY the JSON, no explanations or markdown formatting
- **Absolute Paths**: Use absolute paths for working directories in git_diff map
- **Relative Paths**: Use relative paths for files within working directories
- **Status Values**: Use "modified", "added", or "deleted" for file status
- **Empty Strings**: Use empty strings for notes and errors if not applicable
- **Git Diff**: Include the complete git diff output for each working directory
