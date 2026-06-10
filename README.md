<!--
Component: gsc-cli README
Block-UUID: 5c01d958-b93a-46dd-9f83-65c5d47a15dc
Parent-UUID: N/A
Version: 1.0.0
Description: Reworked README positioning gsc-cli as the terminal half of the GitSense two-part system, with focus on trust, transparency, and what this repository is.
Language: Markdown
Created-at: 2026-05-31T17:26:27.671Z
Authors: Claude Code - Sonnet (v1.0.0)
-->


# gsc - GitSense CLI

`gsc` is the terminal half of [GitSense Chat](https://github.com/gitsense/chat).

GitSense is a two-part system:

- **[The Chat App](https://github.com/gitsense/chat)** imports repositories, runs analyzers, extracts structured domain knowledge, and packages that knowledge into portable Manifests.
- **This repository** is the Go source for the `gsc` binary. The CLI runs on your machine, imports those Manifests, builds local Brains, and makes the intelligence available in your terminal, scripts, and coding agents.

If you are reading this, you may already have `gsc` installed. This repository exists so you can review exactly what the CLI does and build it yourself.

## What This Repository Is

This is the source code for the `gsc` command-line tool. It is the trusted, inspectable part of the GitSense workflow that runs inside your local repositories.

The CLI handles:

- importing and managing Manifests
- building local Brain databases from those Manifests
- enriched terminal searches powered by Brain metadata
- context generation for coding agents
- GitSense Chat app installation, lifecycle, import, and analysis management

The core Brain/search commands operate locally against files and SQLite databases. Commands that explicitly install software, import remote Manifests, publish Manifests, or talk to the Chat app may contact the configured endpoint.

## A Note on AI, Go, and v0.1.0

This repository is approximately 99.9% AI-generated. It was guided by a seasoned software developer who was not proficient in Go when the project began.

That context matters because GitSense is built around eliminating blind discovery. Modern AI can generate code quickly, but complex software still depends on knowing what to build, where to look, what context matters, and how to keep changes coherent over time. GitSense is both the product and a working example of that workflow.

This is also a `v0.1.0` project. Some implementation choices may not reflect ideal Go design, and parts of the codebase will evolve as the project matures. The point of this repository is not to present perfect idiomatic Go. It is to show that, with strong human direction and better repository intelligence, AI can help build, extend, and maintain substantial software even when the human is not already fluent in the target language.

Go was chosen for practical distribution reasons. GitSense Chat itself is a Node.js application, which matched the author's existing JavaScript experience and the needs of the web app. The `gsc` CLI is written in Go because it needs to compile into easy-to-install binaries that let users run commands like:

```bash
gsc app native install
```

That binary-first workflow is central to making GitSense easy for humans and agents to use from the terminal.

## Manifests and Brains

The important distinction:

- A **Manifest** is a portable JSON artifact created from analysis. It can be committed, shared by URL, or published through GitSense Chat.
- A **Brain** is the local SQLite database that `gsc` builds from a Manifest.  It is what `gsc query`, `gsc rg`, and `gsc tree` read.

Manifests are what teams distribute. Brains are what developers and agents use locally.

## What `gsc` Does

### Intelligence Commands

These commands work once a Manifest has been converted to a GitSense Brain (SQLite database).

| Command | Purpose |
| :--- | :--- |
| `gsc manifest import <uri>` | Build a Brain from a local or remote Manifest |
| `gsc manifest list` | Show imported Brains available in the current repository |
| `gsc manifest publish <path>` | Publish a Manifest to your GitSense Chat server |
| `gsc brains` | List active Brains and their metadata fields |
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
