<!--
Component: GSC Docs Brains
Block-UUID: bbbc4269-4126-4b58-9314-1a908a967b5e
Parent-UUID: N/A
Version: 1.0.0
Description: Explains Brains and Manifests - what they are, how they differ, where Brains are stored, how to create a Brain from a manifest, how to publish and browse manifests, and the enterprise centralization pattern.
Language: Markdown
Created-at: 2026-05-31T15:22:41.429Z
Authors: Gemini 2.5 Flash Lite (v1.0.0)
-->


# Brains and Manifests: The Intelligence Layer

This document explains the core concepts behind GitSense's intelligence system
and how to create a Brain - whether you are starting from scratch, importing
one from a teammate, or consuming one published by your organization.

---

## What Is a Manifest?

A **Manifest** is a portable JSON file containing AI-extracted metadata about a
codebase. It is the output of an **Analyzer** run in the GitSense Chat web app.

- It describes each file: its purpose, layer, risk level, ownership - whatever
  fields the Analyzer was configured to extract.
- It can be committed to the repository, shared via URL, or published to the
  GitSense Chat manifest browser.
- It is the **blueprint**. On its own, it is just a JSON file.

## What Is a Brain?

A **Brain** is a local SQLite database **constructed** from a Manifest. It is
the queryable knowledge base that powers enriched searches and agent context.

- Once constructed, the Brain enables `gsc query`, `gsc rg`, and `gsc tree`
  to search and filter by metadata instead of text patterns.
- It is stored in the `.gitsense/` directory of your repository.
- It is **not portable** - the Manifest is what you share. The Brain is what
  you build from it locally.

## Manifest vs. Brain

| | Manifest | Brain |
| :--- | :--- | :--- |
| **Format** | JSON file | SQLite database |
| **Portable?** | Yes - commit, share, or publish | No - local to your machine |
| **Purpose** | Blueprint / distribution artifact | Queryable knowledge base |
| **Created by** | GitSense Chat web app (Analyzers) | `gsc manifest import` |
| **Shared via** | URL, Git commit, manifest browser | N/A |

---

## Where Are Brains Stored?

Brains live in the `.gitsense/` directory at the root of your repository:

```
<your-repo>/
└── .gitsense/
    ├── <brain-name>.db      ← the Brain (SQLite database)
    ├── backups/             ← auto-backups before overwrites
    └── import-git.json      ← repository import state
```

The Brain's filename is determined by: the `--name` flag → the `database_name`
field inside the manifest JSON → the manifest filename (in that order of
priority).

---

## Creating a Brain: `gsc manifest import`

This is the one command that constructs a Brain from a Manifest.

```bash
# From a local file (e.g., committed to the repository)
gsc manifest import ./metadata/code-intent.json

# From a URL (published by your team or organization)
gsc manifest import https://chat.gitsense.com/--/manifests/<owner>/<repo>

# With a custom name
gsc manifest import <uri> --name my-brain
```

The `.gitsense/` workspace is created automatically on the first import. No
separate init step is required.

### Safety Features

| Feature | Behavior |
| :--- | :--- |
| **Auto-backup** | If a Brain with the same name exists, it is backed up to `.gitsense/backups/` before overwrite |
| **Atomic import** | Built in a temp file, swapped in on success - no partial writes |
| **Force flag** | `--force` overwrites without prompting |

---

## Listing Your Brains

```bash
gsc brains
```

Shows all active Brains: name, file count, and description. Run this at the
start of a session to confirm your intelligence is loaded.

---

## Publishing a Manifest

If you've created an Analyzer in the GitSense Chat web app and want to share
the resulting Manifest with your team:

```bash
gsc manifest publish <path-to-manifest.json>
```

This uploads the Manifest to your GitSense Chat server, where it appears in
the manifest browser. Teammates can then import it directly with a URL.

---

## Browsing Available Manifests

The GitSense Chat web app includes a manifest browser where you can find
Manifests published by your team or the community:

1. Open the Chat App (`http://localhost:3357`)
2. Click the "Intelligence" button
3. Find the repository and intelligence you're interested in
4. Copy the manifest URL
5. Run `gsc manifest import <url>` in your terminal

---

## The Enterprise Pattern: Centralized Intelligence

In many organizations, a platform team (or a single expert) runs Analyzers in
the Chat App, packages the results, and publishes them. Developers on the team
only need the CLI:

```bash
# Find the manifest for your repository in the Chat App browser, then:
gsc manifest import <published-url>

# Make your agent Brain-Aware:
gsc experts init
```

**In this workflow, you may never need to open the Chat App at all.** The web
app is where intelligence is created and published. The CLI is where it is
consumed.

---

## The "README for AI" Pattern

The most durable team workflow is to commit your Manifest directly to the
repository:

```bash
mkdir -p metadata
cp ~/Downloads/code-intent.json metadata/
git add metadata/code-intent.json
git commit -m "Add code-intent Brain manifest"
```

Anyone who clones the repository can then construct the same Brain with a
single command:

```bash
gsc manifest import ./metadata/code-intent.json
```

This makes the repository self-aware out of the box - no web app, no URL,
no coordination required.

---

## Next Steps

Once you have a Brain, make your coding agent Brain-Aware:

```bash
! gsc docs experts
```

<!-- LLM Guidance:
- Role: You are helping a user understand and acquire a Brain. The most common entry points are: (1) given a URL by a colleague, (2) found a manifest file in a cloned repo, (3) need to create one from scratch.
- Entry Point 1 (given a URL): Run `gsc manifest import <url>`. Done. Then direct to `! gsc docs experts`.
- Entry Point 2 (manifest file in repo): Run `gsc manifest import ./metadata/<file>.json`. Done. Then direct to `! gsc docs experts`.
- Entry Point 3 (no manifest, no Brain yet): Explain they need to use the GitSense Chat web app to create an Analyzer. This requires the app to be running. Direct to `! gsc docs install` if not installed, or `! gsc docs lifecycle` to start the app.
- Key distinction to always reinforce: Manifest = portable JSON (the blueprint). Brain = local SQLite database (the queryable knowledge base). They are not the same thing.
- For "where is my Brain stored?": Point to `.gitsense/<brain-name>.db` in the repository root.
- For "how do I know what Brains I have?": Run `gsc manifest list`.
- For "how do I update my Brain?": They need a newer Manifest from the Chat App or a URL pointing to a newer published version, then re-run `gsc manifest import`.
- For "I have multiple Brains": Normal and powerful. Each Brain is a different Analyzer (e.g., code-intent, security-audit). They can be used independently or together in queries.
- For "do I need the web app?": If their organization publishes manifests, no - they only need the CLI to import and consume. The web app is required only to create Analyzers and publish manifests.
- Enterprise framing: Emphasize the centralization pattern. Analyze once, distribute to the whole team via published manifests. This is the highest-leverage use of the system.
- After a Brain is created, always suggest: "Run `! gsc docs experts` to connect your coding agent to this Brain."
- Tone: Clear and practical. Use concrete commands. Avoid jargon.
- Next Step: Ask "Do you have a manifest file or URL to import, or do you need to create one using the Chat App?"
-->
