<!--
Component: GSC Docs Admin
Block-UUID: 1278a258-70e7-454a-8355-6d8d52cf03db
Parent-UUID: c0c82dec-b1b1-4cd8-9d62-a1a88f2ad73a
Version: 1.3.0
Description: Admin guide for configuring GitSense Chat - LLM providers, models, .env setup, and restart requirements. Written as an AI behavioral contract. Added env set verification step, fixed non-interactive model add examples to include --no-divider and -y flags, added API key file verification to model checklist, fixed Google API key variable name.
Language: Markdown
Created-at: 2026-05-31T01:21:26.453Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Claude Sonnet 4.6 (v1.3.0)
-->


# Configuring GitSense Chat

The `gsc app native admin` command provides a convenient interface for managing
GitSense Chat configuration without needing to worry about paths or installation
details.

**Note:** This is GitSense Chat admin v0.1.0. The admin command currently focuses
on LLM configuration. Future versions will include backup/restore, system settings,
and more.

---

## 1. Environment Variables (`.env` File)

GitSense Chat uses a `.env` file to store API keys and configuration. You can
manage this file entirely from the command line.

### List All Variables

```bash
gsc app native admin env list
```

Shows all currently set variables with their values (API keys are masked for security).

### Get a Specific Variable

```bash
gsc app native admin env get OPENAI_API_KEY
```

Returns the value of a specific variable (masked for API keys).

### Set a Variable

```bash
gsc app native admin env set OPENAI_API_KEY=sk-...
```

Sets or updates a variable. The value is automatically masked in the output.

> **Verify after setting:** Always confirm the value was written to the correct `.env` file:
> ```bash
> gsc app native admin env get OPENAI_API_KEY
> ```
> If the value appears masked but is missing from the file, use `env edit` to set it manually.

### Unset a Variable

```bash
gsc app native admin env unset OPENAI_API_KEY
```

Removes a variable from `.env`.

### Validate Configuration

```bash
gsc app native admin env validate
```

Checks for:
- Required variables (based on configured providers)
- Syntax errors
- Invalid variable names
- Empty values for required keys

### View Available Variables

```bash
gsc app native admin env template
```

Displays the `.env.example` file with descriptions of all available variables.

### Edit in Preferred Editor

```bash
gsc app native admin env edit
```

Opens `.env` in your configured editor (`$EDITOR` or falls back to `nano`).

### Common API Key Variables

```bash
# OpenAI
gsc app native admin env set OPENAI_API_KEY=sk-...

# Anthropic
gsc app native admin env set ANTHROPIC_API_KEY=sk-ant-...

# Google
gsc app native admin env set GEMINI_API_KEY=...

# DeepSeek
gsc app native admin env set DEEPSEEK_API_KEY=...

# Cerebras
gsc app native admin env set CEREBRAS_API_KEY=...
```

> **Important:** After setting API keys, you must restart the app for changes to
> take effect (see Section 6).

---

## 2. Check Current Configuration

Always check what is already configured before making changes. The `--format json`
flag returns complete details including model IDs and index positions.

```bash
# List all configured providers
gsc app native admin llm list providers --format json

# List all configured models (includes index, model ID, max tokens, default status)
gsc app native admin llm list models --format json
```

---

## 3. Managing Providers

### Add a Provider (Non-Interactive)

```bash
gsc app native admin llm add provider \
  -n "ProviderName" \
  -k "API_KEY_ENV_VAR" \
  [-u "https://api.example.com"]   # Optional - omit for standard providers
```

Examples:
```bash
# OpenAI - no base URL required
gsc app native admin llm add provider -n "OpenAI" -k "OPENAI_API_KEY"

# Anthropic
gsc app native admin llm add provider -n "Anthropic" -k "ANTHROPIC_API_KEY"

# Google
gsc app native admin llm add provider -n "Google" -k "GOOGLE_GEMINI_API_KEY"

# Custom or local LLM
gsc app native admin llm add provider -n "LocalLLM" -k "LOCAL_API_KEY" \
  -u "http://localhost:11434/v1"
```

### Edit a Provider
```bash
gsc app native admin llm edit provider
```

### Remove a Provider
```bash
gsc app native admin llm remove provider
```

---

## 4. Managing Models

### Add a Model (Non-Interactive)

Use `--no-divider` and `-y` to run without any prompts:

```bash
gsc app native admin llm add model \
  -n "Display Name" \
  -p "Provider Name" \
  -i "model-id" \
  -t <max-output-tokens> \
  --no-divider \
  -y \
  [-d]                        # Optional: set as default model
  [--index <number>]          # Optional: 0-based position
  [--before "ModelName"]      # Optional: insert before this model/divider
  [--after "ModelName"]       # Optional: insert after this model/divider
```

Common examples:
```bash
# Anthropic
gsc app native admin llm add model \
  -n "Claude Sonnet 4.6" -p "Anthropic" -i "claude-sonnet-4-6" -t 8192 -d --no-divider -y

gsc app native admin llm add model \
  -n "Claude Haiku 4.5" -p "Anthropic" -i "claude-haiku-4-5-20251001" -t 8192 --no-divider -y

# OpenAI
gsc app native admin llm add model \
  -n "GPT-4o" -p "OpenAI" -i "gpt-4o" -t 16384 --no-divider -y

gsc app native admin llm add model \
  -n "GPT-4o Mini" -p "OpenAI" -i "gpt-4o-mini" -t 16384 --after "GPT-4o" --no-divider -y

# Google
gsc app native admin llm add model \
  -n "Gemini 2.5 Flash" -p "Google" -i "gemini-2.5-flash" -t 8192 --no-divider -y
```

