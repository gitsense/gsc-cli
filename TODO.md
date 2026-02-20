<!--
Component: Project TODO
Block-UUID: 53aa73f6-d994-41d2-8e1f-29395b85c7e3
Parent-UUID: N/A
Version: 1.0.0
Description: Tracks technical debt, bugs, and areas requiring deeper investigation during the development and testing of the gsc-cli tool.
Language: Markdown
Created-at: 2026-02-19T20:00:00.000Z
Authors: GLM-4.7 (v1.0.0)
-->


# Project TODO

This file tracks items that require deeper investigation or resolution.

## High Priority

### 1. Debug Logging Visibility Issue
**Status:** Open
**Context:** When running `gsc -cc manifest publish ...`, the `logger.Debug` call added in `publisher.go` did not produce output to the terminal.
**Impact:** Hinders troubleshooting ability.
**Investigation Steps:**
- [ ] Review `pkg/logger/logger.go` to verify how verbosity flags (`-c`, `-cc`) map to log levels.
- [ ] Check if `logger.Debug` is actually enabled when `-cc` is passed.
- [ ] Verify if there is a buffering issue or if stdout/stderr is being captured incorrectly.
- [ ] Ensure the logger is initialized before the `publish` command's `RunE` function executes.

### 2. Database Path Resolution & Schema Mismatch
**Status:** Investigating
**Context:** Error `SQL logic error: no such table: chats` suggests the CLI is querying a manifest database (which has `files`, `analyzers`) instead of the Chat App database (which has `chats`, `messages`).
**Impact:** `publish` and `unpublish` commands fail.
**Investigation Steps:**
- [ ] Run the updated `publish` command to see the resolved database path in the error message.
- [ ] Verify the `GSC_HOME` environment variable is pointing to the GitSense Chat installation directory, not a project directory.
- [ ] If the path is correct, investigate why the `chats` table is missing. Does the Chat App initialize the DB on first run, or is a schema migration missing?
- [ ] Check `internal/db/schema.go` to see if `CreatePublishedManifestsTable` is the only schema logic being run, or if the base Chat schema needs to be ensured first.

## Low Priority

### 3. Error Message Consistency
**Status:** Open
**Context:** General improvement to error handling across the CLI.
**Investigation Steps:**
- [ ] Audit error messages in `internal/manifest/` to ensure they provide actionable context (e.g., file paths, environment variables) without requiring verbose flags.
