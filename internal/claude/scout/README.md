# Scout Package Documentation

## Overview

Scout is a fire-and-forget file discovery tool that helps developers find relevant files across repositories based on a natural language intent. It runs as a background subprocess independent of the chat interface, enabling asynchronous discovery sessions that can be monitored and queried in real-time.

## Architecture

Scout consists of three main layers:

### 1. Core Models (`models.go`)
Defines all data structures for Scout sessions, including:
- **Session**: Represents the overall scout session with working directories and reference files
- **Candidate**: Discovered file with relevance score and metadata from Tiny Overview brain
- **StatusData**: Current status of a running session
- **StreamEvent**: JSONL events emitted during discovery/verification phases

### 2. Session Management (`config.go`, `manager.go`)
Handles session lifecycle and configuration:
- **SessionConfig**: Path resolution and directory management for sessions
- **Manager**: Orchestrates discovery and verification phases, manages subprocess execution
- Session state persistence to disk (status.json)

### 3. Support Services (`validator.go`, `processor.go`)
Validation and event processing:
- **Validator**: Checks prerequisites (contract.json, tiny-overview.json, working directories)
- **EventWriter/EventReader**: JSONL stream processing for real-time progress updates
- **ProcessorHelper**: Utilities for reading session state from event streams

### 4. CLI Interface (`flags.go`, `start.go`, `status.go`, `stop.go`, `root.go`)
User-facing commands:
- **start**: Initiate new discovery session
- **status**: Monitor session progress with real-time streaming
- **stop**: Terminate running session
- **flags**: Shared CLI flag definitions and validation

## Discovery Flow

### Turn 1: Discovery Phase
1. User provides intent and selects working directories
2. `gsc claude scout start` creates a session and spawns a background subprocess
3. Claude reads Turn 1 discovery prompt with intent, brain metadata, and reference files
4. Claude performs keyword-based discovery using Tiny Overview insights
5. Candidates are discovered and scored (0.0-1.0 relevance)
6. Results written as JSONL events to turn-1 log file

### Turn 2: Verification Phase (Optional)
1. User can review Turn 1 results via `gsc claude scout status`
2. User optionally selects subset of candidates for deeper verification
3. Claude reads Turn 2 verification prompt with selected candidates
4. Claude reads actual file snippets to validate relevance
5. Candidates are re-scored based on deeper inspection
6. Final verified candidate list returned to user

## Session Directory Structure

```
${GSC_HOME}/data/claude-code/scout/
├── {sessionId}/
│   ├── status.json                    # Session state and metadata
│   ├── intent.json                    # User's discovery intent
│   ├── references/                    # User-provided reference files
│   │   ├── reference-0.json
│   │   └── reference-1.json
│   ├── turn-1/                        # Turn 1 (Discovery) artifacts
│   │   └── raw-stream-{timestamp}.ndjson  # JSONL event stream
│   └── turn-2/                        # Turn 2 (Verification) artifacts
│       ├── selected-candidates.json   # User-selected candidates to verify
│       └── raw-stream-{timestamp}.ndjson  # JSONL event stream
```

## JSONL Event Format

All progress is streamed as JSON Lines (one JSON object per line):

```jsonl
{"timestamp":"2026-03-27T05:00:00Z","type":"init","data":{...}}
{"timestamp":"2026-03-27T05:00:05Z","type":"status","data":{"phase":"discovery","message":"Searching..."}}
{"timestamp":"2026-03-27T05:00:30Z","type":"candidates","data":{"total_found":5,"candidates":[...]}}
{"timestamp":"2026-03-27T05:01:00Z","type":"done","data":{"status":"success"}}
```

Event types:
- **init**: Session initialized with intent and options
- **status**: Progress updates during phases
- **candidates**: Candidates discovered and scored
- **verified**: Candidates re-scored in verification phase
- **done**: Phase or session completed successfully
- **error**: Error occurred during execution

## Key Design Decisions

### Fire-and-Forget Architecture
- Scout runs as independent subprocess, not tied to chat session
- Session ID returned immediately to user
- User can close chat and return later to check results
- Status always readable from event log file

### Tiny Overview Brain Requirement
- Scout requires `.gsc/brain/tiny-overview.json` in each working directory
- Brain contains purpose, keywords, and parent_keywords for each component
- Brain-guided discovery prevents "blind grepping" across entire codebase
- Ensures opinionated, intentional file discovery

### Turn-Based Separation
- Turn 1 (Discovery): Fast keyword matching, broader candidate pool
- Turn 2 (Verification): Slower code inspection, precision refinement
- Separation allows user to review and filter before deeper analysis
- Optional Turn 2 keeps MVP simple for first iteration

### Multi-Working-Directory Support
- Each workdir assigned an ID for candidate tracking
- Candidates include both relative path and computed absolute path
- Supports searching across multiple repos in one session
- Uses environment variables to pass workdir context to Claude subprocess

### Event Streaming for Real-Time Feedback
- JSONL format enables line-by-line streaming without buffering
- Frontend can render candidates as they appear
- Status command with `-f/--follow` flag streams live events
- Single source of truth: log file, not in-memory process state

## Evolution and Refactoring Notes

### What Went Well
- Core separation of concerns: models → config → manager → CLI
- Session config abstraction clean and reusable
- JSONL event model flexible for adding new event types
- Fire-and-forget design unlocks powerful multi-repo searching

### Potential Improvements (Future)
1. **Caching**: Cache Tiny Overview brains to avoid re-reading
2. **Resume**: Allow resuming interrupted sessions
3. **Filtering**: Add pre-discovery filtering by file extension/size
4. **Parallel Turns**: Allow Turn 2 verification while new Turn 1 searches run
5. **Templates**: Generalize prompt templates for different search strategies
6. **Metrics**: Collect discovery time, accuracy, false positive rates
7. **Batch Mode**: Support multiple intents in single session
8. **Integration**: Deep integration with web UI for real-time candidate browsing

### Known Limitations
- Requires manual workdir setup (no auto-discovery)
- Tiny Overview brain required per workdir (no fallback to blind grep)
- Turn 1/2 separation adds latency vs. single-phase discovery
- No scoring aggregation across workdirs (weighted by workdir quality?)
- Event log not compressed (large sessions = large log files)

## Testing Strategy

Test coverage should focus on:
1. **Session Management**: Creation, loading, state transitions
2. **Event Streaming**: Writing/reading JSONL without corruption
3. **Validation**: Contract/brain verification with missing files
4. **CLI**: Flag parsing, command execution, output formatting
5. **Integration**: Full flow from start → monitor → stop

## Dependencies

- `github.com/spf13/cobra`: CLI command framework
- `github.com/google/uuid`: Session ID generation
- Standard library: `encoding/json`, `os`, `path/filepath`, `time`, `bufio`

No external dependencies for core session management (lightweight design).
