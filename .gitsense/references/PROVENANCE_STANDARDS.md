<!--
Component: Provenance Standards Reference
Block-UUID: 43eb4404-e110-430a-9664-f467d0ecf0a2
Parent-UUID: 21d1bcfe-baf2-43cd-9046-30486f9b6da6
Version: 1.2.0
Description: Definitive standards for AI-generated code provenance, metadata integrity, and documentation quality. Includes deterministic checklists for agentic readiness, semantic honesty, and calibration examples.
Language: Markdown
Created-at: 2026-02-14T00:00:57.140Z
Authors: Gemini 3 Flash (v1.0.0), Claude Haiku 4.5 (v1.1.0), Gemini 3 Flash (v1.1.0), Kimi K2 Thinking (v1.2.0)
-->


# Provenance & Governance Standards (v1.2.0)

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

### The "Agentic Simulation" Scenario
To calculate this score, you must perform a mental simulation:

**Scenario:** You are a "Junior AI Agent" tasked with fixing a bug or refactoring this file. You have access to:
1. This file's source code.
2. The `ARCHITECTURE.md` file.
3. **NO** access to other files in the repository (no context switching).

**Question:** Based *only* on the comments and docstrings in this file, can you identify the side effects, locking strategies, and error handling paths without guessing?

### The Checklist (What the Agent Needs to Know)
Use the following checklists to determine if the documentation supports the simulation:

**For Database/Persistence Code:**
- [ ] Are transaction boundaries documented? (BEGIN, COMMIT, ROLLBACK)
- [ ] Are side effects called out? (Does this write to disk? Lock a file?)
- [ ] Is the schema/table relationship explained?

**For Concurrency/State Code:**
- [ ] Are mutex/lock semantics documented?
- [ ] Is the order of operations or state transitions explained?
- [ ] Are race conditions or deadlocks mentioned?

**For CLI/Bridge Code:**
- [ ] Are exit codes and error messages explained?
- [ ] Is the handshake or communication protocol documented?
- [ ] Are the pre-conditions for execution clear?

### Scoring Bands
- **90-100 (AI-Native):** The agent can perform the task safely. All critical "Why" and "Side Effects" are documented.
- **70-89 (AI-Ready):** The agent can perform the task with moderate risk. Intent is clear, but some edge cases require inference.
- **40-59 (Human-Required):** The agent would likely introduce bugs. The "What" is documented, but the "Why" is missing.
- **0-39 (Toxic Context):** The agent will likely break the code. Comments are sparse or misleading.

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

---

## 7. Intent Clarity Standards

This field measures whether the code explains **"Why"** (Intent) or just **"What"** (Functionality).

### High Intent Clarity
- **Design Decisions:** Explains *why* a specific algorithm or pattern was chosen.
- **Trade-offs:** Mentions what was rejected and why.
- **Context:** Connects the code to the broader system architecture.
- *Example:* "We use a channel buffer of 100 here to prevent blocking the main goroutine during high-throughput imports."

### Medium Intent Clarity
- **Functionality:** Clearly explains *what* the code does.
- **Implicit Intent:** The "Why" can be inferred by reading the code, but isn't stated.
- *Example:* "Reads from the channel and processes the import." (Clear what, but why buffer size 100? Unclear.)

### Low Intent Clarity
- **Implementation Only:** Explains *how* the code works (loops, variables) but not the purpose.
- **Missing Context:** No explanation of the problem being solved.
- *Example:* "Loop through items and call process()." (Mechanical, no intent.)

---

## 8. Maintenance Burden Standards

This field measures how much "tribal knowledge" is required to maintain the code.

### Low Maintenance Burden
- **Standard Patterns:** Uses idiomatic Go and standard library features.
- **Clear Dependencies:** Imports and relationships are obvious.
- **Self-Documenting:** Variable names and structure make the code readable.

### Medium Maintenance Burden
- **Moderate Complexity:** Requires reading the code to understand the flow.
- **Some Context:** Needs familiarity with the project structure.
- **Acceptable Risk:** Safe for experienced developers, risky for juniors.

### High Maintenance Burden
- **"Clever" Code:** Uses non-obvious tricks, bit-shifting, or obscure language features.
- **Hidden Coupling:** Relies on global state or implicit side effects not documented.
- **Tribal Knowledge:** Requires a specific person to explain "how this actually works."

---

## 9. Calibration Examples (gsc-cli Repository)

Use these files as anchors for your assessments.

### Example A: `internal/bridge/bridge.go`
- **Documentation Quality:** Comprehensive
- **Intent Clarity:** High (Explains handshake lifecycle and stage-based validation)
- **AI-Readiness Score:** 85 (Side effects like DB insertion are documented)
- **Maintenance Burden:** Low (Standard orchestration patterns)
- **Missing Context:** None

### Example B: `internal/search/aggregator.go`
- **Documentation Quality:** Adequate
- **Intent Clarity:** Medium (Explains grouping, but not *why* that specific grouping algorithm was chosen)
- **AI-Readiness Score:** 55 (Safe to read, but risky to refactor without understanding the performance implications)
- **Maintenance Burden:** Medium
- **Missing Context:** ["algorithm-rationale", "performance-considerations"]

### Example C: `internal/cli/config.go`
- **Documentation Quality:** Minimal
- **Intent Clarity:** Low (Hidden commands and profile logic are not explained)
- **AI-Readiness Score:** 40 (An agent would likely miss the hidden profile activation logic)
- **Maintenance Burden:** High (Hidden side effects)
- **Missing Context:** ["profile-lifecycle", "wizard-integration", "hidden-commands"]
