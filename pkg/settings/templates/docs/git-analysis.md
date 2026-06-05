<!--
Component: GSC Docs Git Analysis
Block-UUID: eb0398e6-3d91-44fd-83a2-eca35cb94b6c
Parent-UUID: N/A
Version: 1.0.0
Description: Documents the gsc app analysis command group (dump, load, copy) for managing AI-generated metadata across branches, rebuilds, and worktrees. The agentic worktree workflow is the primary use case.
Language: Markdown
Created-at: 2026-05-31T17:00:00.000Z
Authors: Gemini 2.5 Flash Lite (v1.0.0)
-->


# Git Analysis Management: `gsc app analysis`

Once you have imported a repository and run an Analyzer in the GitSense Chat
web app, you have AI-generated metadata (analysis) attached to every file in
the database. This guide covers how to preserve, transfer, and restore that
analysis across branches, rebuilds, and worktrees.

---

## Why This Matters

Running an Analyzer on a large repository can take **hours or days**. The
`gsc app analysis` command group ensures that work is never lost - you can
back it up, copy it to a new branch, and restore it after a rebuild without
re-running the AI.

---

## Command Reference

| Command | Purpose |
| :--- | :--- |
| `gsc app analysis dump` | Export analysis from the database to a portable JSONL file |
| `gsc app analysis load` | Restore analysis from a JSONL file into the database |
| `gsc app analysis copy` | Copy analysis from one branch to another (chains dump → load) |

---

## The Agentic Worktree Workflow (Primary Use Case)

When working with AI agents in git worktrees, use `copy` to instantly seed
the worktree branch with the analysis from your main branch. This is the most
common use of this command.

```bash
# Create a worktree for a feature branch
git worktree add ../feature-foo feature/foo
cd ../feature-foo

# Import the worktree branch into GitSense Chat
gsc app import git --update

# Copy analysis from main to the worktree branch
gsc app analysis copy --analyzer code-intent --from-branch main --to-branch feature/foo

# Initialize the agent - it now has full Brain-Aware intelligence
gsc experts init
claude
/gitsense
```

Without this step, the agent in the worktree has no Brain data. With it, the
agent inherits everything from main - instantly.

---

## `gsc app analysis dump`

Exports all analysis for a specific analyzer and branch from the database to
a portable JSONL file.

```bash
# Dump code-intent analysis for the current branch
gsc app analysis dump --analyzer code-intent
```

The file is written to a deterministic path:

```
~/.gitsense/data/analysis/<analyzer>/<owner>/<repo>/<branch>.jsonl
```

**Key behaviors:**
- `--owner`, `--repo`, and `--branch` are inferred from `.gitsense/import-git.json` and `git rev-parse` - usually no flags needed
- **Incremental**: re-running dump only appends new records, never duplicates
- **Self-describing**: each line contains `owner`, `repo`, `branch` so the file can be loaded without extra flags

---

## `gsc app analysis load`

Restores analysis from a JSONL dump file into the database for a target branch.

```bash
# Load from the standard dump path (auto-inferred)
gsc app analysis load

# Load from a specific file
gsc app analysis load --file ./my-analysis.jsonl

# Load into a different branch (cross-branch load)
gsc app analysis load --branch feature/foo
```

**Key behaviors:**
- Target branch inferred from dump file metadata when not specified
- **Idempotent**: skips files that already have analysis - safe to re-run
- **Path-matched**: analysis is restored by matching file paths, not old chat IDs - this is why it survives a `--rebuild`

---

## `gsc app analysis copy`

Chains `dump` → `load` in a single command. Use this to propagate analysis
from one branch to another.

```bash
# Copy code-intent analysis from main to a feature branch
gsc app analysis copy --analyzer code-intent --from-branch main --to-branch feature/foo
```

The JSONL dump file is retained on disk as a byproduct, serving as both the
transfer medium and an automatic backup.

---

## Manual Backup Before Destructive Operations

`gsc app import git --rebuild` automatically dumps and restores analysis. But
for other destructive operations, a manual dump is a useful safety net:

```bash
gsc app analysis dump --analyzer code-intent
```

The file at `~/.gitsense/data/analysis/...` is your backup. You can reload it
at any time with `gsc app analysis load`.

> See `! gsc docs import-git` for details on the `--rebuild` workflow and how
> analysis is preserved during a rebuild.

---

## Packaging Insights

Once analysis is in place, you can share it with your team by packaging it
into a manifest. **This is currently a UI-only workflow** - use the GitSense
Chat web app to select the analyses you want to include and export the manifest.

A CLI-based packaging command is planned for a future release, along with
profile-based workflows for one-click manifest generation.

> See `! gsc docs brains` for how to distribute a manifest to your team.

---

<!-- LLM Guidance:
- Role: You are helping a user preserve, transfer, or restore AI-generated analysis metadata. The most common scenario is the agentic worktree workflow.
- Primary use case: User has a worktree and wants to seed it with the main branch's analysis. Give them the full 4-step worktree workflow: (1) import --update, (2) analysis copy, (3) experts init, (4) /gitsense.
- "How do I copy analysis to my worktree?": Run `gsc app analysis copy --analyzer <name> --from-branch main --to-branch <worktree-branch>`. This is the answer 80% of the time.
- "How do I back up my analysis?": Run `gsc app analysis dump --analyzer <name>`. The file is at `~/.gitsense/data/analysis/<analyzer>/<owner>/<repo>/<branch>.jsonl`.
- "How do I restore analysis after a rebuild?": `--rebuild` does this automatically. If they did a manual destructive operation, use `gsc app analysis load`.
- "How do I copy analysis between branches (not worktrees)?": Same as worktree - `gsc app analysis copy`. The `--from-branch` / `--to-branch` flags work for any two branches.
- "How do I package my analysis into a manifest?": This is UI-only. Direct them to the Chat App bridge: `gsc docs app --code <6-digit-code>`.
- Flags are usually inferred: `--owner`, `--repo`, and `--branch` are read from `.gitsense/import-git.json` and git. Users rarely need to specify them.
- Idempotency: Reassure users that `dump` and `load` are safe to re-run. `dump` appends only new records. `load` skips files that already have analysis.
- Multiple analyzers: If the user has more than one analyzer (e.g., code-intent and security-audit), they need to run `copy` once per analyzer. There is no "copy all" flag currently.
- Next Step: After analysis is copied, always suggest running `gsc experts init` to activate Brain-Aware mode in the worktree.
- Tone: Practical and reassuring. Analysis is valuable work - the user needs to trust that it's safe and recoverable.
-->
