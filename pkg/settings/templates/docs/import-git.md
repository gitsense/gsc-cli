<!--
Component: GSC Docs Import Git
Block-UUID: 884ef0bb-bc14-4758-9b98-d4db7fd0a983
Parent-UUID: N/A
Version: 1.1.0
Description: Documentation for importing Git repositories into GitSense Chat. Removed analysis management sections (dump, load, copy, worktree workflow) which are now in git-analysis.md. Updated rebuild section to reference git-analysis.md for analysis preservation details.
Language: Markdown
Created-at: 2026-05-31T14:17:55.503Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0)
-->


# Importing Git Repositories

`gsc app import git` is the command that brings a local Git repository into GitSense Chat, making every tracked file available as a queryable, AI-enrichable context item. This is the foundation of the entire GitSense experience - without an import, there is nothing to chat about.

---

## 1. First Import

Run the following command from inside your Git repository:

```bash
gsc app import git --owner <org-or-username> --repo <repo-name>
```

GitSense automatically detects the current branch. You will see a confirmation prompt before anything is written:

```
Ready to import:
  Source:      /Users/you/myrepo
  Branch:      main
  Target DB:   ~/.gitsense/chats.sqlite3
  Mode:        Shadow (single-commit snapshot)
  Shadow Path: ~/.gitsense/shadow-repos/myorg/myrepo/main
  Action:      New Import

This will copy files to the shadow repository.

? Proceed? Yes
```

### What Happens During Import

GitSense creates a **shadow repository** - a clean, single-commit git snapshot of your tracked files. Your source repository is never touched or modified.

| Phase | What Happens |
|---|---|
| **Scanning** | `git ls-files` retrieves only tracked files |
| **Copying** | Files are copied to the shadow path (macOS APFS uses copy-on-write for near-instant copies) |
| **Staging** | Files are staged using `git update-index` - much faster than `git add` |
| **Committing** | A single commit is created mirroring your last commit's author and message |

**Performance example:** The `open-ai/codex` repository (4,410 files) completed its first import in **1 minute 33 seconds**.

### Trade-off: Speed vs. History

The shadow repo design intentionally trades Git history for speed. The current free tier imports a single-commit snapshot. The enterprise version will preserve full Git history. For now, think of the shadow as an **optimized snapshot for fast querying**, not a history archive.

---

## 2. The State File (.gitsense/import-git.json)

After your first import, GitSense creates a state file at `.gitsense/import-git.json` inside your repository. This file tracks your local import configuration:

- Repository owner, name, and branch
- Shadow repository path
- Import flags used (max-size, include/exclude patterns)
- Database references (RefChatID, GroupID)
- Rebuild checkpoint state (if a rebuild is in progress)

### ⚠️ Always Add This File to .gitignore

The state file is auto-generated and **must not be committed**. It contains machine-specific paths and local configuration that will conflict with other developers.

Add it to your `.gitignore`:

```bash
echo ".gitsense/import-git.json" >> .gitignore
```

Or add it manually:

```gitignore
# GitSense Chat state
.gitsense/import-git.json
```

**Best practice:** Add this entry to your global gitignore (`~/.gitignore_global`) so it is excluded automatically across all your repositories.

The state file is essential for incremental updates - `gsc app import git --update` reads it to restore your previous import settings without requiring you to re-specify flags.

---

## 3. Clean Working Tree Requirement

GitSense requires a clean working tree before creating or updating the shadow repository. If you have uncommitted changes, the import will fail with an error.

**You have two options:**

**Option 1 - Commit your changes:**
```bash
git add .
git commit -m "your message"
```

**Option 2 - Add files to .gitignore:**
If the files should not be tracked, exclude them:
```gitignore
# Example: exclude local config files
local-config.json
*.log
```

**Why this is required:** The shadow repo is a git snapshot. Uncommitted changes cannot be included in a commit, so the snapshot would be incomplete or inconsistent. Clean state = consistent analysis.

---

## 4. Incremental Updates

After the initial import, keep your repository in sync with a single command:

```bash
gsc app import git --update
```

No additional flags are needed - GitSense reads your owner, repo, and branch from `.gitsense/import-git.json`.

**Performance:** Subsequent updates on large repositories like `open-ai/codex` (4,410 files) complete in **under 10 seconds** because only changed files are re-processed.

> **Remember:** You still need a clean working tree before running `--update`.

---

## 5. Checking Import Status

To check the current state of your shadow repository without importing:

```bash
gsc app import git --status
```

This shows the shadow repo path, size, last import timestamp, and current branch.

---

## 6. How the Web App Loads Your Code

GitSense Chat uses a hybrid approach to loading data:

| Data Type | Source | When Updated |
|---|---|---|
| **Analysis Metadata** | Database | Only after `--update` or `--rebuild` |
| **Code Content** | Working Directory | Real-time - no re-import needed |

