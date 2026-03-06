# Component: GitSense Workspace Shell Init
# Block-UUID: f31d0e06-888b-4b2d-8eed-519aa45c234c
# Parent-UUID: 2a404237-e206-43a3-9534-caf8d600c77c
# Version: 1.2.0
# Description: Updated to use GSC_MAPPED_WS_ROOT for absolute path resolution and renamed GSC_WS_HASH to GSC_MAPPED_WS_HASH.
# Language: Bash
# Created-at: 2026-03-06T05:20:00.000Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0)


# GitSense Workspace Shell Init
# Generated for Workspace: {{GSC_MAPPED_WS_HASH}}

# 1. Environment Variables
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_MAPPED_WS_HASH="{{GSC_MAPPED_WS_HASH}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"
export GSC_MAPPED_WS_ROOT="{{GSC_MAPPED_WS_ROOT}}"

# 2. Aliases
alias save='gsc ws save'
alias undo='gsc ws undo'
alias diff='gsc ws diff'
alias help='cat ${GSC_MAPPED_WS_ROOT}/.gsc-welcome'

# 3. Custom Prompt
export PS1="(gsc-ws) \w\n$ "

# 4. Welcome Message
clear
cat ${GSC_MAPPED_WS_ROOT}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
