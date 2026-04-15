<!--
Component: Claude Internal Package Documentation
Block-UUID: 7a8f9c2d-3e4f-4a5b-9c6d-7e8f9a0b1c2d
Parent-UUID: N/A
Version: 2.0.0
Description: Updated to document the change command implementation and future agent package refactoring plan.
Language: Markdown
Created-at: 2026-04-15T04:20:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0)
-->


# Claude Internal Package

Core logic for Claude CLI integration and execution.

## Current Architecture

### Directory Structure

```
internal/claude/
├── manager.go, processor.go, etc.  # Chat execution (legacy)
├── scout/                          # Scout discovery feature
│   ├── models.go                   # Scout session models
│   ├── manager.go                  # Scout session manager
│   ├── subprocess.go               # Claude subprocess spawning
│   ├── stream.go                   # Stream event processing
│   ├── validator.go                # Turn validation
│   └── ...
└── change/                         # Change feature (in-place editing)
    ├── start.go                    # Change start command
    ├── stop.go                     # Change stop command
    └── change.go                   # Change root command
```

### Current Dependencies
- **Change CLI** → **Scout Package**: Change commands depend on scout's `Manager`, `models`, and other core logic
- **Scout CLI** → **Scout Package**: Scout commands use scout's own package

**Note:** The dependency of change on scout is conceptual-change is not really "scout" functionality, but it currently shares code with scout.

## Critical Feature: Change Command

The `gsc claude change` command is **critical and fully functional**. It enables:

1. **In-place code editing** based on verified discovery results
2. **Git diff generation** for each working directory
3. **Change summary** with file modification tracking
4. **Notes and errors** for agent observations

### Change Command Flow
```
1. User runs: gsc claude change start "Change default expire time to 48 hours"
2. Validates verification is complete
3. Spawns Claude subprocess in working directories
4. Claude edits files in place (no workspace)
5. Generates git diffs per workdir
6. Writes result.json with change summary
7. User reviews changes in UI
```

## Future Architecture: Common `agent` Package

### Problem
Currently, change CLI depends on scout package, creating conceptual coupling. Both scout and change share significant code:
- Session management (`models.go`)
- Turn execution (`manager.go`)
- Subprocess spawning (`subprocess.go`)
- Stream processing (`stream.go`)
- Turn validation (`validator.go`)

### Solution: Create `internal/claude/agent/` Package

Move common agent logic to a shared `agent` package that both scout and change can use.

#### Target Architecture
```
internal/claude/
├── agent/                          # Shared agent logic (NEW)
│   ├── models.go                   # Generic session/turn models
│   ├── manager.go                  # Generic agent manager
│   ├── subprocess.go               # Generic subprocess spawning
│   ├── stream.go                   # Generic stream processing
│   ├── validator.go                # Generic turn validation
│   ├── config.go                   # Session configuration
│   ├── processor.go                # Event processing
│   └── permissions.go              # Agent permissions
├── scout/                          # Scout-specific logic
│   ├── manager.go                  # Scout wrapper around agent.Manager
│   ├── codebase_overview.go        # Scout-specific codebase analysis
│   └── ... (scout-specific code)
├── change/                         # Change-specific logic
│   ├── manager.go                  # Change wrapper around agent.Manager
│   ├── git_diff.go                 # Change-specific git diff generation
│   └── ... (change-specific code)
└── ... (chat execution)
```

#### Dependencies After Refactoring
- **Scout CLI** → **Scout Package** → **Agent Package**
- **Change CLI** → **Change Package** → **Agent Package**

**Benefit:** Removes conceptual coupling-change no longer depends on scout.

### Migration Plan

#### Phase 1: Create `agent` Package
1. Create `internal/claude/agent/` directory
2. Move `models.go` → `agent/models.go` (rename types to generic names)
3. Move `validator.go` → `agent/validator.go` (generalize validation)
4. Move `config.go` → `agent/config.go`
5. Move `processor.go` → `agent/processor.go`
6. Move `permissions.go` → `agent/permissions.go`

#### Phase 2: Refactor Manager
1. Move `manager.go` → `agent/manager.go` (rename to `AgentManager`)
2. Extract scout-specific logic to `scout/manager.go` (wrapper)
3. Make template directory configurable
4. Add `validateTurnState()` method for agent-specific validation

#### Phase 3: Refactor Subprocess
1. Move `subprocess.go` → `agent/subprocess.go`
2. Make template path resolution configurable
3. Add `copyMethodologyFiles()` method for agent-specific file copying
4. Add `buildSystemPrompt()` method for agent-specific prompt building
5. Add `writeTaskPrompt()` method for agent-specific task writing

#### Phase 4: Refactor Stream
1. Move `stream.go` → `agent/stream.go`
2. Add `tryParseDiscoveryResult()` method
3. Add `tryParseVerificationResult()` method
4. Add `tryParseChangeResult()` method
5. Add `tryParseReviewResult()` method (for future review command)
6. Use simple if/else chain for result parsing (no registration needed)

#### Phase 5: Update CLI Commands
1. Update scout CLI to use `agent` package
2. Update change CLI to use `agent` package
3. Remove direct dependency on scout package from change CLI
4. Integration tests for both scout and change

### Design Principles

#### Keep It Simple
- **No registration system**: Use simple if/else chains for turn-type specific logic
- **No plugin architecture**: Hardcode turn types in validator
- **No complex configuration**: Pass parameters directly to methods

#### Composition Over Inheritance
- Scout and change packages wrap `agent.Manager`
- Agent-specific logic stays in scout/change packages
- Generic logic stays in agent package

#### Agent-Specific Hooks
The `agent.Manager` will provide hooks for agent-specific behavior:
- `validateTurnState(turnType)` - Agent-specific state validation
- `copyMethodologyFiles(turn, turnType)` - Agent-specific file copying
- `buildSystemPrompt(gscHome, turnType)` - Agent-specific prompt building
- `writeTaskPrompt(turn, turnType)` - Agent-specific task writing
- `tryParse*Result(resultContent, turn)` - Agent-specific result parsing

### Benefits of Refactoring

1. **Reduced Code Duplication**: Common agent logic is shared
2. **Better Separation of Concerns**: Scout and change are truly independent
3. **Easier to Add New Agent Types**: Future agents (e.g., "review", "test") can reuse agent package
4. **Improved Testability**: Agent logic can be tested independently
5. **Clearer Architecture**: Dependencies are more logical and explicit

### Estimated Effort

- **Phase 1**: 0.5 day (move models, validator, config, processor, permissions)
- **Phase 2**: 0.5 day (refactor manager)
- **Phase 3**: 0.5 day (refactor subprocess)
- **Phase 4**: 0.5 day (refactor stream)
- **Phase 5**: 0.5 day (update CLI commands)

**Total**: 2-3 days for careful, well-tested migration.

## Status

- ✅ **Change command**: Fully functional and critical
- ✅ **Scout command**: Fully functional
- 🔄 **Agent package**: Planned refactoring (not started)
- 📋 **Migration plan**: Documented and ready to execute

## Notes

- The change command is **critical** and must remain functional throughout the refactoring
- The refactoring should be done incrementally with comprehensive tests
- The `agent` package should remain simple and focused on generic logic only
- Agent-specific logic should stay in scout/change packages