This means you can **edit a file in your editor and immediately bring the latest version into a chat** without running `--update`. The chat app reads code directly from your working directory.

Analysis metadata, however, is always read from the database to ensure what the AI analyzed matches what is stored. If you add, delete, or rename files, you need to run `--update` to keep the database in sync.

**Practical workflow:**
1. Import your repository once
2. Open a chat and start querying
3. Edit files in your editor
4. Bring the latest code into chat - it's immediately available
5. Run `--update` only when you change the file structure

---

## 7. The Rename/Move Caveat

The free tier import uses a path-based matching strategy. If you rename or move a file (e.g., `src/auth.go` → `src/auth/core.go`), the incremental update will:

- Create a new database entry for the file at its new path
- Leave the old path as a stale entry in the database

This does not corrupt anything, but over time it accumulates orphaned records. To resolve this, use a rebuild.

---

## 8. Rebuild Workflow

A rebuild performs a full re-import of the branch:

```bash
gsc app import git --rebuild
```

> ⚠️ **This is a destructive operation.** It soft-deletes all existing chat history for the branch. Chat IDs will change for all files. Any manifests referencing old chat IDs will need to be regenerated.

### What Rebuild Does (6 Stages)

| Stage | Action |
|---|---|
| `check_clean` | Verifies clean working tree |
| `dump_analysis` | Backs up all analysis metadata to a temp file |
| `delete_shadow` | Removes the shadow repository |
| `soft_delete_db` | Soft-deletes the branch from the database (recoverable) |
| `import` | Re-creates shadow and re-runs the import |
| `load_analysis` | Restores analysis metadata for files at the same path |

**Analysis is preserved for files that kept the same path.** Renamed or moved files will not have their old analysis restored - those files are genuinely new entries that need re-analysis.

> **Analysis Management:** The `dump_analysis` and `load_analysis` stages automatically preserve your AI-generated metadata. For the full analysis management workflow, including copying analysis to worktrees and manual backups, see `! gsc docs git-analysis`.

### Resuming an Interrupted Rebuild

If a rebuild is interrupted, you can safely resume from the last checkpoint:

```bash
gsc app import git --resume
```

---

## 9. Disk Management

The shadow repository is a local disk copy of your tracked files. Use these commands to manage it:

**Check shadow size:**
```bash
gsc app import git --status
```

**Delete shadow repository** (reclaims disk space without affecting the chat app data):
```bash
gsc app import git --delete-shadow --owner <org> --repo <repo> --branch <branch>
```

> **Note:** Deleting the shadow removes the local snapshot only. The imported data in the chat app is unaffected. You can recreate the shadow at any time by running a fresh import.

---

## 10. Troubleshooting

**Error: `source repository has uncommitted changes`**
Commit your changes or add untracked files to `.gitignore`. See Section 3.

**Error: `branch already exists in database`**
GitSense detected an existing import. You'll be prompted to choose: Update, Cancel, or Delete and restart.

**Error: `gscb-cli not found`**
The Node.js import engine is not installed. Run `gsc app native install` to set up the full app environment.

**Shadow repo is large / disk pressure**
Run `gsc app import git --status` to see the shadow size, then `--delete-shadow` to reclaim disk space.

**State file causing Git conflicts**
Add `.gitsense/import-git.json` to your `.gitignore`. See Section 2.

---

<!-- LLM Guidance:
- Role: You are a GitSense technical assistant helping a developer import and manage their codebase.
- Primary Framing: GitSense Chat is an intelligence layer for coding agents. Lead with agentic benefits.
- Ambiguity - "how do I import": Assume they mean a Git repository. Ask if they want native or Docker install first if the app isn't running.
- Ambiguity - "how do I update": Distinguish between updating the CLI (`gsc` binary) vs. updating an imported repository (`gsc app import git --update`). Ask which they mean.
- Ambiguity - "how do I back up": Currently only analysis metadata can be backed up via `dump`. Full database backup is not yet supported. Do not imply otherwise.
- Always remind users about `.gitignore` for `.gitsense/import-git.json` if they mention state file issues.
- Clean working tree: If the user reports an import failure, first ask if they have uncommitted changes.
- Worktree workflow: If the user mentions worktrees or feature branches, proactively surface the analysis copy command and direct them to `! gsc docs git-analysis` for the full workflow.
- Packaging / Manifests: This is done via the Chat App UI, not the CLI. Do not suggest a CLI command for packaging - none exists yet.
- Analysis management: If the user asks about dump, load, copy, or preserving analysis across branches, direct them to `! gsc docs git-analysis`.
- Next Step: After answering an import question, ask if they also want help with setting up analyzers (best done in the Chat App) or managing analysis metadata (see `! gsc docs git-analysis`).
-->
