<!--
Component: System Architecture Reference
Block-UUID: 891965fc-b43e-4c8b-bf4d-9b40e8d10fd9
Parent-UUID: 7746e4b2-77ec-40d3-8b1c-dd9c9cca9cbe
Version: 1.4.0
Description: Definitive architectural specification for gsc-cli. Includes structural mapping, data flow patterns, layer interaction guardrails, and critical abstraction definitions.
Language: Markdown
Created-at: 2026-02-12T04:27:26.111Z
Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), Gemini 3 Flash (v1.3.0), GLM-4.7 (v1.4.0)
-->


# GSC-CLI System Architecture Reference (v1.2.0)

This document is the definitive "Source of Truth" for the `gsc-cli` architecture. It provides unambiguous rules for file categorization, logic interpretation, and architectural guardrails for AI analysis.

## 1. Layer Definitions

| Layer | Primary Rule | Path Patterns |
| :--- | :--- | :--- |
| `cli` | Entry points and command definitions. Imports `github.com/spf13/cobra`. | `cmd/gsc/*`, `internal/cli/*` |
| `internal-logic` | Business logic and orchestration. Coordinates between data and system tools. | `internal/manifest/*`, `internal/search/*`, `internal/tree/*` |
| `data-access` | Direct persistence layer. Handles SQLite (modernc) or JSON registry I/O. | `internal/db/*`, `internal/registry/*` |
| `pkg-util` | Stateless helpers, system wrappers (Git/Ripgrep), and shared formatting. | `pkg/*`, `internal/git/*`, `internal/output/*` |
| `config` | Management of global constants, environment variables, and user settings. | `pkg/settings/*`, `internal/manifest/config.go` |

## 2. Data Flow Patterns

### Primary Flows
| Flow | Components | Purpose |
| :--- | :--- | :--- |
| **Search Flow** | CLI (grep) → RipgrepEngine → Enricher → Formatter → Output | Find code with metadata context |
| **Import Flow** | CLI (manifest import) → Importer → Validator → AtomicSwap → Registry | Load intelligence into workspace |
| **Query Flow** | CLI (query) → SimpleQuerier → SQLite → Formatter → Output | Discover files by metadata |
| **Tree Flow** | CLI (tree) → TreeBuilder → FilterParser → Enricher (with Filters) → Renderer → Output | Visualize file hierarchy with metadata and semantic filtering |

### Enrichment Pipeline (Sub-Flow)
`Raw System Output (Files/Grep)` → `Metadata Lookup (SQLite)` → `Field Projection` → `Formatter` → `Output`

## 3. Layer Interaction Rules (Guardrails)

| From Layer | To Layer | Allowed? | Rationale |
| :--- | :--- | :--- | :--- |
| `cli` | `internal-logic` | ✅ Yes | Commands must delegate to logic handlers. |
| `cli` | `data-access` | ❌ No | CLI should never query SQLite or Registry directly. |
| `internal-logic` | `data-access` | ✅ Yes | Logic layer orchestrates persistence. |
| `internal-logic` | `pkg-util` | ✅ Yes | Logic uses system wrappers (Git/Ripgrep). |
| `data-access` | `internal-logic` | ❌ No | Data layer must remain stateless regarding business logic. |

## 4. Controlled Topic Taxonomy

### Features
*   `bridge`: 6-digit handshake for terminal-to-chat output insertion.
*   `ripgrep`: Integration with `rg` binary for high-performance code searching.
*   `sqlite`: CGO-free persistence for manifest metadata and search history.
*   `git-integration`: Discovery of project roots and retrieval of tracked files (`ls-files`).
*   `manifest-management`: Lifecycle of importing, exporting, and validating metadata.
*   `focus-scope`: Glob-based inclusion/exclusion filtering for targeted analysis.
*   `tree-visualization`: Hierarchical rendering of files enriched with metadata, supporting semantic filtering to generate a "Heat Map" of relevant files.
*   `interactive-wizard`: Survey-based CLI prompts for profile and manifest setup.

