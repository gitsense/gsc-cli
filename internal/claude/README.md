# Claude Internal Package

Core logic for Claude CLI integration and execution.

## Directory Structure & Evolution

### Current Layout
- `manager.go`, `processor.go`, etc. - Chat execution (legacy location)
- `scout/` - Scout discovery feature (new, organized by feature)

### Note on Organization
This package evolved organically: chat functionality was implemented first and placed directly in this directory. As new features like Scout were added, they were organized into subdirectories.

For now:
- Chat code: Direct in `internal/claude/`
- Scout code: `internal/claude/scout/`
