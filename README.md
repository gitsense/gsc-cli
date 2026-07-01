<!--
Component: gsc-cli README
Block-UUID: d5f493cc-7035-43fc-a3d0-d013fadec153
Parent-UUID: 5c01d958-b93a-46dd-9f83-65c5d47a15dc
Version: 1.5.0
Description: Added gsc topics and gsc knowledge commands for unified knowledge discovery.
Language: Markdown
Created-at: 2026-05-31T17:26:27.671Z
Authors: Claude Code - Sonnet (v1.0.0), Codex GPT-5 (v1.1.0), MiMo-v2.5-pro (v1.2.0), claude-opus-4-8 (v1.3.0), MiMo-v2.5-pro (v1.4.0), MiMo-v2.5-pro (v1.5.0)
-->


# gsc - GitSense CLI

`gsc` is the terminal half of [GitSense Chat](https://github.com/gitsense/chat).

GitSense is a two-part system:

- **[The Chat App](https://github.com/gitsense/chat)** imports repositories, runs analyzers, extracts structured domain knowledge, and packages that knowledge into portable Manifests.
- **This repository** is the Go source for the `gsc` binary. The CLI runs on your machine, imports those Manifests, builds local Brains (SQLite databases), and makes the intelligence available in your terminal, scripts, and coding agents.

Brains can be built from analyzer output or grown from development sessions as lessons. Both live with the repository. Both are queryable.

Here is what that split means in practice. Plain ripgrep finds the string:

    rg cache

`gsc` returns the same matches plus what each file is for, so the agent can drop the junk before it opens anything:

    gsc rg --db code-intent --fields purpose cache

    crates/ignore/src/dir.rs
    purpose: Modify this file to change how ignore rules are loaded, matched, and prioritized during directory traversal, including support for custom ignore files and git integration.

The App built that purpose line. The CLI delivered it. Same search, but now the agent looks first and thinks second.

If you are reading this, you may already have `gsc` installed. This repository exists so you can review exactly what the CLI does and build it yourself.

## What This Repository Is

This is the source code for the `gsc` command-line tool. It is the trusted, inspectable part of the GitSense workflow that runs inside your local repositories.

The CLI handles:

- importing and managing Manifests
- building local Brain databases from those Manifests
- capturing durable lessons from development sessions
- enriched terminal searches powered by Brain metadata
- context generation for coding agents
- GitSense Chat app installation, lifecycle, import, and analysis management

The core Brain/search commands operate locally against files and SQLite databases. Commands that explicitly install software, import remote Manifests, publish Manifests, or talk to the Chat app may contact the configured endpoint.

## Topics and Knowledge Discovery

GitSense organizes repository knowledge around **topics** — broad navigational domains that span lessons, notes, and rules. Every knowledge item must reference exactly one primary topic, with optional related topics.

### Topics Commands

These commands manage the shared topic registry used by lessons, notes, and rules.

| Command | Purpose |
| :--- | :--- |
| `gsc topics add <slug>` | Register a new topic with a description |
| `gsc topics list` | List all registered topics |
| `gsc topics show <slug>` | Show topic details |
| `gsc topics search <query>` | Search topics by slug or description |
| `gsc topics update <slug>` | Update a topic's description |
| `gsc topics migrate` | Migrate existing records to the new topic format |

### Knowledge Commands

These commands provide unified search and discovery across lessons, notes, and rules.

| Command | Purpose |
| :--- | :--- |
| `gsc knowledge search <query>` | Search across all knowledge types with relevance ranking |
| `gsc knowledge list --topic <slug>` | List all items in a specific topic |

**Discovery flow:**

```bash
# General question → search all knowledge
 gsc knowledge search "manifest import performance"

# Known topic → browse items
 gsc knowledge list --topic data-layer

# Filter by type
 gsc knowledge search "database" --type lessons
 gsc knowledge list --topic data-layer --type rules

# Control output
 gsc knowledge search "lessons" --limit 10 --truncate 80 -o json
```

**Search ranking:**
1. Exact topic match (highest)
2. Exact tag match
3. Summary term match
4. Body term match (lowest)

## A Note on AI, Go, and Project Maturity

This repository is approximately 99.9% AI-generated. It was guided by a seasoned software developer who was not proficient in Go when the project began.

That context matters because GitSense is built around eliminating blind discovery. Modern AI can generate code quickly, but complex software still depends on knowing what to build, where to look, what context matters, and how to keep changes coherent over time. GitSense is both the product and a working example of that workflow.

This is also an early-stage project, currently preparing for `v0.2.0`. Some implementation choices may not reflect ideal Go design, and parts of the codebase will evolve as the project matures. The point of this repository is not to present perfect idiomatic Go. It is to show that, with strong human direction and better repository intelligence, AI can help build, extend, and maintain substantial software even when the human is not already fluent in the target language.

Go was chosen for practical distribution reasons. GitSense Chat itself is a Node.js application, which matched the author's existing JavaScript experience and the needs of the web app. The `gsc` CLI is written in Go because it needs to compile into easy-to-install binaries that let users run commands like:

```bash
gsc app native install
```

That binary-first workflow is central to making GitSense easy for humans and agents to use from the terminal.

## Manifests and Brains

The important distinction:

- A **Manifest** is a portable JSON artifact that packages structured repository metadata. It can be committed, shared by URL, or published through GitSense Chat.
- A **Brain** is the local SQLite database that `gsc` builds from a Manifest. It is what `gsc query`, `gsc rg`, and `gsc tree` read.

Manifests are what teams distribute. Brains are what developers and agents use locally.

There are two common ways Brains are created:

- **Analyzer Brains** come from structured analysis of a codebase. GitSense Chat can produce these by running analyzers over repository files and exporting metadata such as file purpose, keywords, ownership, risk, or any custom fields an analyzer defines.
- **Lessons Brains** grow from development sessions. When a human or agent discovers a useful constraint, coupling, workflow, design decision, or gotcha, it can be saved as a lesson. Lessons are committed as records in `.gitsense/lessons/records.jsonl`; `gsc lessons build` projects those records into a generated Manifest and imports the local `gsc-lessons` Brain.

## What `gsc` Does

### Intelligence Commands

These commands work once a Manifest has been imported into a repository.

| Command | Purpose |
| :--- | :--- |
| `gsc manifest import <uri>` | Build a Brain from a local or remote Manifest |
| `gsc manifest list` | Show imported Brains available in the current repository |
| `gsc manifest publish <path>` | Publish a Manifest to your GitSense Chat server |
| `gsc brains` | List active Brains and their metadata fields |
| `gsc brains delete <db>` | Delete a Brain (SQLite database) |
| `gsc query` | Find files by metadata: concept, layer, ownership, risk, or custom fields |
| `gsc rg` | Enriched ripgrep: text matches plus Brain metadata |
| `gsc grep` | Alias-compatible enriched search for grep-oriented workflows |
| `gsc tree` | Visualize the tracked file tree enriched with metadata |
| `gsc insights` | Analyze metadata value distributions across the codebase |
| `gsc coverage` | Show analyzed versus unanalyzed file coverage |

### Agent Commands

| Command | Purpose |
| :--- | :--- |
| `gsc experts init` | Generate `.gitsense/experts-context.md` for the current repository |
| `gsc experts status` | Check whether agent context is stale |
| `gsc experts guide` | Load the consultation guide for structured agent workflows |
| `gsc experts setup-agent claude` | Install the `/gitsense` skill for Claude Code |
| `gsc docs help` | Browse AI-facing documentation topics (about, brains, experts, import, install, lifecycle, …) |

### Lesson Commands

These commands capture and rebuild durable repository knowledge from development sessions. `gsc experts init` will also build the `gsc-lessons` Brain automatically when committed lesson records exist and the Brain is missing.

| Command | Purpose |
| :--- | :--- |
| `gsc lessons add` | Create and stage a lesson in one shot from flags, `--from-file`, or `--stdin` |
| `gsc lessons draft new` | Start an interactive draft, then `draft validate` / `review` / `commit --target <repo\|personal>` / `discard` |
| `gsc lessons update --target <repo\|personal> --id <id>` | Stage and commit a full replacement of an existing lesson |
| `gsc lessons list [--scope <all\|repo\|personal>]` | List and filter committed lessons (`--tag` / `--topic` / `--file` / `--importance`, `-o json`) |
| `gsc lessons search <query> [--scope <all\|repo\|personal>]` | Full-text search across committed lessons |
| `gsc lessons tags [--scope <all\|repo\|personal>]` | Show the tag vocabulary with lesson counts |
| `gsc lessons overview [--scope <all\|repo\|personal>]` | Print a human-readable digest of all lessons |
| `gsc lessons show <id> [--scope <all\|repo\|personal>]` | Show a committed lesson (`-o json` supported) |
| `gsc lessons delete <id> --target <repo\|personal>` | Delete a lesson and rebuild the selected lessons store |
| `gsc lessons build --target <repo\|personal>` | Rebuild the generated lessons Manifest and Brain from committed records |

### Rules Commands

These commands define queryable guardrails and conventions for coding agents. Agents can consult rules before modifying files to follow project conventions. `gsc experts init` will also build the `gsc-rules` Brain automatically when committed rule records exist and the Brain is missing.

| Command | Purpose |
| :--- | :--- |
| `gsc rules new --target <repo\|personal>` | Create a rule from flags, `--from-file`, `--stdin`, or `--template` |
| `gsc rules update --target <repo\|personal> --id <id>` | Update an existing rule (requires `--changelog`) |
| `gsc rules delete <id> --target <repo\|personal>` | Delete a rule and rebuild the selected rules store |
| `gsc rules get --file <path> [--scope <all\|repo\|personal>]` | Query rules for a specific file (returns `git_root` in JSON) |
| `gsc rules get --glob <pattern> [--scope <all\|repo\|personal>]` | Query rules matching a glob pattern |
| `gsc rules get --tag <tag> [--scope <all\|repo\|personal>]` | Query rules by tag |
| `gsc rules changelog --file <path>` | Query changelog for rules (new) |
| `gsc rules list [--scope <all\|repo\|personal>]` | List and filter rules (`--tag` / `--topic` / `--importance`, `-o json`) |
| `gsc rules search <query> [--scope <all\|repo\|personal>]` | Full-text search across rules |
| `gsc rules tags [--scope <all\|repo\|personal>]` | Show the tag vocabulary with rule counts |
| `gsc rules overview [--scope <all\|repo\|personal>]` | Print a human-readable digest of all rules |
| `gsc rules show <id> [--scope <all\|repo\|personal>]` | Show a rule in detail (`-o json` supported) |
| `gsc rules build --target <repo\|personal>` | Rebuild the generated rules Manifest and Brain from committed records |

### Notes Commands

These commands provide a searchable scratchpad for coding agents. Notes are for research, context, and observations that help agents understand the codebase. Unlike rules (guardrails) and lessons (learned constraints), notes are a scratchpad for things you want to keep track of.

| Command | Purpose |
| :--- | :--- |
| `gsc notes add --target <repo\|personal>` | Create a note from flags, `--from-file`, `--stdin`, or `--template` |
| `gsc notes update --target <repo\|personal> --id <id>` | Update an existing note |
| `gsc notes delete <id> --target <repo\|personal>` | Delete a note and rebuild the selected notes store |
| `gsc notes get --file <path> [--scope <all\|repo\|personal>]` | Query notes for a specific file |
| `gsc notes get --glob <pattern> [--scope <all\|repo\|personal>]` | Query notes matching a glob pattern |
| `gsc notes get --tag <tag> [--scope <all\|repo\|personal>]` | Query notes by tag |
| `gsc notes list [--scope <all\|repo\|personal>]` | List and filter notes (`--tag` / `--topic` / `--importance`, `-o json`) |
| `gsc notes search <query> [--scope <all\|repo\|personal>]` | Full-text search across notes |
| `gsc notes tags [--scope <all\|repo\|personal>]` | Show the tag vocabulary with note counts |
| `gsc notes overview [--scope <all\|repo\|personal>]` | Print a human-readable digest of all notes |
| `gsc notes show <id> [--scope <all\|repo\|personal>]` | Show a note in detail (`-o json` supported) |
| `gsc notes build --target <repo\|personal>` | Rebuild the generated notes Manifest and Brain from committed records |

### Pi Commands

These commands manage Pi coding session data: importing session logs, querying past conversations, resuming sessions, and monitoring context usage.

| Command | Purpose |
| :--- | :--- |
| `gsc pi -r`, `--resume` | Interactive session picker with split-pane preview |
| `gsc pi -b`, `--brains` | Show session statistics (tokens, model, files) |
| `gsc pi --hud` | Pick a session and open in tmux split with HUD sidebar |
| `gsc pi guide` | Print detailed reference documentation for gsc pi |
| `gsc pi sessions sync` | Import Pi session JSONL files into the SQLite mirror |
| `gsc pi sessions list` | List imported sessions |
| `gsc pi sessions query` | Full-text search across sessions |
| `gsc pi sessions show <id>` | Show detailed session information |
| `gsc pi sessions verify` | Verify session import fidelity |

### App Management Commands

| Command | Purpose |
| :--- | :--- |
| `gsc app native install` | Install the GitSense Chat web app |
| `gsc app native start` | Start the app |
| `gsc app native stop` | Stop the app |
| `gsc app import git` | Import a repository into GitSense Chat |
| `gsc app import git --update` | Update an existing repository import |
| `gsc app analysis dump` | Back up AI-generated analysis metadata |
| `gsc app analysis load` | Restore analysis metadata |
| `gsc app analysis copy` | Copy analysis between branches or worktrees |

## Installation

Download a prebuilt binary for Linux, macOS, or Windows from the
[GitSense Chat releases page](https://github.com/gitsense/chat/releases).

You can also install through the GitSense Chat installer:

```bash
curl https://raw.githubusercontent.com/gitsense/chat/refs/heads/main/install.sh | bash
```

## Building from Source

Go 1.21 or later is required.

```bash
git clone https://github.com/gitsense/gsc-cli
cd gsc-cli
make build
```

The binary is written to `dist/gsc`. Add it to your `PATH` or alias it:

```bash
alias gsc="$(pwd)/dist/gsc"
gsc --help
```

Check the CLI version with either form:

```bash
gsc --version
gsc version
```

Runtime requirements depend on what you use:

- **Git**: repository detection and tracked-file operations
- **ripgrep**: `gsc rg` and `gsc grep`
- **Node.js 18+**: native GitSense Chat app installation and execution
- **Docker**: Docker preview mode for the Chat app

## Source Provenance

This repository experiments with traceable AI-assisted development. Many source files include metadata such as:

```go
/*
Component: Example
Block-UUID: 2f841e0c-b452-4b5a-831c-7d45c17977a7
Parent-UUID: 0f44a7e5-01e0-464f-a051-258f4779687c
Version: 1.4.0
Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), ...
*/
```

- `Block-UUID`: unique identifier for a component block
- `Parent-UUID`: inheritance link to a prior version
- `Authors`: model or author history for generated versions

That metadata is not required to use the CLI. It exists so provenance can be reviewed as an artifact instead of hidden in chat history.

## Related

- [GitSense Chat](https://github.com/gitsense/chat): the app that builds the
  intelligence this CLI consumes

## License

See [LICENSE](LICENSE).
