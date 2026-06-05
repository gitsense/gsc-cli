<!--
Component: GSC Docs Install
Block-UUID: 317f4341-8401-4d3f-bcd5-5ff5950ae611
Parent-UUID: 63d358c6-089e-4a51-aef6-07d93a94347b
Version: 1.5.0
Description: Installation wizard document for the GitSense Chat web application. Fixed path inconsistencies to use $GSC_HOME/data/.env for consistency with GSC_HOME environment variable, added symlink explanation, clarified GSC_HOME requirements for status commands, and updated references to use 'help' as primary command.
Language: Markdown
Created-at: 2026-05-30T01:49:27.657Z
Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
-->


# Installing the GitSense Chat App

The GitSense Chat App is the web-based interface that lets you interact with your codebases using AI. It runs locally on your machine at `http://localhost:3357`.

**Note:** GitSense Chat is continuously being improved. The initial release is **v0.1.0**, and we're actively adding new features and enhancements. The installation commands below will always fetch the latest available version.

There are two installation paths:

- **Native** - Runs directly on your host. Requires Node.js. Best for local development and **production use**.
- **Docker** - Runs in a container. Requires Docker Desktop or Docker CLI. Best for **quick UI preview only**.

> ⚠️ **Important:** Docker installation is intended for **UI preview only**. Due to path translation complexities between the host and container (the "twin brain" problem), Docker mode is **not supported for production work**. For full functionality, reliable operation, and all CLI features, please use the Native installation.

The `gsc` CLI you are currently using is already installed. This guide covers installing the **App** (the web server), not the CLI.

---

## Prerequisites

### Native Installation
- Node.js (v18 or higher recommended)
- npm (included with Node.js)
- Git (optional, for shadow repo operations)

### Docker Installation
- Docker CLI
- Docker daemon running

---

## Installation Commands

### Native
```bash
gsc app native install
```

With custom directories:
```bash
gsc app native install --app-dir <path> --data-dir <path>
```

### Docker
```bash
gsc app docker install
```

With custom directories:
```bash
gsc app docker install --data-dir <path> --repos-dir <path>
```

---

## What Happens During Installation

### Native
1. Downloads the latest release tarball from GitHub
2. Extracts it to a staging directory (`~/.gitsense/releases/<version>/app`)
3. Deploys to the active directory (`~/.gitsense/active/app`)
4. Runs `npm install` to install dependencies
5. Initializes the database (first-time only)
6. Creates a `.env` file from the template

### Docker
1. Verifies Docker is running
2. Creates data and repos directories (`~/.gitsense/docker-data`, `~/.gitsense/docker-repos`)
3. Pulls the GitSense Chat Docker image
4. Creates an `.env` file for Docker-specific configuration

---

## Post-Installation Steps

### 0. Set GSC_HOME Environment Variable (Native Only)

After installation, you must set the `GSC_HOME` environment variable to point to your app directory. This is required for the CLI to find the application.

**Temporary (current session only):**
```bash
export GSC_HOME=~/.gitsense/active/app
```

**Permanent (add to your shell config):**
```bash
# For zsh:
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.zshrc
source ~/.zshrc

# For bash:
echo 'export GSC_HOME=~/.gitsense/active/app' >> ~/.bashrc
source ~/.bashrc
```

### 1. Start the Application

You can start the application immediately after installation. The app will launch and you can explore the interface and built-in guides without needing to configure API keys first.

**Native:**
```bash
gsc app native start
```

**Docker:**
```bash
gsc app docker start
```

Open `http://localhost:3357` in your browser.

**What you can do without API keys:**
- Explore the GitSense Chat interface
- Read built-in guides like "Ergonomic Chats 101" and "Smarter Agents 101"
- Understand the workflow and features

**What requires API keys:**
- AI chat completions
- Code analysis and metadata extraction
- Creating analyzers and brains

### 2. Configure Your API Keys

To enable AI features, you'll need to configure your LLM API keys. This is the most important configuration step.

**Option A: Edit the .env file directly**

Open the `.env` file and add your API keys:
- Native: `$GSC_HOME/data/.env` (this is a symlink to `~/.gitsense/app/data/.env`)
- Docker: `~/.gitsense/docker-data/.env`

**Option B: Use the admin command**

List available providers:
```bash
gsc app native admin llm list providers
```

Add a model:
```bash
gsc app native admin llm add model
```

Edit provider settings:
```bash
gsc app native admin llm edit provider
```

After configuring keys, restart the application:
```bash
gsc app native restart
```

---

## Verify the Installation

Check that the app is running:
```bash
gsc app native status
```

For Docker:
```bash
gsc app docker status
```

**Note:** The `gsc app native status` command requires `GSC_HOME` to be set. If you get an error, make sure you've set the `GSC_HOME` environment variable as described in the "Post-Installation Steps" section.

---

## Changing the Default Port

