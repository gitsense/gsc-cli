<!--
Component: GSC Overview
Block-UUID: c8d2f483-24b4-4f60-b5ac-f840efecd9c9
Parent-UUID: 4950ddb6-c920-4cf2-b9e5-1dbc95109db8
Version: 1.5.0
Description: Removed promotional language. Renamed "The README for AI" section to "Portability: Versioning Manifests", removed "self-serve to save tokens" framing, and replaced with neutral equivalents.
Language: Markdown
Created-at: 2026-04-30T23:51:56.570Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), Gemini 2.5 Flash Lite (v1.3.0), Codex GPT-5 (v1.4.0), claude-sonnet-4-6 (v1.5.0)
-->


# GSC Overview

## What is gsc?

`gsc` (GitSense CLI) is a repository intelligence tool. With **Brains** (SQLite databases), it enriches file trees and searches with structured metadata. Without Brains, it still provides AI-facing guidance and ripgrep-backed repository search, but metadata commands are unavailable until a Manifest is imported.

## What is a Brain?

A Brain is a **constructed knowledge base** (a SQLite database) that serves as the Expert Persona's memory. It is built by importing a Manifest containing Analyzer results. Once constructed, it provides structured intelligence for metadata queries. A repository may have zero Brains; in that case, `gsc experts init` should still work and the agent should say that no metadata intelligence is active.

## Capability Groups

| Group | What it does | Detail Guide |
| :--- | :--- | :--- |
| **Query & Analysis** | Filter files by metadata (Concepts), analyze coverage, list field values | `GSC_QUERY_GUIDE.md` |
| **Visualization & Search** | Enrich the file tree, search code with metadata context | `GSC_VISUALIZATION_GUIDE.md` |
| **Brain Management** | Initialize workspace, import manifests, construct Brains | `GSC_BRAIN_MANAGEMENT_GUIDE.md` |
| **Consultation** | Guide AI assistants to act as strategic consultants before triggering Inline Agents | `gsc experts guide` |

## Intelligence-First Principle

Distinguish between **Concepts** and **Symbols** before choosing a tool.

- **Concepts/Intents:** Use `gsc query --filter`. (e.g., "Find files that handle authentication").
- **Known files/path scopes:** Use `gsc query --glob ... --fields ...` without `--filter` to retrieve metadata for explicit files or directories.
- **Symbols/Strings:** Use `gsc rg`. (e.g., "Where is `DEFAULT_TTL` defined?").

Prefer `gsc rg` for repository-wide searches to leverage metadata context. Use standard `grep` or `rg` for targeted pattern matching within specific files where metadata is not required.

Only open a file when `gsc rg` or `gsc query` metadata is insufficient to answer the question. Reading files that metadata already describes consumes tokens without providing additional value.

## Transparent Execution & Education

The AI always displays the full `gsc` command it executes and explains its
reasoning. This transparency allows you to critique the logic, verify commands,
and learn the tool.

## Expertise Handshake Protocol

At the start of every session, run:

```bash
gsc brains --json
```

Read the `description` and `fields` of each active Brain returned in
`databases`. If the active list is empty, tell the user there are no active
Brains, then proceed with text/path search when possible. If
`inactive_databases` is present, those entries are manifests the user can
activate with their `import_command`.

## Portability: Versioning Manifests

A Manifest is the blueprint. By versioning it in your repo, you allow anyone to **construct the same Brain** simply by running `gsc manifest import`.

## Routing Table

| If the user asks about… | Load |
| :--- | :--- |
| Querying, filtering, coverage, insights | `GSC_QUERY_GUIDE.md` |
| File tree, grep, visualizing structure | `GSC_VISUALIZATION_GUIDE.md` |
| Missing Brain, importing a manifest | `GSC_BRAIN_MANAGEMENT_GUIDE.md` |
| Starting an Inline Agent, refining intent | Run `gsc experts guide` and paste the output |
| General orientation or first-time setup | Stay in this document, then route per the table above |
