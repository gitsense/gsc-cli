<!--
Component: Technical Debt Register
Block-UUID: 278e9fcc-acec-485e-a92f-ec3276c26b7c
Parent-UUID: a116ef10-5473-40a8-90ae-9e7acdf1e796
Version: 1.2.0
Description: Final pre-release audit update. Added item regarding the public API surface of the registry package.
Language: Markdown
Created-at: 2026-02-18T02:22:40.111Z
Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0)
-->


# Technical Debt Register: gsc-cli

This document tracks known deviations from the `ARCHITECTURE.md` specification.

## 1. Layer Interaction Violations

### CLI Direct Registry Access
*   **Status:** Open
*   **Identified:** 2026-02-18
*   **Violation:** CLI commands directly import `internal/registry`.
*   **Remediation Plan:** Create `internal/manifest/resolver.go` as the sole entry point for the CLI.
*   **Target Milestone:** v0.2.0

## 2. Feature Completeness

### Profile System Visibility
*   **Status:** Open
*   **Description:** Profile logic is hidden from UI.
*   **Target Milestone:** v0.3.0

## 3. Logic Layer Gaps

### Missing Orchestration in `internal/manifest`
*   **Status:** Open
*   **Description:** Lack of a centralized "Manifest Service" for the CLI to interact with.
*   **Target Milestone:** v0.2.0

## 4. API Encapsulation

### Over-Exposed Data Access Methods
*   **Status:** Open
*   **Identified:** 2026-02-18
*   **Description:** `internal/registry/resolver.go` exports `ResolveDatabase` publicly. While necessary for the current (indebted) CLI implementation, this should be unexported (made private) once the logic layer wrapper is built.
*   **Remediation Plan:** Rename `ResolveDatabase` to `resolveDatabase` (lowercase) to enforce the `internal-logic` -> `data-access` boundary.
*   **Target Milestone:** v0.2.0
