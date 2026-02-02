<!--
Component: GSC CLI README
Block-UUID: bc53d7c3-e829-4977-b1c8-2941019de84f
Parent-UUID: N/A
Version: 1.0.0
Description: README documentation for the GSC CLI tool, providing installation, quick start, and development instructions.
Language: Markdown
Created-at: 2026-02-02T05:45:15.703Z
Authors: GLM-4.7 (v1.0.0)
-->


# GSC CLI

The GitSense Chat (GSC) CLI is a command-line tool designed to manage and query codebase intelligence manifests. It enables AI agents and developers to interact with structured metadata extracted from code repositories.

## Features

- **Manifest Management**: Initialize, import, and list metadata manifests.
- **SQLite Integration**: Queryable database generation for efficient agent access.
- **Agent-First Design**: JSON output modes for seamless AI integration.
- **Git-Aware**: Works from any subdirectory within a project.

## Installation

### From Source

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/gsc-cli.git
   cd gsc-cli
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the binary:
   ```bash
   make build
   ```

4. Install to your path (optional):
   ```bash
   make install
   ```

## Quick Start

### Initialize a Project

Initialize the `.gitsense` directory in your project root:

```bash
gsc manifest init
```

### Import a Manifest

Import a JSON manifest file to create a local SQLite database:

```bash
gsc manifest import path/to/manifest.json
```

### List Manifests

List all available manifests in the current project:

```bash
gsc manifest list
```

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Clean

```bash
make clean
```

## Project Structure

```
gsc-cli/
├── cmd/            # Entry points
├── internal/       # Private application code
├── pkg/            # Public libraries
└── test/           # Test fixtures
```

## License

[Your License Here]
```

---

**Status Update:**
*   **Processed:** `go.mod`, `go.sum`, `Makefile`, `.gitignore`, `README.md`
*   **Remaining in Group 1:** `internal/version/version.go`, `pkg/logger/logger.go`, `pkg/settings/settings.go`
