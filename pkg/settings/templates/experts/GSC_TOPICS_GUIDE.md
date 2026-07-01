<!--
Component: GSC Topics and Knowledge Guide
Block-UUID: a1b2c3d4-e5f6-7890-abcd-300000000001
Parent-UUID: N/A
Version: 1.2.0
Description: Added gsc knowledge topics command documentation.
Language: Markdown (Go Template)
Created-at: 2026-06-22T10:00:00Z
Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0), MiMo-v2.5-pro (v1.2.0)
-->


# Topics and Knowledge Guide

Topics organize repository knowledge across lessons, notes, and rules. Every knowledge item must reference exactly one primary topic, with optional related topics (max 2).

---

## Topic Registry

Topics are stored in `.gitsense/topics/records.jsonl`. Each topic has:
- **slug**: lowercase, hyphenated identifier (e.g., `data-layer`, `cli-workflow`)
- **description**: what this topic covers

### Topic Commands

| Command | Purpose |
| :--- | :--- |
| `gsc topics list [-o json]` | List all registered topics |
| `gsc topics show <slug> [-o json]` | Show topic details |
| `gsc topics search <query> [-o json]` | Search topics by slug or description |
| `gsc topics add <slug> --description "..."` | Register a new topic |
| `gsc topics update <slug> --description "..."` | Update topic description |
| `gsc topics migrate [--dry-run]` | Migrate legacy records to new format |
| `gsc knowledge topics [--sort count\|name] [--asc] [--empty] [-o json]` | View topic statistics with item counts |

### Topic Naming Convention

Use topics for **broad navigational domains**:
- Subsystems: `tui`, `cli`, `parser`, `storage`
- Domains: `data-layer`, `auth`, `billing`
- Workflows: `build-deploy`, `release-process`

Use tags for **specific concepts**:
- Technologies: `sql`, `jsonl`, `redis`
- Symptoms: `error-message`, `panic`, `nil-pointer`
- Failure types: `timeout`, `race-condition`

---

## Knowledge Discovery

### Unified Search

Search across all knowledge types with relevance ranking:

```bash
gsc knowledge search <query> [--type lessons,notes,rules] [--topic <slug>] [--limit N] [-o json]
```

**Ranking order:**
1. Exact topic match (highest)
2. Exact tag match
3. Summary term match
4. Body term match (lowest)

**Example:**
```bash
gsc knowledge search "manifest import performance"
```

### Browse by Topic

List all items in a specific topic:

```bash
gsc knowledge list --topic <slug> [--type lessons,notes,rules] [--limit N] [--sort <field>] [--asc] [-o json]
```

**Sort fields:**
- `updated` (default): Sort by last modified date, newest first
- `importance`: Sort by importance level (critical > high > medium > low)
- `type`: Sort by entity type (lesson, note, rule), then by updated date

**Examples:**
```bash
# Default: sorted by updated (newest first)
gsc knowledge list --topic data-layer

# Sort by importance (high first)
gsc knowledge list --topic data-layer --sort importance

# Sort by importance, ascending (low first)
gsc knowledge list --topic data-layer --sort importance --asc

# Sort by type
gsc knowledge list --topic data-layer --sort type
```

### View Topic Statistics

See aggregated statistics across all topics:

```bash
gsc knowledge topics [--sort count|name] [--asc] [--empty] [-o json]
```

**Output:**
```
TOPIC                    LESSONS NOTES RULES TOTAL  UPDATED
------------------------ ------- ----- ----- ------ ----------
implementation           5       0     0     5      2026-06-21
pi                       4       0     0     4      2026-06-19
experts                  3       0     0     3      2026-06-17
```

**Flags:**
- `--sort count|name`: Sort by total items (default) or alphabetical
- `--asc`: Sort ascending (default: descending for count, descending for name)
- `--empty`: Include topics with 0 items
- `-o json`: JSON output with full details including `latest_update`

**Examples:**
```bash
# View all topics with items (sorted by count)
gsc knowledge topics

# Sort alphabetically
gsc knowledge topics --sort name

# Include empty topics
gsc knowledge topics --empty

# Find topics with most items
gsc knowledge topics --sort count --asc
```

### Filter by Type

Search or list specific entity types:

```bash
gsc knowledge search "database" --type lessons
gsc knowledge list --topic data-layer --type rules
```

### Control Output

```bash
# Limit results
gsc knowledge search "lessons" --limit 10

# Truncate summary (default: 50 chars, 0 = no truncation)
gsc knowledge search "lessons" --truncate 80

# JSON output
gsc knowledge search "lessons" -o json
```

---

## Discovery Flow

### After a Code Edit

1. **File-first**: Check rules and lessons for the changed file
   ```bash
   gsc rules get --file <path> -o json
   gsc lessons list --file <path> -o json
   ```

2. **Intent search**: Search for related knowledge
   ```bash
   gsc knowledge search "manifest import"
   ```

3. **Topic expansion**: Browse the topic for broader context
   ```bash
   gsc knowledge list --topic data-layer
   ```

### General Question

1. **Search all knowledge**
   ```bash
   gsc knowledge search "performance large files"
   ```

2. **If results are useful, browse the topic**
   ```bash
   gsc knowledge list --topic data-layer
   ```

3. **Filter by type if needed**
   ```bash
   gsc knowledge search "performance" --type lessons
   ```

---

## Migrating Legacy Records

If you have lessons from before topics were required:

```bash
# Preview changes
gsc topics migrate --dry-run

# Apply migration
gsc topics migrate
```

This will:
1. Extract topics from legacy `applies_to.topics`
2. Register extracted topics in the registry
3. Update records to use the new top-level `topic` field

---

## Creating Lessons with Topics

```bash
# Required: --topic
gsc lessons add --topic data-layer --tag storage --summary "..." --details "..."

# Optional: --related-topic (max 2)
gsc lessons add --topic data-layer --related-topic migration --tag storage --summary "..." --details "..."
```

## Creating Notes with Topics

```bash
gsc notes add --topic data-layer --summary "..." --content "..."
```

## Creating Rules with Topics

```bash
gsc rules new --topic data-layer --glob "**/*.sql" --summary "..." --instruction "..."
```
