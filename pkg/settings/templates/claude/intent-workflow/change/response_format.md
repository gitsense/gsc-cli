<!--
Component: Change Response Format Specification
Block-UUID: 7f34567e-95dd-46b6-9b85-ce8ce4f63789
Parent-UUID: 3d4b15b5-ed32-4d8c-827e-e588fe51dfa3
Version: 1.4.0
Description: Exact JSON schema and validation rules for change turn responses. Read by the correction turn AI to map a malformed response to the expected format. Includes discovery scope, file modifications with scope tracking, and discovery gap sections. Updated to reflect flattened ChangeResult structure where change_summary fields are now at the root level and turn_number has been removed.
Language: Markdown
Created-at: 2026-04-25T15:25:26.418Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
-->


# Change Response Format Specification

This document defines the exact JSON schema that MUST be returned by all change turns.

## Required JSON Structure

```json
{
  "change_request": "The original change request",
  "files_modified": {
    "total_count": 3,
    "in_scope_count": 2,
    "out_of_scope_count": 1,
    "files": [
      {
        "working_dir": "/absolute/path/to/workdir",
        "path": "relative/path/to/file.go",
        "status": "modified",
        "scope": "in_scope"
      },
      {
        "working_dir": "/absolute/path/to/workdir",
        "path": "relative/path/to/dependency.go",
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
        "working_dir": "/absolute/path/to/workdir",
        "path": "relative/path/to/missed.go",
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

## Field Specifications

### Root Level
- `change_request` (string, required): Original change request from user
- `files_modified` (object, required): Summary of all files modified
- `discovery_gap` (object, required): Files that were missed by discovery
- `changelog` (array, required): Aggregated changelog from .change-meta.json files
- `notes` (string, optional): Notes or observations
- `errors` (string, optional): Errors encountered (empty string if none)

### files_modified Object
- `total_count` (integer, required): Total number of files modified
- `in_scope_count` (integer, required): Number of in-scope files modified
- `out_of_scope_count` (integer, required): Number of out-of-scope files modified
- `files` (array, required): Array of file modification entries
  - `working_dir` (string, required): Absolute path to working directory
  - `path` (string, required): Relative path to file
  - `status` (string, required): `"modified"`, `"added"`, or `"deleted"`
  - `scope` (string, required): `"in_scope"` or `"out_of_scope"`
  - `reason` (string, optional): Why this out-of-scope file was modified

### discovery_gap Object
- `files_added` (integer, required): Number of files in discovery gap
- `files` (array, required): Array of discovery gap entries
  - `working_dir` (string, required): Absolute path to working directory
  - `path` (string, required): Relative path to file
  - `reason` (string, required): Why this file was missed by discovery
  - `suggested_keywords` (array of strings, required): Keywords that would have found this file

### changelog Array
- Array of changelog entries aggregated from .change-meta.json files
- Each entry contains metadata about the changes made

## Common Format Errors to Avoid

### ❌ WRONG - Count mismatch
```json
{
  "files_modified": {
    "total_count": 3,
    "in_scope_count": 2,
    "out_of_scope_count": 1,
    "files": [
      { "scope": "in_scope" },
      { "scope": "in_scope" }
    ]
  }
}
```

### ✅ CORRECT - Counts match actual files
```json
{
  "files_modified": {
    "total_count": 2,
    "in_scope_count": 2,
    "out_of_scope_count": 0,
    "files": [
      { "scope": "in_scope" },
      { "scope": "in_scope" }
    ]
  }
}
```

---

### ❌ WRONG - Invalid scope value
```json
{
  "files": [
    {
      "scope": "outside",
      "status": "modified"
    }
  ]
}
```

### ✅ CORRECT - Valid scope values
```json
{
  "files": [
    {
      "scope": "out_of_scope",
      "status": "modified"
    }
  ]
}
```

---

### ❌ WRONG - Missing reason for out_of_scope
```json
{
  "files": [
    {
      "scope": "out_of_scope",
      "status": "modified",
      "path": "dependency.go"
    }
  ]
}
```

### ✅ CORRECT - Include reason for out_of_scope
```json
{
  "files": [
    {
      "scope": "out_of_scope",
      "status": "modified",
      "path": "dependency.go",
      "reason": "Required dependency for in_scope file settings.go"
    }
  ]
}
```

## Validation Checklist

Before returning your response, verify:

- [ ] All required fields are present
- [ ] `change_request` is not empty
- [ ] `files_modified.total_count` matches length of `files_modified.files` array
- [ ] `files_modified.in_scope_count` matches actual in_scope files
- [ ] `files_modified.out_of_scope_count` matches actual out_of_scope files
- [ ] All `scope` values are either `"in_scope"` or `"out_of_scope"`
- [ ] All `status` values are `"modified"`, `"added"`, or `"deleted"`
- [ ] Out-of-scope files have a `reason` field explaining why they were modified
- [ ] `discovery_gap.files_added` matches length of `discovery_gap.files` array
- [ ] JSON is syntactically valid and fully parseable

## Example Valid Response

```json
{
  "change_request": "Change default contract expiration time to 48 hours",
  "files_modified": {
    "total_count": 1,
    "in_scope_count": 1,
    "out_of_scope_count": 0,
    "files": [
      {
        "working_dir": "/home/user/projects/gsc-server",
        "path": "pkg/settings/settings.go",
        "status": "modified",
        "scope": "in_scope"
      }
    ]
  },
  "discovery_gap": {
    "files_added": 0,
    "files": []
  },
  "changelog": [],
  "notes": "Successfully updated default contract TTL from 4 to 48 hours. This change was based on discovery validation that identified settings.go as the source of the DefaultContractTTL constant (score: 0.99). No other files needed modification.",
  "errors": ""
}
```

## Example with Out-of-Scope Files

```json
{
  "change_request": "Add logging to contract creation",
  "files_modified": {
    "total_count": 2,
    "in_scope_count": 1,
    "out_of_scope_count": 1,
    "files": [
      {
        "working_dir": "/home/user/projects/gsc-server",
        "path": "internal/contract/manager.go",
        "status": "modified",
        "scope": "in_scope"
      },
      {
        "working_dir": "/home/user/projects/gsc-server",
        "path": "pkg/logger/logger.go",
        "status": "modified",
        "scope": "out_of_scope",
        "reason": "Required dependency for logging functionality in CreateContract"
      }
    ]
  },
  "discovery_gap": {
    "files_added": 0,
    "files": []
  },
  "changelog": [],
  "notes": "Added logging to CreateContract function. Modified logger.go (out_of_scope) to add new log level for contract events.",
  "errors": ""
}
```