### Architectural Patterns
*   `atomic-import`: "Temp-Build -> Backup -> Rename" workflow for database safety.
*   `metadata-enrichment`: Joining raw system output (files/grep) with SQLite metadata.
*   `dual-pass-search`: Executing a JSON pass for data and a Raw pass for terminal display.
*   `hierarchical-discovery`: The `query list` drill-down (DB -> Field -> Value).
*   `stage-based-validation`: Lifecycle checks (Discovery, Execution, Insertion) for the CLI Bridge.
*   `batch-metadata-lookup`: Efficient multi-file metadata retrieval using SQL `IN` clauses.
*   `filter-parsing-and-validation`: Operator-based metadata filtering with type-aware SQL generation.
*   `profile-based-configuration`: Hidden feature for context switching via profiles and aliases.
*   `scope-resolution-precedence`: Three-tier fallback: CLI Override → Active Profile → `.gitsense-map`.

## 5. Critical Abstractions

| Abstraction | Location | Purpose |
| :--- | :--- | :--- |
| `SearchEngine` | `internal/search/engine.go` | Interface for pluggable search implementations (ripgrep, git grep). |
| `Registry` | `internal/registry/models.go` | Tracks all imported databases and their metadata. |
| `FilterCondition` | `internal/search/models.go` | Represents a single metadata filter (field, operator, value). |
| `ScopeConfig` | `internal/manifest/scope.go` | Represents a Focus Scope (include/exclude patterns). |

## 6. Metadata Extraction Rules

### API & Dependency Rules
| Field | Extraction Rule |
| :--- | :--- |
| **Public API** | Include capitalized definitions in `pkg/*`. For `internal/*`, include only interfaces and types imported by multiple internal packages (e.g., `SearchEngine`, `Registry`). Exclude single-use helpers. |
| **Dependencies** | **Critical:** `cobra` (CLI), `sqlite` (Data), `enry` (Language), `doublestar` (Scope), `survey/v2` (Wizard), `levenshtein` (Suggestions). |
| **Error Handling** | Identify `BridgeError` for exit-code logic. Look for `SilenceUsage: true` in Cobra commands. |

### Field-Specific Logic Rules
| Field Type | Storage & Query Rule |
| :--- | :--- |
| **Array Fields** | Stored as JSON strings in SQLite. Must be queried using `json_each()`. |
| **Scalar Fields** | Stored as TEXT. Queried with standard `=`, `!=`, or `LIKE` operators. |

### Pattern Matching
The `gsc query` command supports glob-style wildcards (`*`) in the `--value` argument to enable fuzzy discovery.
*   **Syntax:** `*` matches any sequence of characters.
*   **Translation:** The CLI translates `*` to the SQL `%` wildcard before execution.
*   **Example:** `--value "*connection*` translates to `LIKE '%connection%'`.
*   **Scope:** Applies to both scalar fields and array fields (via `json_each`).

### Intent Triggers (High-Signal Queries)
*   "How does the CLI Bridge insert messages into the SQLite chat database?"
*   "What is the precedence chain for resolving a Focus Scope?"
*   "How are array-type metadata fields stored and queried in SQLite?"
*   "Where is the logic for rotating compressed database backups?"
*   "How does the tree command calculate analysis coverage percentages?"
*   "How are JSON array fields (like topics) queried differently from scalar fields?"
*   "How does the tree command apply semantic filters to generate the heat map?"

## 7. Project Glossary

| Term | Definition |
| :--- | :--- |
| **Manifest** | A queryable intelligence layer (JSON or SQLite) containing file-level metadata. |
| **Bridge** | A secure channel using a 6-digit code to pipe CLI output into a GitSense Chat session. |
| **Focus Scope** | A set of glob patterns that restrict the CLI's "territory of interest" within a repository. |
| **Brain** | A specialized analyzer (defined in GitSense Chat) that produces the metadata found in manifests. |
| **Registry** | The `manifest.json` file that tracks all imported databases in a workspace. |
| **CWD-Aware** | Logic that adjusts file paths and tree roots based on the user's current directory relative to the Git root. |

## 8. Internal & Hidden Features

**Note on Profiles:** The profile system (`internal/manifest/profile_manager.go`) is **RETAINED INTERNAL LOGIC**. While the implementation for context switching and aliases exists, the corresponding CLI flags are currently hidden from the user interface to reduce complexity. AI analyzers should recognize this code as "dormant/internal" rather than "active user-facing."
