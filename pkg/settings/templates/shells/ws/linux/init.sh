# Component: GitSense Workspace Shell Init
# Block-UUID: 2a404237-e206-43a3-9534-caf8d600c77c
# Parent-UUID: N/A
# Version: 1.1.0
# Description: Shell initialization script for shadow workspaces on Linux. Sets environment variables, aliases, displays the welcome screen, and navigates to the target directory.
# Language: Bash
# Created-at: 2026-03-06T05:20:00.000Z
# Authors: GLM-4.7 (v1.0.0)


# GitSense Workspace Shell Init
# Generated for Workspace: {{GSC_WS_HASH}}

# 1. Environment Variables
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_WS_HASH="{{GSC_WS_HASH}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"

# 2. Aliases
alias save='gsc ws save'
alias undo='gsc ws undo'
alias diff='gsc ws diff'
alias help='cat .gsc-welcome'

# 3. Custom Prompt
export PS1="(gsc-ws) \w $ "

# 4. Welcome Message
clear
cat .gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