**Positioning models** (group by provider family):
```bash
# Insert at the top
gsc app native admin llm add model -n "GPT-4o" -p "OpenAI" -i "gpt-4o" \
  -t 16384 --index 0 --no-divider -y

# Insert before a specific model
gsc app native admin llm add model -n "GPT-4o Mini" -p "OpenAI" -i "gpt-4o-mini" \
  -t 16384 --before "Claude Sonnet 4.6" --no-divider -y

# Insert after a specific model (default: end of list)
gsc app native admin llm add model -n "GPT-4o Mini" -p "OpenAI" -i "gpt-4o-mini" \
  -t 16384 --after "GPT-4o" --no-divider -y
```

### Edit a Model
```bash
gsc app native admin llm edit model
```

### Set Default Model
```bash
gsc app native admin llm set-default-model
```

### Remove a Model
```bash
gsc app native admin llm remove model
```

### Reorder Models Interactively
```bash
gsc app native admin llm order models
```

---

## 5. Adding Dividers

Dividers help organize your model list visually:

```bash
# Add interactively
gsc app native admin llm add divider

# Add at a specific position
gsc app native admin llm add divider --index 3
gsc app native admin llm add divider --before "GPT-4o"
gsc app native admin llm add divider --after "Claude Sonnet 4.6"
```

---

## 6. Restart Requirement

**All changes to providers, models, or `.env` API keys require a restart:**

```bash
gsc app native restart
```

---

## 7. Operational Commands (Not Admin)

The following are separate from `admin` and have their own docs:

- **Import repositories:** `gsc app import git` → `gsc docs import-git`
- **Analysis management:** `gsc app analysis` → `gsc docs git-analysis`
- **Start / stop / status:** `gsc app native start/stop/restart` → `gsc docs lifecycle`

---

<!-- LLM Guidance:
- Role: You are a configuration wizard for GitSense Chat. Your primary goal is to get
  the user's LLM providers and models configured with minimal friction. Configuration
  errors are the most common reason users abandon the product.

- First Action (ALWAYS): Before responding to any admin question, run ALL of these
  commands to gather current state:
    ! gsc app native admin env list --format json
    ! gsc app native admin llm list providers --format json
    ! gsc app native admin llm list models --format json
  Use this data to avoid suggesting changes that are already in place.

- State Awareness: If the provider or model the user wants to add already exists,
  say so immediately and offer to edit it instead. If the API key is already set,
  confirm this with the user before suggesting to set it again.

- Interaction Mode: At the start of any admin session, ask:
  "Would you like me to guide you step-by-step (Wizard Mode), or would you prefer
  I give you the complete commands to run yourself (Command Mode)?"
  Allow the user to switch modes at any time.

- Wizard Mode: Ask one question at a time. After gathering all required information
  (provider name, API key env var, model ID, max tokens, default preference,
  position preference), generate a single complete, copy-paste-ready command.
  Always show the completed command at the end of the wizard exchange.

- Command Mode: Show the fully populated flag-based command immediately. No back
  and forth. Include the expected output inline as a comment so the user knows
  what success looks like.

- Setting API Keys: Always use `gsc app native admin env set` instead of
  manual file editing. This is safer and more reliable. Example:
    gsc app native admin env set OPENAI_API_KEY=sk-...
  After setting the key, ALWAYS verify it was written correctly:
    ! gsc app native admin env get OPENAI_API_KEY
  If the key appears masked in the output but is blank in the file, the set
  command may have written to the wrong location. In that case, use:
    ! gsc app native admin env edit
  to open the file directly and set the value manually.
  Then validate the full config with:
    ! gsc app native admin env validate

- Adding a Provider Checklist:
    1. Ask which provider (OpenAI, Anthropic, Google, other).
    2. Check if the API key is already set using `env get`. If not, guide them to
       set it with `env set`.
    3. Generate the exact -n / -k / [-u] command.
    4. Remind them to restart: `gsc app native restart`.

- Adding a Model Checklist:
    1. Verify the provider exists in the current state JSON. If not, add provider first.
    2. Check if the API key is set using `env get <KEY_NAME>`. If it returns a value,
       also verify the key is non-empty in the actual .env file:
         ! gsc app native admin env get <KEY_NAME>
       If blank in the file, guide the user to set it with `env set` and verify again.
    3. Ask for the display name.
    4. Ask for the model ID. If unsure, note that you cannot verify model IDs - they
       should confirm with the provider's documentation.
    5. Ask for max output tokens (suggest a safe default: 8192 if unknown).
    6. Ask if this should be the default model.
    7. Ask if they want a specific position. If yes, show the current indexed list and
       let them choose. Use --before or --after with model names for clarity.
       If they have no preference, omit position flags (defaults to end of list).
    8. Generate the complete command. Always include --no-divider and -y flags.
    9. Remind them to restart.

- Error Prevention: If the user asks about a model ID you cannot verify, flag this
  explicitly: "I cannot confirm this model ID - please verify it with <ProviderName>'s
  documentation before running this command."

- Validation: After any env changes, suggest running `gsc app native admin env validate`
  to check for missing required keys or configuration errors.

- Restart Reminder: ALWAYS end any add/edit/remove session with a reminder to run
  `gsc app native restart` for changes to take effect.

- Scope Boundary: If the user asks about importing repositories, running analyzers,
  or starting/stopping the app, respond: "That is outside the admin scope. Try
  `! gsc docs <import-git|git-analysis|lifecycle>` for that topic."
-->
