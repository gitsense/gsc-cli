<!--
Component: Lessons Draft Schema
Block-UUID: 26e960e9-c6a6-48d4-bf8c-be577b1eac0a
Parent-UUID: 76104d3e-46ca-40d8-a38b-bf41034335b6
Version: 1.1.0
Description: Clarified that summary must be 1-2 sentences max, written to stand alone as a file-level annotation for gsc rg overlays.
Language: Markdown
Created-at: 2026-06-12T12:44:13Z
Authors: Codex GPT-5 (v1.0.0), claude-sonnet-4-6 (v1.1.0)
-->


# GitSense Lesson Draft Schema

Write `.gitsense/tmp/lesson-draft.json` with this shape:

```json
{
  "summary": "1-2 sentences max. Written to stand alone as a file-level annotation.",
  "details": "Why this is worth preserving.",
  "applies_to": {
    "files": [],
    "linked_files": [],
    "commands": [],
    "topics": []
  },
  "tags": [],
  "importance": "low|medium|high",
  "review_checks": [],
  "ai": {
    "provider": "openai",
    "model_id": "gpt-5-codex",
    "agent": "codex"
  }
}
```

Validation rules:

- Use exact repo-relative paths in `files` and `linked_files`.
- Do not use absolute paths, globs, ellipses, or shortened paths.
- Use lowercase slug strings for `tags` and `topics`.
- Include at least one anchor: file, linked file, command, topic, or tag.
- Do not include `id`; `gsc` generates `lsn_<uuid-v7>` on commit.
