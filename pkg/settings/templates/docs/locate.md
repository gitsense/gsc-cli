<!--
Component: GSC Docs Locate
Block-UUID: 8621fee3-0951-44ba-8553-60bcfa18621c
Parent-UUID: N/A
Version: 1.0.0
Description: Guide for finding where GitSense Chat is installed and where data is stored.
Language: Markdown
Created-at: 2026-05-31T14:45:00.000Z
Authors: GLM-4.7 (v1.0.0)
-->


# Finding GitSense Chat Installation & Data

This guide helps you locate where GitSense Chat is installed on your system and where your data is stored.

---

## 1. The GSC_HOME Environment Variable

GitSense Chat uses the `GSC_HOME` environment variable to determine where to store data and configuration.

### Check if GSC_HOME is Set

```bash
echo $GSC_HOME
```

If it returns a path, `GSC_HOME` is set. If it returns empty, GitSense Chat will use the default location.

### Default Location

If `GSC_HOME` is not set, GitSense Chat uses:

```
~/.gitsense/active/app
```

### Set GSC_HOME (Temporary)

For the current terminal session only:

```bash
export GSC_HOME=~/.gitsense/active/app
```

### Set GSC_HOME (Permanent)

Add to your shell configuration file:

**For zsh:**
```bash
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.zshrc
source ~/.zshrc
```

**For bash:**
```bash
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.bashrc
source ~/.bashrc
```

---

## 2. Directory Structure

GitSense Chat stores data in several locations under `GSC_HOME`:



```
$GSC_HOME/
├── app/                    # Application files (Node.js app)
│   ├── node_modules/       # Dependencies
│   ├── package.json        # App configuration
│   └── ...                 # Other app files
├── data/                   # Persistent data
│   ├── .env                # Environment variables (API keys, port)
│   ├── chats.sqlite3       # Chat database
│   └── logs/               # Application logs
│       └── app.log         # Main application log
├── shadow-repos/           # Shadow repository snapshots
│   └── <owner>/<repo>/<branch>/
└── releases/               # Downloaded release archives
    └── <version>/
```

---

## 3. Finding Specific Files

### Database Location

The main database is at:

```bash
$GSC_HOME/data/chats.sqlite3
```

Or use the default:

```bash
~/.gitsense/active/app/data/chats.sqlite3
```

### Log Files

Application logs are stored in:

```bash
$GSC_HOME/data/logs/
```

View the current log:

```bash
gsc app native logs
```

List all log files:

```bash
gsc app native logs --list
```

### Configuration (.env)

The environment configuration file is at:

```bash
$GSC_HOME/data/.env
```

Edit it to change API keys, port, or other settings:

```bash
nano $GSC_HOME/data/.env
```

Or use the admin command:

```bash
gsc app native admin env edit
```

### Shadow Repositories

Shadow repository snapshots are stored in:

```bash
$GSC_HOME/shadow-repos/<owner>/<repo>/<branch>/
```

Check shadow status:

```bash
gsc app import git --status
```

### Analysis Data

Analysis dumps are stored in:

```bash
~/.gitsense/data/analysis/<analyzer>/<owner>/<repo>/<branch>.jsonl
```

Example for code-intent analysis:

```bash
~/.gitsense/data/analysis/code-intent/myorg/myrepo/main.jsonl
```

---

## 4. Using Status Commands

The easiest way to find paths is to use the status commands:

### Native Installation

```bash
gsc app native status
```

This shows:
- Data directory path
- `.env` file location
- Log file location
- App directory
- Installed version

### Docker Installation

```bash
gsc app docker status
```

This shows:
- Container status
- Volume paths (data, repos)
- Port mapping

---

## 5. Platform Differences

### Native Installation

- **App Location:** `$GSC_HOME/app/`
- **Data Location:** `$GSC_HOME/data/`
- **Logs:** `$GSC_HOME/data/logs/`
- **Shadow Repos:** `$GSC_HOME/shadow-repos/`

### Docker Installation

- **Container:** Managed by Docker
- **Data Volume:** `~/.gitsense/docker-data/`
- **Repos Volume:** `~/.gitsense/docker-repos/`
- **Logs:** Docker container logs (`docker logs gitsense-chat`)

---

## 6. Common Questions

**Q: Where is my chat history stored?**
A: In the SQLite database at `$GSC_HOME/data/chats.sqlite3`.

**Q: Where are my API keys stored?**
A: In the `.env` file at `$GSC_HOME/data/.env`.

**Q: Where can I find error logs?**
A: In `$GSC_HOME/data/logs/app.log` or use `gsc app native logs`.

**Q: How much disk space is my shadow repo using?**
A: Run `gsc app import git --status` to see shadow repo size.

**Q: Can I move my installation to a different location?**
A: Yes. Set `GSC_HOME` to the new location, then reinstall the app with `gsc app native install --app-dir <new-path>`.

---

## 7. Troubleshooting

### GSC_HOME Not Set

If you get errors about `GSC_HOME` not being set:

```bash
# Check if it's set
echo $GSC_HOME

# Set it temporarily
export GSC_HOME=~/.gitsense/active/app

# Set it permanently (zsh)
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.zshrc
source ~/.zshrc
```

### Database Not Found

If the database is missing:

```bash
# Check if it exists
ls -la $GSC_HOME/data/chats.sqlite3

# If missing, the app may not have been started yet
# Start the app to initialize the database
gsc app native start
```

### Permission Denied

If you get permission errors:

```bash
# Check ownership
ls -la $GSC_HOME

# Fix ownership (replace $USER with your username)
sudo chown -R $USER:$USER $GSC_HOME
```

---

<!-- LLM Guidance:
- Role: You are a GitSense technical assistant helping a user locate their installation and data.
- Primary Command: Always suggest `gsc app native status` or `gsc app docker status` as the first step to find paths.
- Ambiguity - "where is my data": Ask if they mean chat history, logs, configuration, or shadow repos. Then provide the specific path.
- Ambiguity - "where is it installed": Clarify if they mean the CLI (already installed) or the Chat App (web application). For the app, show how to find `GSC_HOME`.
- GSC_HOME is critical: If the user reports issues with any command that requires `GSC_HOME`, first ask if they've set it. Provide both temporary and permanent setup commands.
- Platform differences: Always ask if they're using Native or Docker installation, as paths differ significantly.
- Next Step: After helping locate files, ask if they need help with anything else (e.g., viewing logs, editing configuration, managing disk space).
- Documentation References: Use `gsc docs help` to show the full documentation roadmap.
-->
