/**
 * Component: Lessons Guide Loader
 * Block-UUID: 8583bb15-0659-46c3-9715-836db66fa4dc
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Loads markdown-backed lesson draft and schema guidance with built-in fallbacks for agent-facing instructions.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"os"
	"path/filepath"
)

const fallbackDraftGuide = `# GitSense Lesson Draft Guide

Knowledge is everything we could remember. Lessons are the parts worth carrying forward.

Create a concise draft at .gitsense/tmp/lesson-draft.json. Capture durable repository knowledge only: hidden coupling, gotchas, design decisions, workflow rules, review checks, or repeated failure modes.
`

const fallbackSchemaGuide = `# GitSense Lesson Draft Schema

Required draft fields: summary, details, applies_to, tags, importance, review_checks, ai.

Use exact repo-relative paths for files and linked_files. Do not use shortened paths, globs, absolute paths, or ellipses.
`

func DraftGuide() string {
	return readTemplate("LESSON_DRAFT_GUIDE.md", fallbackDraftGuide)
}

func SchemaGuide() string {
	return readTemplate("LESSON_SCHEMA.md", fallbackSchemaGuide)
}

func readTemplate(name string, fallback string) string {
	root, err := rootDir()
	if err != nil {
		return fallback
	}
	path := filepath.Join(root, "pkg", "settings", "templates", "lessons", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(data)
}
