<!--
Component: GSC Docs Quickstart
Block-UUID: 73b4d171-5ccb-429a-9d51-561be1c8fdde
Parent-UUID: e2f2d7e6-2b47-4bbc-936a-5a9fe27e0921
Version: 1.1.0
Description: A choose-your-path quickstart guide that routes users from zero to a fully intelligent coding session. Covers the smart repo fast path, the fresh start path, the human developer gsc rg workflow, and the agentic worktree workflow. Designed to be the first document a user or agent reads after gsc docs init.
Language: Markdown
Created-at: 2026-05-31T16:52:09.267Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0)
-->


# GitSense Chat: Quickstart Guide

GitSense Chat makes your codebase queryable. Instead of blindly searching files,
you and your coding agents can ask questions and get answers grounded in structured
knowledge. This guide gets you from zero to a fully intelligent session.

---

## What Do You Want to Do?

Choose the path that describes your situation:

| My Situation | Start Here |
| :--- | :--- |
| I cloned a repo and it has a `.gitsense/manifests/` folder | [Path A: Smart Repo](#path-a-smart-repo-fastest) |
| I want to import a repo and start from scratch | [Path B: Fresh Start](#path-b-fresh-start) |
| I just want smarter searches without opening a chat | [Path C: Human Developer](#path-c-human-developer-gsc-rg) |
| I'm working with an AI agent in a worktree | [Path D: Agentic Workflow](#path-d-agentic-worktree-workflow) |

---

## Path A: Smart Repo (Fastest)

A "smart repo" ships with a Brain Manifest already committed. Anyone who clones
it can be up and running in minutes.

```bash
# 1. Clone and enter the repository
git clone <repo-url>
cd <repo-name>

# 2. Build the Brain from the committed manifest
gsc manifest import code-intent

# 3. Start your agent
claude

# 4. Initialize Brain-Aware mode
/gitsense

# 5. Ask questions
"Tell me about this repo"
"Where is authentication handled?"
"Show me all files related to payments"
```

Your agent now has structured knowledge of the entire codebase. It will use
`gsc query`, `gsc rg`, and `gsc tree` on your behalf - no grepping required.

> **No `/gitsense` skill?** Run `gsc experts setup-agent claude` once to install
> it. For other agents, run `gsc experts init` and then tell your agent to read
> `.gitsense/experts-context.md`.

> **Manifest not found?** Run `gsc manifest list` to see what Brains are available.
> If none, see [Path B: Fresh Start](#path-b-fresh-start).

---

## Path B: Fresh Start

You have a repository that has never been imported into GitSense Chat.

### Step 1: Install and Start the Chat App

If the GitSense Chat App is not yet installed:

```bash
! gsc docs install
```

If it is installed but not running:

```bash
! gsc docs lifecycle
```

### Step 2: Import Your Repository

```bash
gsc app import git --owner <your-org> --repo <repo-name>
```

This creates a snapshot of your repository in the GitSense Chat database. For
large repositories, the initial import may take a few minutes. Subsequent updates
(`gsc app import git --update`) complete in seconds.

> See `! gsc docs import-git` for full import options, including incremental
> updates, rebuilds, and disk management.

### Step 3: Create a Brain in the Chat App

Open the GitSense Chat App (`http://localhost:3357`), start a new chat for your
repository, and run a **code-intent** analyzer. This is a UI-driven step - the
analyzer reads every file and extracts structured metadata (purpose, layer, risk,
ownership, etc.).

> For help with this step, use the Chat App bridge:
> `gsc docs app --code <6-digit-code>`

### Step 4: Export and Import the Manifest

Once the analyzer completes, export the manifest from the Chat App and import it
into your repository:

```bash
# Save the manifest to the standard location
mkdir -p .gitsense/manifests
mv ~/Downloads/code-intent.json .gitsense/manifests/

# Build the Brain
gsc manifest import code-intent
```

**Optional but recommended**: Commit the manifest so teammates can skip Steps 1-3:

```bash
git add .gitsense/manifests/code-intent.json
git commit -m "Add code-intent Brain manifest"
```

### Step 5: Connect Your Agent

```bash
gsc experts init   # generates .gitsense/experts-context.md
claude
/gitsense          # agent reads the Brain context automatically
```

---

## Path C: Human Developer (`gsc rg`)

You don't need a chat interface. You just want smarter searches in your terminal.

Once a Brain is imported (`gsc manifest import <name>`), replace `rg` with
`gsc rg`. Every match is enriched with Brain metadata - you see not just the
matching line, but the *purpose* of the file it came from.

```bash
# Standard ripgrep search
rg "handleAuth"

# GitSense enriched search - same syntax, but every match includes Brain context
gsc rg "handleAuth"

# Add --fields to control which Brain field appears alongside each result
gsc rg "handleAuth" --fields layer,risk_level
```

Use `gsc query` for concept-based searches (no pattern needed):

```bash
# Find all files related to authentication
gsc query --filter "layer:auth"

# Find all files flagged as high-risk
gsc query --filter "risk_level:high"
```

> Run `! gsc docs experts` for the full query and visualization reference.

---

## Path D: Agentic Worktree Workflow

You're using `git worktree` so an agent can work on a feature branch while you
continue on main.

```bash
# 1. Create the worktree
git worktree add ../feature-foo feature/foo
cd ../feature-foo

# 2. Import the worktree branch into GitSense Chat
gsc app import git --update

# 3. Copy analysis from main - avoids hours of re-running analyzers (see ! gsc docs git-analysis)
gsc app analysis copy --analyzer code-intent --from-branch main --to-branch feature/foo

# 4. Initialize the agent in the worktree
gsc experts init
claude
/gitsense
```

The agent now has the same Brain-Aware intelligence in the worktree as on main.
AI-generated analysis that took hours or days to produce is transferred instantly.

> **Packaging insights**: Once your feature work is complete, use the GitSense
> Chat UI to package the updated analysis into a new manifest. CLI-based packaging
> is planned for a future release.

---

## Reference: Document Map

Use `! gsc docs <topic>` to load any of these guides:

| Topic | What It Covers |
| :--- | :--- |
| `install` | Installing the GitSense Chat App (Native or Docker) |
| `lifecycle` | Starting, stopping, restarting, and checking status |
| `admin` | API keys, LLM models, environment configuration |
| `locate` | Where the app, databases, and config files are stored |
| `import-git` | Importing repositories, incremental updates, rebuilds |
| `git-analysis` | Preserving AI-generated metadata across branches, rebuilds, and worktrees |
| `brains` | Manifests vs. Brains, creating Brains, publishing, enterprise patterns |
| `experts` | Connecting coding agents to the Brain intelligence layer |
| `app` | Comprehensive Chat App guide (use `--code` to inject into chat) |

---

<!-- LLM Guidance:
- Role: You are the triage officer. Your job is to identify which path applies to the user and guide them through it step by step. Do not present all paths at once - ask one clarifying question and route accordingly.
- Triage question 1: "Have you cloned a repository that already has a `.gitsense/manifests/` folder?" → Yes: Path A. No: proceed.
- Triage question 2: "Do you have the GitSense Chat App installed and running?" → No: fetch `! gsc docs install` immediately. Yes: proceed.
- Triage question 3: "Have you already imported this repository into GitSense Chat?" → No: guide through gsc app import git. Yes: proceed.
- Triage question 4: "Do you have a Brain (manifest file) for this repository?" → No: fetch `! gsc docs brains`. Yes: proceed to gsc experts init.
- Human developer path: If the user asks about gsc rg, gsc query, or gsc tree syntax, fetch `! gsc docs experts`. These commands require a Brain to be active.
- Agentic workflow: If the user mentions worktrees or feature branches, highlight the analysis copy command before experts init. This is the most common missed step.
- For the smart repo fast path: Check `gsc manifest list` first. If Brains already exist, skip directly to `gsc experts init`.
- For the fresh start path: The Chat App must be running before the user can create an analyzer. Confirm this before proceeding to Step 3.
- Future behavior (not yet implemented): `/gitsense` will eventually auto-detect unimported manifests in `.gitsense/manifests/` and prompt the user. For now, `gsc manifest import <name>` is the manual step.
- Tone: Direct and action-oriented. Each step should end with a clear command. No philosophy - the user is here to get something done.
- Next Step: Ask "Which of the four paths describes your situation?" or infer from context if the user has already provided enough information.
-->

