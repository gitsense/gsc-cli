# Project Structure Map

This file provides a hierarchical JSON representation of the project structure. It is intended to give AI agents immediate context regarding file locations, module organization, and codebase scope without requiring file system traversal.

```json
{
  "context": {
    "about": "This JSON represents a hierarchical Git tree. Each node represents a file or directory. Metadata is included for files where available to provide additional context for analysis.",
    "cwd": ".",
    "fields": [
      "purpose"
    ],
    "pruned": false
  },
  "stats": {
    "files_with_metadata": 87,
    "matched_files": 87,
    "metadata_coverage_percent": 100,
    "total_files": 87
  },
  "tree": {
    "name": ".",
    "is_dir": true,
    "matched": false,
    "children": [
      {
        "name": ".github",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "workflows",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "release.yml",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Automates the cross-platform build and release of the gsc CLI tool using GitHub Actions, triggered by version tags."
                }
              }
            ]
          }
        ]
      },
      {
        "name": ".gitsense",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "analyzers",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "gsc-architect-governance.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "This file defines the analyzer instructions for the gsc-architect-governance component, which enforces architectural governance and layer compliance within the gsc-cli project."
                }
              },
              {
                "name": "gsc-code-reader.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "This file defines the analyzer instructions for the gsc-code-reader component, specializing in raw technical extraction and structural mapping of code files."
                }
              },
              {
                "name": "provenance-code-doc-quality.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the analyzer logic for assessing code documentation quality, semantic honesty, and AI-readiness to determine maintenance burden."
                }
              },
              {
                "name": "provenance-metadata-integrity.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the analyzer logic for validating the honesty and accuracy of AI-generated code metadata and traceability headers."
                }
              }
            ]
          },
          {
            "name": "manifests",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "gsc-provenance.json",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the schema and data for the \"Provenance \u0026 Governance Brain,\" which validates AI-generated code metadata integrity and assesses code documentation quality to ensure governance and traceability."
                }
              }
            ]
          },
          {
            "name": "references",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "PROVENANCE_STANDARDS.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines non-negotiable standards for AI-generated code provenance, metadata integrity, and documentation quality to ensure architectural compliance and maintainability."
                }
              }
            ]
          },
          {
            "name": "README.md",
            "is_dir": false,
            "matched": true,
            "metadata": {
              "purpose": "This directory serves as the intelligence layer for the repository, transforming the codebase into a self-aware, queryable knowledge base for AI agents."
            }
          }
        ]
      },
      {
        "name": "cmd",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "gsc",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "main.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "The main entry point for the gsc CLI application, responsible for initializing the command-line interface and executing the root command defined in the internal CLI layer."
                }
              }
            ]
          }
        ]
      },
      {
        "name": "internal",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "bridge",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "bridge.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Orchestrates the CLI Bridge lifecycle, managing handshake validation, status updates, user confirmation, and the insertion of command output into the GitSense Chat database."
                }
              }
            ]
          },
          {
            "name": "cli",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "manifest",
                "is_dir": true,
                "matched": false,
                "children": [
                  {
                    "name": "bundle.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command definition for generating context bundles from a manifest database using SQL queries."
                    }
                  },
                  {
                    "name": "delete.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines the CLI command for deleting a manifest database, handling user input and invoking the deletion logic to remove the database file and registry entry."
                    }
                  },
                  {
                    "name": "doctor.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Provides a CLI command to run health checks on the `.gitsense` environment and databases, diagnosing structural issues and connectivity with optional auto-fix capabilities."
                    }
                  },
                  {
                    "name": "export.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the CLI command to export manifest databases to human-readable formats (Markdown or JSON), resolving database names via the registry and handling file output."
                    }
                  },
                  {
                    "name": "flags.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines shared command-line flags for manifest subcommands, such as database name and output format, to ensure consistency across the CLI interface."
                    }
                  },
                  {
                    "name": "import.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command to import manifest JSON files into SQLite databases, supporting atomic imports with backup and overwrite options."
                    }
                  },
                  {
                    "name": "init.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command to initialize the GitSense directory structure and create the manifest.json registry file in the project root."
                    }
                  },
                  {
                    "name": "list.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command to list available manifest databases from the registry, displaying metadata such as name, description, and tags in various formats."
                    }
                  },
                  {
                    "name": "root.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines the root command for the manifest subcommand group, serving as the entry point for manifest management tasks."
                    }
                  },
                  {
                    "name": "schema.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the manifest schema command to inspect and display the metadata structure of a manifest database."
                    }
                  }
                ]
              },
              {
                "name": "config.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the gsc config command group for managing context profiles and workspace settings, including focus scope validation."
                }
              },
              {
                "name": "examples.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Serves as a central registry for curated usage examples, demonstrating real-world patterns for the `gsc` CLI to facilitate command discovery."
                }
              },
              {
                "name": "grep.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Executes high-performance code searches using ripgrep and enriches the results with manifest metadata, allowing users to filter by semantic fields like risk or topic while viewing raw code matches."
                }
              },
              {
                "name": "info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Displays the current workspace context, including active databases and configuration status, providing a quick overview of the environment without executing complex queries."
                }
              },
              {
                "name": "query.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Orchestrates metadata discovery and analysis commands, enabling users to query files by value, list available intelligence, and generate insights or coverage reports."
                }
              },
              {
                "name": "root.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Root command definition for the gsc CLI, registering all subcommands, managing global flags, and enforcing pre-flight workspace checks."
                }
              },
              {
                "name": "tree.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "CLI command definition for 'gsc tree', handling user input, coordinating tree construction and enrichment, and managing output formatting and CLI Bridge integration."
                }
              },
              {
                "name": "values.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the `values` CLI command, a convenience shortcut for listing unique metadata values within a specific database field."
                }
              }
            ]
          },
          {
            "name": "db",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "chats.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Library methods for interacting with the GitSense Chat database, including message insertion and validation."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Data structures mapping to the GitSense Chat SQLite schema (chats and messages tables)."
                }
              },
              {
                "name": "schema.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines and initializes the SQL schema for manifest and stats databases, including tables for files, metadata, and search history, with backwards compatibility for older versions."
                }
              },
              {
                "name": "sqlite.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles SQLite database connections using the modernc.org/sqlite driver for CGO-free execution, configuring pragmas and connection pooling for optimal CLI performance."
                }
              }
            ]
          },
          {
            "name": "git",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "discovery.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Locates the Git repository root and retrieves tracked files to support scope validation and repository context."
                }
              },
              {
                "name": "repo_info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Extracts repository metadata (name, URL, remote) from the local Git configuration and provides system environment details like OS and project root."
                }
              }
            ]
          },
          {
            "name": "manifest",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "bundler.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Generates context bundles by executing SQL queries against a manifest database, formatting results into lists or JSON for use in chat sessions or other tools."
                }
              },
              {
                "name": "config.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages the loading, saving, and merging of query configurations and profiles, handling scope resolution precedence and integrating project-level settings from `.gitsense-map`."
                }
              },
              {
                "name": "deleter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements the core logic for deleting a manifest database, coordinating file system removal with registry updates to ensure consistency."
                }
              },
              {
                "name": "doctor_logic.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Performs health checks on the `.gitsense` environment, validating the project root, directory structure, registry file, database connectivity, and identifying orphaned database files."
                }
              },
              {
                "name": "exporter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Exports the contents of a manifest database to Markdown or JSON formats, validating database existence and retrieving all associated metadata, files, and references."
                }
              },
              {
                "name": "gitsense_map.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles loading and validation of the project-level `.gitsense-map` file, which defines version-controlled team defaults for Focus Scope configuration."
                }
              },
              {
                "name": "importer.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles the parsing and importing of JSON manifests into SQLite databases using an atomic swap strategy, including backup rotation, file locking, and language detection."
                }
              },
              {
                "name": "importer_test.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Unit tests for the manifest importer, validating atomic import workflows, backup rotation logic, registry upserts, and file compression utilities to ensure data integrity during manifest operations."
                }
              },
              {
                "name": "info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Logic to gather and format workspace information, including available databases and project root details, while internally managing profile data that is hidden from the user interface."
                }
              },
              {
                "name": "initializer.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Initializes the `.gitsense` workspace directory structure and the manifest registry file, ensuring the environment is ready for metadata operations."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the Go structs that map to the JSON manifest schema, representing the core data model for repositories, branches, analyzers, and file metadata."
                }
              },
              {
                "name": "path_helper.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Helper functions to resolve file paths for databases, backups, and manifests within the `.gitsense` directory, supporting atomic imports and concurrency control."
                }
              },
              {
                "name": "profile_manager.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages the lifecycle of Context Profiles, including creation, deletion, activation, and alias resolution."
                }
              },
              {
                "name": "profile_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines data structures for Context Profiles, which encapsulate configuration settings for workspace switching."
                }
              },
              {
                "name": "querier.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Retrieves and summarizes registered manifest databases from the registry, including file counts and metadata tags."
                }
              },
              {
                "name": "query_formatter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Formats query results, discovery lists, coverage reports, and insights into JSON or human-readable tables, supporting hierarchical views for the intelligence map."
                }
              },
              {
                "name": "query_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core data structures for query operations, coverage analysis, and insights reporting, including hierarchical list items for the discovery dashboard."
                }
              },
              {
                "name": "rg_enricher.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Enriches raw ripgrep search results with file metadata from the SQLite database, supporting both single-match and batch lookup workflows for the dual-pass search system."
                }
              },
              {
                "name": "rg_executor.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Executes the ripgrep binary as a subprocess, handling both JSON discovery passes and raw display passes to support the dual-pass search workflow."
                }
              },
              {
                "name": "rg_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the data structures for ripgrep operations, mapping raw JSON output to Go structs and defining enriched results for the search pipeline."
                }
              },
              {
                "name": "schemareader.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Retrieves and structures schema definitions (analyzers and fields) from the manifest database to support introspection and query validation."
                }
              },
              {
                "name": "scope.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Core logic for Focus Scope handling, implementing glob-based file filtering, pattern validation with Levenshtein suggestions, and the precedence chain for resolving active scopes from CLI, profiles, or project maps."
                }
              },
              {
                "name": "simple_querier.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Executes metadata queries and hierarchical discovery operations, including coverage analysis and insights aggregation, while respecting focus scope constraints."
                }
              },
              {
                "name": "validator.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Validates the structural integrity and data types of manifest files, ensuring all required fields and references are present and correctly formatted before import."
                }
              },
              {
                "name": "wizard.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Interactive CLI wizard for creating and updating context profiles, guiding users through database selection, field configuration, and focus scope definition."
                }
              }
            ]
          },
          {
            "name": "output",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "formatter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides utility functions to format data into JSON, Table, or CSV strings and constructs Markdown messages for the CLI Bridge."
                }
              }
            ]
          },
          {
            "name": "registry",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core data structures and schema for the GitSense registry, including methods for managing database entries."
                }
              },
              {
                "name": "registry.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles the persistence layer for the manifest registry, including loading, saving, and upserting database entries."
                }
              },
              {
                "name": "resolver.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Resolves user-provided database identifiers to canonical physical names."
                }
              }
            ]
          },
          {
            "name": "search",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "aggregator.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Aggregates enriched search results into a structured summary, grouping matches by file, calculating statistics, and applying truncation limits for display."
                }
              },
              {
                "name": "engine.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core abstraction for search engines, allowing pluggable implementations like ripgrep, and structures raw search results for further enrichment."
                }
              },
              {
                "name": "enricher.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Orchestrates the enrichment of raw search matches by fetching metadata from SQLite, applying system and metadata filters, and projecting requested fields for the final result set."
                }
              },
              {
                "name": "filter_parser.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Parses user-provided filter strings into structured SQL WHERE clauses, supporting operators, ranges, and type-aware validation for metadata queries."
                }
              },
              {
                "name": "formatter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles the formatting and presentation of search results, supporting both human-readable terminal output with syntax highlighting and structured JSON for the CLI Bridge."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core data structures and JSON schemas for the search subsystem, including request contexts, result summaries, file matches, and filter conditions."
                }
              },
              {
                "name": "ripgrep_engine.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements the SearchEngine interface using the ripgrep binary to perform high-performance code search and parse JSON output into structured results."
                }
              },
              {
                "name": "stats.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Records search execution details, such as patterns, duration, and match counts, to a local SQLite database to support analytics and the future Scout intelligence feature."
                }
              }
            ]
          },
          {
            "name": "tree",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "tree.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Core logic for building, enriching, pruning, and rendering hierarchical filesystem trees with metadata coverage statistics."
                }
              }
            ]
          },
          {
            "name": "version",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "version.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Stores and retrieves version information, including the application version, Git commit hash, and build timestamp."
                }
              }
            ]
          }
        ]
      },
      {
        "name": "pkg",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "logger",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "logger.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides standardized logging utilities with severity levels, color-coded terminal output, and structured key-value formatting for the GSC CLI."
                }
              }
            ]
          },
          {
            "name": "settings",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "settings.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines global configuration constants and default values for directory structures, file extensions, backup policies, and bridge parameters."
                }
              }
            ]
          }
        ]
      },
      {
        "name": ".gitignore",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "This file specifies the exclusion patterns for the Git repository, preventing build artifacts, IDE configurations, and sensitive environment data from being committed to version control."
        }
      },
      {
        "name": "ARCHITECTURE.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Definitive architectural specification for gsc-cli, defining layers, data flows, interaction guardrails, and critical abstractions to guide AI analysis and development."
        }
      },
      {
        "name": "DEBT.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "This document serves as a technical debt register for the gsc-cli project, tracking known deviations from the architecture specification."
        }
      },
      {
        "name": "LICENSE",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "The full text of the Apache License 2.0, governing the terms, conditions, and permissions for using, reproducing, and distributing the software."
        }
      },
      {
        "name": "Makefile",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Automates the build, installation, testing, and cross-compilation processes for the gsc-cli binary, streamlining development and deployment workflows."
        }
      },
      {
        "name": "NOTICE",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Legal notice asserting copyright for GitSense Chat CLI and mandating the preservation of AI traceability metadata in derivative works."
        }
      },
      {
        "name": "PROJECT_MAP.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Provides an AI-portable hierarchical map of the repository enriched with architectural metadata to optimize token usage and assist LLM navigation."
        }
      },
      {
        "name": "README.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "User-facing documentation for the gsc-cli tool, explaining its purpose as an intelligence layer for codebases, installation, core features, and the philosophy of metadata-driven discovery."
        }
      },
      {
        "name": "go.mod",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Defines the Go module path, required Go version, and lists the direct dependencies necessary for the gsc-cli tool's functionality."
        }
      },
      {
        "name": "go.sum",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Locks the cryptographic hashes of all Go module dependencies to ensure reproducible and secure builds for the project."
        }
      }
    ]
  }
}
```
