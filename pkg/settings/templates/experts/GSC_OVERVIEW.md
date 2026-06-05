<!--
Component: GSC Overview
Block-UUID: 4950ddb6-c920-4cf2-b9e5-1dbc95109db8
Parent-UUID: 520b5510-a610-40d3-b46e-dd691a3f5f63
Version: 1.3.0
Description: Hub document for the gsc claude experts AI reference system. Updated to include the new 'guide' command in the routing table and capability groups.
Language: Markdown
Created-at: 2026-04-30T23:51:56.570Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), Gemini 2.5 Flash Lite (v1.3.0)
-->


# GSC Overview

## What is gsc?

`gsc` (GitSense CLI) transforms any Git repository into a queryable knowledge
base. It enriches the file tree with structured metadata stored in **Brains**,
enabling AI agents to find and understand code using intent and domain
attributes rather than text patterns.

## What is a Brain?

A Brain is a **constructed knowledge base** (a SQLite database) that serves as the Expert Persona's memory. It is built by importing a Manifest containing Analyzer results. Once constructed, it provides the Expert Persona with the structured intelligence needed to answer questions about your code.

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
- **Symbols/Strings:** Use `gsc grep`. (e.g., "Where is `DEFAULT_TTL` defined?").

Never perform a "blind grep." When using `gsc grep`, always enrich results
with metadata (`--fields purpose`) to eliminate noise.

## Transparent Execution & Education

The AI always displays the full `gsc` command it executes and explains its
reasoning. This transparency allows you to critique the logic, learn the
tool, and self-serve to save tokens.

## Expertise Handshake Protocol

At the start of every session, run:

```bash
gsc brains
```

Read the `Description` and `Fields` of each Brain returned. If the list is
empty, guide the user to run `gsc manifest import <uri>` before proceeding.

## The README for AI

A Manifest is the blueprint. By versioning it in your repo, you allow anyone to **construct the same Brain** simply by running `gsc manifest import`. This makes the repository's intelligence portable and accessible to all team members.

## Routing Table

| If the user asks about… | Load |
| :--- | :--- |
| Querying, filtering, coverage, insights | `GSC_QUERY_GUIDE.md` |
| File tree, grep, visualizing structure | `GSC_VISUALIZATION_GUIDE.md` |
| Missing Brain, importing a manifest | `GSC_BRAIN_MANAGEMENT_GUIDE.md` |
| Starting an Inline Agent, refining intent | Run `gsc experts guide` and paste the output |
| General orientation or first-time setup | Stay in this document, then route per the table above |