If port `3357` is already in use, you can change it in the `.env` file:
```
PORT=3358
```

Then restart the app.

---

## Installing a Specific Version

By default, the installation commands fetch the latest version. If you need to install a specific version (for testing or compatibility), you can use the `--version` flag:

```bash
gsc app native install --version v0.1.0
```

This is typically only needed for advanced use cases. Most users should use the default (latest) installation.

---

## Uninstalling

To remove the application binaries (your data is preserved):
```bash
gsc app native uninstall
```

To also remove your data directory, delete `~/.gitsense/app/data` manually.

---

<!-- LLM Guidance:
- Role: You are the GitSense Installation Wizard. Your goal is to guide the user through installation and configuration. Do not dump all options at once - ask questions first, then provide the command.
- Primary Command: Always suggest `gsc docs help` to users as the primary entry point for documentation. `gsc docs init` is a technical alias that does the same thing.
- Messaging about continuous improvement: When introducing the installation, mention that "GitSense Chat is continuously being improved. The initial release is v0.1.0, and we're actively adding new features. The installation will always fetch the latest available version."
- Docker Warning (CRITICAL): If the user asks about Docker or seems interested in it, you MUST provide this warning: "⚠️ **Important:** Docker installation is intended for **UI preview only**. Due to path translation complexities between the host and container, Docker mode is **not supported for production work**. For full functionality and reliable operation, please use the Native installation." Do not let users choose Docker for production work without this warning.
- Wizard Decision Tree:
  1. FIRST: Ask "Do you have Docker installed and running on your system?"
     - If YES → Ask "Are you looking to quickly preview the UI, or do you need full functionality for production work?"
       - If "Preview UI" → Recommend Docker installation. Provide the warning above. Proceed to step 3.
       - If "Production work" → Strongly recommend Native installation. Ask about Node.js. Proceed to step 2.
     - If NO or UNSURE → Ask "Do you have Node.js installed? You can check by running: node --version"
       - If YES (Node.js found) → Recommend Native installation. Proceed to step 2.
       - If NO → Recommend installing Node.js first (https://nodejs.org), then return to Native path.
  2. Native path - Ask "Would you like to use the default installation directory (~/.gitsense), or do you have a preferred location?"
     - If DEFAULT → Provide: gsc app native install
     - If CUSTOM → Ask for app-dir and data-dir paths, then provide: gsc app native install --app-dir <path> --data-dir <path>
  3. Docker path → Provide: gsc app docker install (no further questions needed for default setup)
- After providing the command: Tell the user what will happen (brief summary from the "What Happens During Installation" section), then guide them through the post-install steps.
- Post-Install Guidance (CRITICAL - Reduce Frustration):
  1. First, guide them to set GSC_HOME (for native installs). This is required for the CLI to work.
  2. Then, tell them they can start the app immediately to explore the interface and built-in guides like "Ergonomic Chats 101" and "Smarter Agents 101" - no API keys needed for this.
  3. Finally, guide them through API key configuration. Explain that this is required for AI features. Offer two options: (A) edit .env file directly, or (B) use admin commands. Walk them through the process step-by-step.
  4. Remind them to restart the app after configuring keys.
- Built-in Guides: Mention that the app includes built-in guides that don't require API keys. This helps users understand they can explore the interface immediately without configuration.
- GSC_HOME is critical: For native installs, emphasize that setting GSC_HOME is required. Provide both temporary and permanent setup commands. If the user reports issues with `gsc app native status` or `gsc app native start` not working, first ask if they've set GSC_HOME.
- Admin command correction: When suggesting how to configure API keys, do not suggest `gsc app native admin` by itself (it only shows help). Instead, suggest specific subcommands like `gsc app native admin llm list providers` or `gsc app native admin llm add model`.
- Port conflict handling: If the user reports the app is not accessible or a port conflict, ask them to check `$GSC_HOME/data/.env` and change the PORT value, then restart.
- Clarifying "install gsc" vs "install the app": If the user says "install gitsense" or "install gsc", clarify: "The `gsc` CLI is already installed - you're using it right now. This guide covers installing the GitSense Chat **web application**, which is a separate component that runs a local server at http://localhost:3357. Shall I continue with that?"
- Asking about Node.js: If the user is unsure whether they have Node.js, suggest they run `node --version` in a separate terminal and tell you the result.
- Specific version requests: If a user asks about installing a specific version (e.g., "I want v0.1.0"), provide the command with the --version flag: `gsc app native install --version v0.1.0`. Otherwise, always use the default (latest).
- Next Step: After the user confirms installation is complete, guide them through: (1) set GSC_HOME, (2) start the app to explore built-in guides, (3) configure API keys for AI features. Offer to help with each step.
- Documentation References: When suggesting users explore more topics, always use `gsc docs help` as the primary command. For example: "For more information, run `gsc docs help` to see all available documentation topics."
-->
