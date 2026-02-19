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
    "files_with_metadata": 91,
    "matched_files": 93,
    "metadata_coverage_percent": 97.84946236559139,
    "total_files": 93
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
                  "purpose": "Automates the release process for the gsc-cli project by building cross-platform Go binaries and uploading them as GitHub release assets upon version tag pushes."
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
                  "purpose": "Provides analyzer instructions for enforcing architectural governance, specifically focusing on layer assignment, controlled taxonomy classification, and the generation of governance reports for the gsc-cli project."
                }
              },
              {
                "name": "gsc-code-reader.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "This file defines the gsc-code-reader analyzer, which extracts technical details like APIs and dependencies from code files to create a structural intelligence layer."
                }
              },
              {
                "name": "gsc-intent-scout.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "This file configures the gsc-intent-scout analyzer to translate technical code into human-centric descriptions and identify intent triggers for discovery."
                }
              },
              {
                "name": "provenance-code-doc-quality.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "This file defines the provenance-code-doc-quality analyzer, which assesses code documentation quality, semantic honesty, and AI-readiness scores."
                }
              },
              {
                "name": "provenance-metadata-integrity.md",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the analyzer instructions for validating the integrity and accuracy of AI-generated code metadata, specifically focusing on traceability headers and governance risk assessment."
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
                "name": "gsc-cli-architect.json",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Architectural mapping of the gsc-cli repository to build an intelligence layer for humans and AI agents."
                }
              },
              {
                "name": "gsc-cli-provenance.json",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Governance layer for AI-generated code in gsc-cli, ensuring traceability, metadata integrity, and AI-readiness."
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
                  "purpose": "Defines non-negotiable standards for AI-generated code provenance, metadata integrity, and documentation quality within the gsc-cli repository."
                }
              }
            ]
          },
          {
            "name": "README.md",
            "is_dir": false,
            "matched": true,
            "metadata": {
              "purpose": "This document introduces the `.gitsense` directory as an intelligence layer that transforms repositories into self-aware, queryable knowledge bases for AI agents, reducing token usage and hallucination risks."
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
                  "purpose": "Serves as the entry point for the GSC CLI application, initializing the command-line interface and handling the execution flow."
                }
              }
            ]
          }
        ]
      },
      {
        "name": "docs",
        "is_dir": true,
        "matched": false,
        "children": [
          {
            "name": "assets",
            "is_dir": true,
            "matched": false,
            "children": [
              {
                "name": "demo.mp4",
                "is_dir": false,
                "matched": true
              },
              {
                "name": "poster.png",
                "is_dir": false,
                "matched": true
              }
            ]
          },
          {
            "name": "demo.tape",
            "is_dir": false,
            "matched": true,
            "metadata": {
              "purpose": "A VHS (Video Helper Script) file used to generate a terminal demonstration video for the `gsc` CLI, showcasing features like self-aware repository discovery and intent-based search."
            }
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
                  "purpose": "Orchestrates the CLI bridge lifecycle, managing handshakes, user confirmations, and the insertion of command outputs into the chat database."
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
                      "purpose": "CLI command definition for generating context bundles from a manifest database using SQL queries to create focused file lists for AI analysis."
                    }
                  },
                  {
                    "name": "delete.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command for deleting a manifest database by name, removing the physical file and updating the project registry."
                    }
                  },
                  {
                    "name": "doctor.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "CLI command for running health checks on the .gitsense environment and databases, diagnosing structural and connectivity issues."
                    }
                  },
                  {
                    "name": "export.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines the CLI command for exporting manifest databases to Markdown or JSON formats, resolving database names and handling output to file or stdout."
                    }
                  },
                  {
                    "name": "flags.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Provides shared flag definitions and a helper function to standardize CLI options across manifest subcommands."
                    }
                  },
                  {
                    "name": "import.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the CLI command for importing JSON manifest files into SQLite databases, with options for naming, overwriting, and backup management."
                    }
                  },
                  {
                    "name": "init.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines the CLI command for initializing the GitSense directory structure and manifest registry."
                    }
                  },
                  {
                    "name": "list.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the CLI command to list available manifest databases with support for multiple output formats."
                    }
                  },
                  {
                    "name": "publish.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the CLI command to publish a local manifest file to the GitSense Chat application, handling environment validation and repository details."
                    }
                  },
                  {
                    "name": "root.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Serves as the root command group for manifest management, aggregating subcommands for initialization, listing, publishing, and maintenance."
                    }
                  },
                  {
                    "name": "schema.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Defines the CLI command to inspect the schema of a manifest database, resolving the database name and formatting the output."
                    }
                  },
                  {
                    "name": "unpublish.go",
                    "is_dir": false,
                    "matched": true,
                    "metadata": {
                      "purpose": "Implements the CLI command to remove a published manifest from the GitSense Chat application, validating the environment and triggering deletion."
                    }
                  }
                ]
              },
              {
                "name": "config.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the CLI command structure for managing GitSense context profiles and workspace settings, including creation, activation, and validation of configuration scopes."
                }
              },
              {
                "name": "examples.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Acts as a central registry for curated usage examples, demonstrating real-world patterns for discovery, search, visualization, and AI integration within the gsc-cli tool."
                }
              },
              {
                "name": "grep.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the CLI command for searching code using ripgrep, enriched with metadata from a manifest database."
                }
              },
              {
                "name": "info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements the CLI command to display current workspace context and available databases."
                }
              },
              {
                "name": "query.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides the CLI interface for querying metadata, listing databases and fields, and analyzing coverage or insights."
                }
              },
              {
                "name": "root.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the root command for the GitSense Chat CLI, handling global configuration, logging initialization, and the registration of all subcommands."
                }
              },
              {
                "name": "tree.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements the `tree` command to display a hierarchical, metadata-enriched view of tracked files, supporting filtering, pruning, and multiple output formats."
                }
              },
              {
                "name": "values.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides a shortcut command to list unique values for a specific metadata field within a database, delegating logic to the hierarchical list handler."
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
                  "purpose": "Implements database operations for managing chat hierarchies, message persistence, and published manifest indexing."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the Go data structures that map to the SQLite database schema for chats, messages, and published manifests."
                }
              },
              {
                "name": "schema.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Contains the SQL DDL and logic to initialize the SQLite database schema for manifests, files, and analysis metadata, including backwards compatibility handling."
                }
              },
              {
                "name": "sqlite.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages SQLite database connections with optimized settings for the CLI, including connection pooling and pragma configurations."
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
                  "purpose": "Discovers the project root by locating the .git directory and retrieves tracked files using 'git ls-files' for scope validation."
                }
              },
              {
                "name": "repo_info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Extracts repository metadata such as name and URL from .git/config, and provides system information including OS and project root details."
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
                  "purpose": "Generates context bundles from SQL queries against a manifest database, supporting multiple output formats like JSON and context lists."
                }
              },
              {
                "name": "config.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages the loading, saving, and merging of query configurations and profiles to provide effective settings for CLI commands."
                }
              },
              {
                "name": "deleter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Removes the physical database file and updates the registry to delete a manifest entry."
                }
              },
              {
                "name": "doctor_logic.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Performs comprehensive health checks on the `.gitsense` environment to validate directory structure, registry integrity, and database connectivity."
                }
              },
              {
                "name": "exporter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Exports manifest database content to Markdown or JSON formats, including validation and data querying."
                }
              },
              {
                "name": "gitsense_map.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Loads and validates the project-level .gitsense-map configuration file to define team-wide scope defaults."
                }
              },
              {
                "name": "importer.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles the parsing of JSON manifest files and the atomic import of data into a SQLite database, including schema creation, metadata insertion, and registry updates."
                }
              },
              {
                "name": "importer_test.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides unit tests for the manifest importer, covering atomic imports, backup rotation, and registry management."
                }
              },
              {
                "name": "info.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Gathers and formats workspace information for the `gsc info` command, listing available databases and managing internal profile state."
                }
              },
              {
                "name": "initializer.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Initializes the `.gitsense` directory structure and registry file, setting up the necessary workspace environment within a Git repository."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the Go data structures that map to the JSON manifest schema, representing repositories, branches, analyzers, and file metadata."
                }
              },
              {
                "name": "path_helper.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides utility functions to resolve absolute paths for databases, backups, and lock files, and validates the workspace structure."
                }
              },
              {
                "name": "profile_manager.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages the lifecycle of context profiles, including creation, deletion, activation, and alias resolution, though currently marked as an internal feature."
                }
              },
              {
                "name": "profile_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the Go data structures for Context Profiles, which represent named workspaces containing pre-defined configuration values for the CLI."
                }
              },
              {
                "name": "publisher.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles the orchestration of publishing and unpublishing intelligence manifests, managing hierarchical chat structures, file persistence, and UI synchronization via Markdown regeneration."
                }
              },
              {
                "name": "querier.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements logic to query the manifest registry and list available databases, providing summary information such as entry counts and metadata."
                }
              },
              {
                "name": "query_formatter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Centralizes output formatting logic for query results, lists, schemas, coverage reports, and insights, supporting JSON, table, and CSV formats."
                }
              },
              {
                "name": "query_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core data structures and models for query operations, coverage analysis, and insights reporting within the manifest system."
                }
              },
              {
                "name": "rg_enricher.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Enriches raw ripgrep search results with file metadata from the database and formats the output for display or further processing."
                }
              },
              {
                "name": "rg_executor.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Executes the ripgrep search tool as a subprocess, handling both JSON parsing for discovery and raw output for terminal display."
                }
              },
              {
                "name": "rg_models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the Go data structures for ripgrep search operations, including raw match results, enriched metadata outputs, and configuration options for search execution."
                }
              },
              {
                "name": "schemareader.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides database query logic to retrieve analyzer and field definitions, supporting schema introspection and field type resolution for filtering and CLI interactions."
                }
              },
              {
                "name": "scope.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements core logic for parsing, validating, and resolving file scope patterns using glob matching and precedence rules to filter repository operations."
                }
              },
              {
                "name": "simple_querier.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Handles execution of simple queries, hierarchical discovery lists, coverage analysis, and metadata insights against the manifest database."
                }
              },
              {
                "name": "validator.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Validates the structure and content of loaded ManifestFile objects, ensuring required fields are present and data types match the schema."
                }
              },
              {
                "name": "wizard.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements interactive CLI wizards for creating, updating, and selecting context profiles, guiding users through configuration steps."
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
                  "purpose": "Provides utility functions to format data into JSON, Table, or CSV strings, including terminal width detection and Markdown generation for CLI bridges."
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
                  "purpose": "Defines the core data structures and CRUD operations for managing the GitSense manifest registry index."
                }
              },
              {
                "name": "registry.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements file system operations for loading, saving, and persisting the registry JSON file to the `.gitsense` directory."
                }
              },
              {
                "name": "resolver.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Provides logic to resolve user-provided database identifiers to canonical physical names, supporting fuzzy matching and input sanitization."
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
                  "purpose": "Aggregates raw search matches into a structured summary, grouping results by file and calculating statistics like match counts and field distributions."
                }
              },
              {
                "name": "engine.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the core abstraction and data structures for the search engine, decoupling the implementation (ripgrep/git grep) from the application logic."
                }
              },
              {
                "name": "enricher.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Enriches raw search matches with metadata from a SQLite database, applying system filters and user-defined metadata conditions to refine the result set."
                }
              },
              {
                "name": "filter_parser.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Parses string-based filter expressions into structured condition objects and generates the corresponding SQL WHERE clauses and arguments for database queries."
                }
              },
              {
                "name": "formatter.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Formats and outputs search results, supporting both human-readable text with syntax highlighting and structured JSON, while grouping matches by file."
                }
              },
              {
                "name": "models.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Defines the structured data models and JSON schemas for search results, query context, and system metadata used by the search intelligence layer."
                }
              },
              {
                "name": "ripgrep_engine.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Implements the search engine interface by wrapping the ripgrep binary, executing search commands, and parsing JSON output into structured results."
                }
              },
              {
                "name": "stats.go",
                "is_dir": false,
                "matched": true,
                "metadata": {
                  "purpose": "Manages the persistence of search execution statistics to a local SQLite database, including schema management and record insertion for analytics."
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
                  "purpose": "Core logic for constructing, enriching, and rendering hierarchical file trees, supporting semantic filtering and visibility pruning for visualization."
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
                  "purpose": "Manages application versioning by storing build-time metadata and providing a formatted version string for the GSC CLI."
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
                  "purpose": "Provides standardized logging utilities with configurable severity levels, color-coded terminal output, and structured key-value formatting for the GSC CLI."
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
                  "purpose": "Manages environment resolution for GSC_HOME and constructs file paths for application storage, databases, and backups."
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
          "purpose": "This file specifies intentionally untracked files to be ignored by Git, focusing on build artifacts, dependencies, IDE configurations, and sensitive environment data for the gsc-cli project."
        }
      },
      {
        "name": "ARCHITECTURE.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "This document serves as the definitive source of truth for the `gsc-cli` system architecture, defining layer rules, data flow patterns, interaction guardrails, and critical abstractions."
        }
      },
      {
        "name": "DEBT.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "This document serves as a technical debt register for the gsc-cli project, tracking known deviations from the architecture specification and planned remediation steps."
        }
      },
      {
        "name": "LICENSE",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Contains the full text of the Apache License 2.0, outlining terms and conditions for use, reproduction, and distribution of the software."
        }
      },
      {
        "name": "Makefile",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Automates the build, installation, testing, and cross-compilation processes for the gsc-cli Go tool, including version metadata injection."
        }
      },
      {
        "name": "NOTICE",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Legal notice file containing copyright information, attribution details, and requirements for maintaining AI traceability metadata in derivative works under the Apache License 2.0."
        }
      },
      {
        "name": "PROJECT_MAP.md",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "This file provides a hierarchical JSON representation of the project structure, offering immediate context regarding file locations, module organization, and codebase scope for AI agents."
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
          "purpose": "Defines the Go module for the gsc-cli project, specifying the Go version and listing required dependencies for building the application."
        }
      },
      {
        "name": "go.sum",
        "is_dir": false,
        "matched": true,
        "metadata": {
          "purpose": "Provides cryptographic checksums for all project dependencies to ensure reproducible builds and verify the integrity of downloaded modules."
        }
      }
    ]
  }
}
```
