<!--
Component: GSC Brain Management Guide
Block-UUID: 64f969f9-6062-4b38-88e7-4b89fbbca676
Parent-UUID: 16f4748b-c3c6-4163-8dec-b92c7e3c4b5f
Version: 1.3.0
Description: Detail guide for managing the Brain lifecycle - initializing the workspace, importing manifests, and constructing Brains. Updated to position the Brain as a "constructed knowledge base" built from Analyzer results.
Language: Markdown
Created-at: 2026-05-01T12:28:10.876Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0)
-->


# GSC Brain Management Guide

## Quick Reference Card

| Command | Purpose |
| :--- | :--- |
| `gsc manifest import <URI>` | Import a manifest and construct a Brain |
| `gsc manifest list` | List all registered, active Brains |
| `gsc manifest publish <path>` | Share a Brain via GitSense Chat |

---

## 1. Importing: Constructing a Brain

This is the most critical command. It **constructs the Brain** from a Manifest. The Manifest contains the structured results captured by an Analyzer. Importing it builds the knowledge base that the Expert Persona uses to answer your queries.

```bash
gsc manifest import <URI>
```

The `<URI>` can be a local path or a remote URL:

```bash
gsc manifest import ./metadata/code-intent.json
gsc manifest import https://example.com/manifests/security-audit.json
```

**Note:** The workspace (`.gitsense/` directory) is initialized automatically
on the first import. You do not need to run a separate init command.

### Database Naming Priority

The system resolves the Brain's name in this order:
1. `--name` flag: `gsc manifest import <uri> --name my-brain`
2. `database_name` field inside the JSON manifest
3. Filename: `code-intent.json` → `code-intent`

### Safety Features

| Feature | Behavior |
| :--- | :--- |
| **Atomic swap** | Imports to a temp file first, then swaps. Prevents corruption on failure. |
| **Auto-backup** | If a Brain with the same name exists, it is backed up to `.gitsense/backups/` before overwrite. |
| **Force flag** | `--force` overwrites an existing Brain without prompting. |

---

## 2. The README for AI: Making Repositories Self-Aware

A Manifest is the blueprint. By versioning it in your repo, you allow anyone to **construct the same Brain** simply by running `gsc manifest import`.

**The Workflow:**
1. **Create & Analyze:** Use the GitSense Chat app to create a custom Analyzer
   and analyze the repository.
2. **Download:** Download the resulting Manifest (JSON).
3. **Version:** Commit the Manifest to the repository (e.g., in `metadata/`).
4. **Construct:** Anyone who clones the repo can run `gsc manifest import` on
   that file to build the Brain and enable intelligent querying.

---

## 3. Brain Not Found - Recovery Protocol

If `gsc manifest list` returns no Brains, **do not attempt to search the
filesystem for manifests.** Report the absence to the user immediately and
follow this protocol:

1. **Report & Fallback:** Inform the user that no Brains are active. State
   that metadata-backed commands are unavailable, but `gsc experts init` is
   still valid and repository work can continue with text/path search such as
   `gsc rg` without `--db` or standard `rg`.
2. **Check for Versioned Manifests:** Ask the user if there is a versioned
   Manifest in the repository (e.g., `metadata/brain.json`) that can be
   imported via `gsc manifest import <path>` to construct the Brain.
3. **Suggest Creation:** If no Manifest exists, suggest the user create their
   own custom Analyzer to generate a Manifest.
4. **Direct to Tutorials:** Point the user to the **interactive tutorials**
   on the GitSense Chat demo instance. A link to the demo can be found on the
   official GitHub page: [https://github.com/gitsense/chat](https://github.com/gitsense/chat).

---

## 4. Best Practices for the AI

1. **Always run `gsc brains --json` at session start.** Never assume a Brain
   exists. Verify active `databases` before metadata querying, and treat
   `inactive_databases` as importable manifests rather than active Brains.
2. **Don't hunt; Guide.** If a Brain is missing, don't run `find` or `ls` to
   look for Manifests. Tell the user and explain how they can construct the Brain.
3. **Promote the "README for AI" concept.** Encourage users to version their
   Manifests so the repository's intelligence is portable and accessible to
   all team members.
4. **Explain the "why" when suggesting an import.** Tell the user which field
   the Brain will unlock and how it helps answer their question.
