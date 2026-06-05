<!--
Component: GSC Docs Experts
Block-UUID: 2e867d7e-4525-4138-95b4-73f390522bdd
Parent-UUID: 38d55131-dd24-40e6-92d8-c074057d8ca8
Version: 1.0.2
Description: Documentation for the gsc experts command suite. Clarified that setup-agent is Claude Code only, emphasized gsc experts init as the universal method for all agents, and added explicit instruction that users must tell the AI to read the context file after running !gsc experts init.
Language: Markdown
Created-at: 2026-05-31T15:22:41.429Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
-->


# Making Your Coding Agent Smarter: `gsc experts`

`gsc experts` connects your coding agent (Claude Code, Cursor, Aider, etc.) to
the intelligence layer in your repository. Once connected, your agent stops
guessing and starts querying - using structured Brain metadata instead of blind
file searches.

## Quick Reference

| Command | Purpose |
| :--- | :--- |
| `gsc experts init` | Generate the experts context file for the current repository |
| `gsc experts status` | Check if the context file is stale |
| `gsc experts forget` | Remove the context file and reset the agent |
| `gsc experts guide` | Load the consultation guide for Inline Agent workflows |
| `gsc experts setup-agent <agent>` | One-time install of the `/gitsense` skill for your agent |

---

## One-Time Setup (Per Machine)

```bash
gsc experts setup-agent claude   # for Claude Code
```

This installs a `/gitsense` skill in your agent's command directory, so the
agent can self-initialize in any repository with a single command.

> **Note:** The `setup-agent` command is currently supported only for Claude Code.
> Support for other agents will be added in future releases.

## Every Agent Session

At the start of every agent session in a repository that has Brains:

```bash
gsc experts init
```

Or, if you have the Claude Code skill installed (recommended):

```
/gitsense
```

This generates `.gitsense/experts-context.md` - a single file that tells your
agent what Brains exist, what fields they expose, and how to query them
efficiently. Your agent reads it automatically.

## Universal Method (All Agents)

For any coding agent (Claude Code, Cursor, Aider, etc.), the universal way
to connect to Brains is to run:

```bash
! gsc experts init
```

**Important:** After running this command, you must tell your agent to read
the output which will instruct the agent to read the context file.

The `/gitsense` skill (Claude Code only) does this automatically, but for
all other agents, this follow-up step is required.

<!-- LLM Guidance:
- Role: You are the last step before the user gets a fully intelligent agent. Your job is to get them over the line quickly.
- If no Brains exist: Immediately pivot to `! gsc docs brains`. Do not proceed without a Brain.
- Primary sequence to communicate: (1) setup-agent once (Claude Code only), (2) experts init each session, (3) ask in Chat App.
- If user asks about gsc tree / gsc query / gsc rg syntax: Tell them they don't need to learn these. Once they run `gsc experts init` in the Chat App, the agent has full documentation on all commands and will use them on the user's behalf. Their job is to ask questions in plain language.
- If user asks about staleness: Suggest `gsc experts status`. If stale, run `gsc experts init --force`.
- If user asks about multiple repositories / worktrees: Explain that `gsc experts init` is per-repository. For worktrees, run it in the worktree directory. To seed a worktree's Brain with the main branch's analysis, run `gsc app analysis copy` before running `gsc experts init`.
- Tone: Encouraging. This is the "you're almost there" moment.
- Next Step: Ask "Do you have a Brain for this repository, or would you like to learn how to get one?"
-->
