<!--
Component: Provenance Standards Reference
Block-UUID: 21d1bcfe-baf2-43cd-9046-30486f9b6da6
Parent-UUID: 5d5748c1-5a87-43e4-87dd-54d65c7c4da0
Version: 1.1.0
Description: Definitive standards for AI-generated code provenance, metadata integrity, and documentation quality. Includes deterministic checklists for agentic readiness and semantic honesty.
Language: Markdown
Created-at: 2026-02-14T00:00:57.140Z
Authors: Gemini 3 Flash (v1.0.0), Claude Haiku 4.5 (v1.1.0), Gemini 3 Flash (v1.1.0)
-->


# Provenance & Governance Standards (v1.1.0)

This document defines the non-negotiable standards for AI-generated code within the `gsc-cli` repository. It is used by the Provenance Brain to audit metadata integrity, documentation quality, and architectural compliance.

## 1. Description Field Standards

The `Description` field in the Code Block Header is the "Source of Truth" for a file's intent. It must be technically honest and current.

### ✅ ACCURATE Description Criteria
- **Specific:** Mentions the primary responsibility and key patterns (e.g., "atomic swap strategy", "glob-based filtering").
- **Current:** Reflects the *actual* code logic present in the file.
- **Intent-Focused:** Explains what the file *achieves*, not just what it *is*.

### ❌ STALE/DISHONEST Description Criteria
- **Outdated:** Describes features that have been removed or logic that has been refactored.
- **Vague:** Uses generic phrases like "Handles logic" or "Utility functions."
- **Comment-Code Mismatch:** If the code does something the description doesn't mention (e.g., a hidden database write), it is **Stale**.

---

## 2. Semantic Honesty & Anti-Fluff

AI-generated documentation must be technically accurate. Comments that "sound professional" but are technically hollow are a form of hallucination and must be penalized.

### ❌ Examples of Semantic Fluff (Mark as Stale/Minimal):
- `// Process the data` (What data? What processing?)
- `// Handle the error` (Which error? How is it handled?)
- `// Update the database` (Which table? What transaction semantics?)

### ✅ Examples of Semantic Honesty:
- `// Validate that the file path is within the .gitsense directory to prevent directory traversal attacks.`
- `// Acquire the import lock before writing to the temp database; release it in a defer to ensure cleanup even on panic.`

**The Rule:** If a comment could apply to 10 different implementations of the same function, it is **Semantic Fluff**.

---

## 3. Agentic Readiness Scoring Rubric (0-100)

This score represents how safely an AI agent could refactor this file without human supervision.

### The "Agentic Simulation" Checklist
To calculate the score, verify these specific items based on the code type:

**For Database/Persistence Code:**
- [ ] Are transaction boundaries documented? (BEGIN, COMMIT, ROLLBACK)
- [ ] Are side effects called out? (Does this write to disk? Lock a file?)

**For Concurrency/State Code:**
- [ ] Are mutex/lock semantics documented?
- [ ] Is the order of operations or state transitions explained?

**For CLI/Bridge Code:**
- [ ] Are exit codes and error messages explained?
- [ ] Is the handshake or communication protocol documented?

**Scoring:**
- **90-100 (AI-Native):** ALL relevant items checked. Intent is crystal clear.
- **60-89 (AI-Ready):** 70% of items checked. Safe for AI with human review.
- **40-59 (Human-Required):** 50% of items checked. High risk of hallucination.
- **0-39 (Toxic Context):** <50% of items checked. AI will likely break side effects.

---

## 4. Complexity-Documentation Balance

More complex code requires more documentation. This is non-negotiable.

- **Simple Code:** Basic docstring explaining inputs/outputs is sufficient.
- **Moderate Code:** Docstring + inline comments for non-obvious branches.
- **Complex Code (Algorithms, Concurrency):** Comprehensive docstring + detailed inline comments + edge case documentation.

**The Rule:** If code complexity is **HIGH** but documentation is **MINIMAL**, the file is **High Risk** regardless of other factors.

---

## 5. Governance Risk Assessment

| Risk Level | Trigger Conditions |
| :--- | :--- |
| **None** | Full traceability, Accurate description, Comprehensive documentation, No layer violations. |
| **Low** | Full traceability, Accurate description, Adequate documentation. |
| **Medium** | Stale description **OR** Minimal documentation **OR** Partial traceability. |
| **High** | Missing traceability **OR** Layer violation **OR** Stale description with High complexity. |

---

## 6. Architectural Integrity (Layer Rules)

The Provenance Brain enforces the layer rules defined in `ARCHITECTURE.md`.

### Layer Violations (Automatic Risk Escalation)
- **Data-Access Violation:** A file in `internal/db/` that imports `cobra` or prints directly to `os.Stdout`.
- **CLI Violation:** A file in `internal/cli/` that directly opens a SQLite connection instead of using the `db` package.
- **Utility Violation:** A file in `pkg/` that has non-logging side effects (e.g., writing to the `.gitsense` directory).
