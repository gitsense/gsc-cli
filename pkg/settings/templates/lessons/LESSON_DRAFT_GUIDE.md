<!--
Component: Lessons Draft Guide
Block-UUID: ec0f64d0-be63-4dc1-9c02-851748e61890
Parent-UUID: 24d85c5b-e71f-47f5-aa41-9706d461b909
Version: 1.3.0
Description: Added discard step to workflow documentation.
Language: Markdown
Created-at: 2026-06-12T12:44:13Z
Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0), MiMo-v2.5-pro (v1.3.0)
-->


# GitSense Lesson Draft Guide

Knowledge is everything we could remember. Lessons are the parts worth carrying forward.

Create a concise draft at `.gitsense/tmp/lesson-draft.json`. Capture durable repository knowledge only when it should help future humans or agents avoid rediscovery, mistakes, or missed context.

The `summary` field must be 1-2 sentences maximum. It is stored as a scalar and used directly as a file-level annotation in `gsc rg` overlays — write it to stand alone without surrounding context.

Good lesson candidates:

- Hidden coupling between files, commands, or workflows.
- Gotchas discovered while changing or reviewing code.
- Design decisions that explain why an approach was chosen.
- Review checks that should run when touching an area.
- Repeated failure modes or validation steps worth preserving.

Do not capture raw transcript, generic summaries, or information that is unlikely to change future work.

After writing the draft, the agent must validate it before telling the user it is ready:

```bash
gsc lessons validate
```

Only after validation passes, tell the user to run:

```bash
gsc lessons review
```

If the draft is incorrect or no longer needed, discard it:

```bash
gsc lessons discard
```

After the user confirms the meaning in the review output, run:

```bash
gsc lessons commit
```
