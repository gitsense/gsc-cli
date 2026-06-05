<!--
Component: GSC Docs Lifecycle
Block-UUID: 834b61ab-bfc5-4fdb-8602-69f098db9c85
Parent-UUID: 95be2d44-942e-462d-b4ce-5023d7ad4d19
Version: 1.1.0
Description: Lifecycle management guide for the GitSense Chat web application, including start, stop, restart, status, and logs commands for both Native and Docker deployments. Fixed incorrect symlink claim for .env file and added missing --env-file flag documentation.
Language: Markdown
Created-at: 2026-05-31T08:00:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
-->


# Managing the GitSense Chat Application Lifecycle

This guide covers starting, stopping, restarting, and monitoring the GitSense Chat web application for both Native and Docker installations.

## Quick Reference

| Command | Native | Docker |
|---------|--------|--------|
| Start | `gsc app native start` | `gsc app docker start` |
| Stop | `gsc app native stop` | `gsc app docker stop` |
| Restart | `gsc app native restart` | `gsc app docker restart` |
| Status | `gsc app native status` | `gsc app docker status` |
| Logs | `gsc app native logs` | `gsc app docker logs` |

---

## Starting the Application

### Native

```bash
gsc app native start
```

**What happens:**
- The application starts in the background as a daemon
- It binds to the configured port (default: 3357)
- The process supervisor monitors the app and restarts it on crashes
- Logs are written to `$GSC_HOME/data/logs/app.log`
- A health status file tracks uptime, crash count, and port

**Flags:**
- `--port` - Override the listening port (default: 3357)
- `--foreground` - Run in the foreground instead of as a background daemon
- `--app-dir` - Path to the Node.js app root (default: from `GSC_HOME` env var or `native-config.json`)
- `--data-dir` - Path to the persistent data directory
- `--env-file` - Path to the `.env` file to load
- `--max-retries` - Maximum number of restart attempts on crash

### Docker

```bash
gsc app docker start
```

**What happens:**
- The container starts in the background
- The configured host port is mapped to the container
- If no API keys are configured, a warning is displayed
- A Docker context file is saved for subsequent operations

**Flags:**
- `--port` - Host port to map (default: 3357)
- `--repos-dir` - Host path to your Git repositories
- `--data-dir` - Host path for persistent data
- `--env-file` - Path to a `.env` file containing API keys
- `--name` - Custom container name
- `--pull` - Force pull the latest image before starting

**Verification:**
1. Check status: `gsc app native status` or `gsc app docker status`
2. Open browser: `http://localhost:3357`
3. Check logs: `gsc app native logs --lines 20` or `gsc app docker logs`

---

## Stopping the Application

### Native

```bash
gsc app native stop
```

Sends a graceful termination signal to the running application. The process and its supervisor are both stopped.

### Docker

```bash
gsc app docker stop
```

Stops and removes the Docker container.

---

## Restarting the Application

Use this after configuration changes (API keys, models, etc.).

### Native

```bash
gsc app native restart
```

Stops the application, waits for it to fully terminate, then starts it again with the same configuration.

**Flags:**
- `--port` - Override the listening port (default: 3357)
- `--foreground` - Run in the foreground instead of as a background daemon
- `--app-dir` - Path to the Node.js app root (default: from `GSC_HOME` env var or `native-config.json`)
- `--data-dir` - Path to the persistent data directory
- `--env-file` - Path to the `.env` file to load
- `--max-retries` - Maximum number of restart attempts on crash

### Docker

```bash
gsc app docker restart
```

Restarts the Docker container in place.

---

## Checking Status

### Native

```bash
gsc app native status
```

**Output shows:**
- Running state (running/stopped)
- Process ID and supervisor PID (if running)
- Uptime (if running)
- Port (if running, read from health status)
- Crash count and last crash time
- Restart count
- Data directory path
- `.env` file status (PRESENT/MISSING)
- Log file location
- App directory and installed version

### Docker

```bash
gsc app docker status
```

**Output shows:**
- Container status
- Container name
- Port mapping
- Uptime and restart count
- Configuration type and status
- Volume paths

---

## Application Logs

### Native

**View last 100 lines (default):**
```bash
gsc app native logs
```

**View last N lines:**
```bash
gsc app native logs --lines 50
```

**Follow logs in real-time:**
```bash
gsc app native logs --follow
```
This behaves like `tail -f`. Press Ctrl+C to stop. The command handles file rotation gracefully and will continue monitoring even if the log file is rotated.

**List all available log files:**
```bash
gsc app native logs --list
```

This displays all log files in `$GSC_HOME/data/logs/` with:
- File name (with "(current)" marker for the active log)
- File size
- Last modification time
- Time since last modification

### Docker

```bash
gsc app docker logs
```

**Follow container logs in real-time:**
```bash
gsc app docker logs --follow
```

---

## Log Locations

- **Native:** `$GSC_HOME/data/logs/app.log`
- **Docker:** Container logs (also viewable with `docker logs gitsense-chat`)

---

## Environment Variables

### Native

The application requires `GSC_HOME` to be set. If you haven't set it yet:

```bash
# Temporary (current session)
export GSC_HOME=~/.gitsense/active/app

# Permanent (add to shell config)
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.zshrc
source ~/.zshrc
```

The `.env` file is located at `$GSC_HOME/data/.env`. Edit this file to configure API keys and port.

### Docker

The `.env` file is managed through the Docker data directory. Use:
- `gsc app docker env init` - Initialize a master or Docker-only `.env` file
- `gsc app docker env link <path>` - Link an existing `.env` file
- `gsc app docker env update` - Sync changes from the linked file

---

## Troubleshooting

### Application Won't Start

1. **Check GSC_HOME is set** (Native only):
```bash
echo $GSC_HOME
```
If empty, set it as shown above.

2. **Check logs for errors:**
```bash
gsc app native logs --lines 50
gsc app docker logs
```

3. **Common errors:**
- `EADDRINUSE` - Port already in use. Change the port in `.env` or kill the conflicting process.
- `Failed to load native config` - The app hasn't been installed or `GSC_HOME` is not set.
- `Container is not running` - Docker daemon may be stopped. Check with `docker ps`.
- `.env file not found` - API keys not configured. The app may start but AI features will be unavailable.

### Port Already in Use

**Option 1: Change the port**
```bash
# Native: Edit .env file
nano $GSC_HOME/data/.env
# Change PORT=3357 to PORT=3358

# Docker: Recreate with different port
gsc app docker start --port 3358
```

Then restart:
```bash
gsc app native restart
gsc app docker restart
```

**Option 2: Kill the conflicting process**
```bash
# Find process using the port
lsof -i :3357

# Kill it
kill -9 <PID>
```

### Application Crashed

Check the status for crash information:
```bash
gsc app native status
```
This shows the crash count and time of the last crash. The supervisor automatically restarts the app on crashes.

### Process Won't Stop (Native)

Check the status to get the PID:
```bash
gsc app native status
```

If graceful stop fails, manually kill:
```bash
kill -9 <PID>
```

### Docker Container Not Responding

Check container logs:
```bash
gsc app docker logs
```

Restart the container:
```bash
gsc app docker restart
```

Run diagnostics:
```bash
gsc app docker admin doctor
```

### .env File Sync Issues (Docker)

Check sync status:
```bash
gsc app docker env
```

If out of sync:
```bash
gsc app docker env update
gsc app docker restart
```

---

## Disk Management (Native)

Old release archives can accumulate over time. To free disk space:

```bash
gsc app native cleanup
```

By default, this keeps the 3 most recent releases. Use `--keep` to adjust:
```bash
gsc app native cleanup --keep 5
```

Use `--dry-run` to preview what would be removed without actually deleting anything:
```bash
gsc app native cleanup --dry-run
```

---

<!-- LLM Guidance:
- Role: You are the GitSense Operations Guide. Your goal is to help users manage the application lifecycle effectively.
- Tone: Practical and troubleshooting-focused. Be direct with diagnostic steps.
- Primary Command: Always suggest `gsc docs help` as the entry point for documentation discovery. `gsc docs init` is a technical alias.
- Diagnostic Approach: When a user reports a problem, follow this order:
  1. Check status: `gsc app native status` or `gsc app docker status`
  2. Check logs: `gsc app native logs --lines 50` or `gsc app docker logs`
  3. Identify the error message
  4. Provide specific solution based on the error
- Common Issues Reference:
  - Port conflicts (EADDRINUSE) → Suggest changing port in .env or Docker start flag
  - Missing GSC_HOME → Provide temporary and permanent export commands
  - .env file missing → Guide user to create or link one
  - Container not running → Suggest `gsc app docker start` or `docker ps`
  - Log directory not found → The app may never have been started; suggest starting it first
  - .env sync issues (Docker) → Suggest `gsc app docker env update`
- Log Commands Summary:
  - Native:
    - `gsc app native logs` (last 100 lines)
    - `gsc app native logs --lines N` (last N lines)
    - `gsc app native logs --follow` (real-time tailing)
    - `gsc app native logs --list` (list all available log files)
  - Docker:
    - `gsc app docker logs` (view logs)
    - `gsc app docker logs --follow` (follow logs)
- Distinction Between CLI and App: If a user says "start gitsense" or "how do I start it?", clarify that the `gsc` CLI is already running (they're using it), and this guide covers starting the GitSense Chat web application.
- Administration Commands: For managing LLM models/providers, guide users to `gsc app native admin llm` or `gsc app docker admin llm`. For environment variables, guide to `gsc app native admin env` or `gsc app docker env`.
- Docker Warning: If the user is using Docker, remind them that Docker mode is intended for UI preview only. If they need production functionality, suggest Native installation.
- Next Steps: After resolving an issue, suggest verification steps:
  - "Check status with: `gsc app native status`"
  - "Open http://localhost:3357 in your browser"
  - "Check logs with: `gsc app native logs --lines 20`"
- After a restart for configuration changes: Suggest verifying the new configuration is active (e.g., check models, test chat).
- Disk Management: If the user mentions disk space concerns, suggest `gsc app native cleanup` to remove old release archives.
- Documentation References: Use `gsc docs help` as the primary command for exploring other documentation topics.
-->
