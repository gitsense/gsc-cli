package manifest

func GenerateExample() string {
	return manifestExampleMarkdown
}

const manifestExampleMarkdown = `# GitSense Manifest Format

## What Is a Manifest

A manifest is a JSON file that packages structured metadata about repository files.
` + "`gsc manifest import`" + ` consumes this format to create a local Brain (SQLite database)
that agents query with ` + "`gsc rg`" + `, ` + "`gsc query`" + `, and other commands. No GitSense Chat
app is required.

All you need to create a manifest is a script that walks your files and writes
JSON in the format below.

## Top-Level Structure

| Field | Type | Required | Description |
|---|---|---|---|
| schema_version | string | yes | Manifest format version. Use "1.0.0" |
| generated_at | string | yes | ISO 8601 timestamp when manifest was created |
| manifest | object | yes | Metadata about this manifest |
| repositories | array | yes | Repository references |
| branches | array | yes | Branch references |
| analyzers | array | yes | Analyzer definitions |
| fields | array | yes | Field definitions |
| data | array | yes | File records with metadata values |

Each data entry must include a unique non-zero ` + "`chat_id`" + `. For manifests that
do not come from GitSense Chat, generate stable integer IDs such as 1001, 1002,
and 1003.

## The manifest Object

| Field | Type | Required | Description |
|---|---|---|---|
| name | string | yes | Human-readable name for this manifest |
| database_name | string | no | Database filename, without ".db" |
| description | string | no | One-line description |
| tags | string[] | no | Searchable tags |

## Reference System

Repositories, branches, analyzers, and fields each get a short reference ID
(R1000, B1000, A1000, F1000). Data entries use these refs instead of repeating
full names. This keeps the data section compact.

Convention: R = repository, B = branch, A = analyzer, F = field. The number is
arbitrary; use any unique string per type.

## Supported Field Types

| Type | JSON Type | Example Value |
|---|---|---|
| string | string | "backend" |
| number | number | 42 |
| boolean | boolean | true |
| array | JSON array | ["auth", "high-risk"] |

## Database Name Resolution

When you run ` + "`gsc manifest import`" + `, the database name is determined by this priority:

1. ` + "`--name`" + ` flag on the import command
2. ` + "`manifest.database_name`" + ` field in the JSON
3. Filename, for example ` + "`file-stats.json`" + ` becomes ` + "`file-stats`" + `

## Complete Example

The following is a valid manifest that creates a ` + "`file-stats`" + ` Brain containing
lines-of-code and file-size metadata for three files.

` + "```json" + `
{
  "schema_version": "1.0.0",
  "generated_at": "2026-06-11T17:00:00Z",
  "manifest": {
    "name": "File Statistics",
    "database_name": "file-stats",
    "description": "Lines of code and file size for every tracked file",
    "tags": ["file-stats", "loc", "size"]
  },
  "repositories": [
    { "ref": "R1000", "name": "my-org/my-repo" }
  ],
  "branches": [
    { "ref": "B1000", "name": "main" }
  ],
  "analyzers": [
    {
      "ref": "A1000",
      "id": "file-stats",
      "name": "File Statistics Extractor",
      "description": "Extracts LOC, SLOC, and byte size from each file",
      "version": "1.0.0"
    }
  ],
  "fields": [
    {
      "ref": "F1000",
      "analyzer_ref": "A1000",
      "name": "loc",
      "display_name": "Lines of Code",
      "type": "number",
      "description": "Total lines including blanks and comments"
    },
    {
      "ref": "F1001",
      "analyzer_ref": "A1000",
      "name": "sloc",
      "display_name": "Source Lines",
      "type": "number",
      "description": "Non-blank, non-comment lines"
    },
    {
      "ref": "F1002",
      "analyzer_ref": "A1000",
      "name": "size_bytes",
      "display_name": "File Size",
      "type": "number",
      "description": "File size in bytes"
    }
  ],
  "data": [
    {
      "repo_ref": "R1000",
      "branch_ref": "B1000",
      "file_path": "src/main.rs",
      "language": "Rust",
      "chat_id": 1001,
      "fields": { "F1000": 247, "F1001": 183, "F1002": 8421 }
    },
    {
      "repo_ref": "R1000",
      "branch_ref": "B1000",
      "file_path": "src/parser.rs",
      "language": "Rust",
      "chat_id": 1002,
      "fields": { "F1000": 512, "F1001": 398, "F1002": 18743 }
    },
    {
      "repo_ref": "R1000",
      "branch_ref": "B1000",
      "file_path": "README.md",
      "language": "Markdown",
      "chat_id": 1003,
      "fields": { "F1000": 89, "F1001": 72, "F1002": 3210 }
    }
  ]
}
` + "```" + `

## Creating Your Own

1. Run ` + "`gsc manifest example`" + ` to see this reference
2. Write a script that walks your repository and produces JSON in the format above
3. Validate with ` + "`gsc manifest validate <your-file>.json`" + `
4. Fix any errors the validator reports
5. Import with ` + "`gsc manifest import <your-file>.json`" + `
6. Query with ` + "`gsc rg <pattern> --db <database_name> --fields <field1,field2>`" + `
`
