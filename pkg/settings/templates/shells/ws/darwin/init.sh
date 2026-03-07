# Component: GitSense Workspace Shell Init
# Block-UUID: 73fc5657-d62b-43e2-98ad-413ec0947399
# Parent-UUID: 1263b9b7-86ca-4965-b9bf-c95b03ac3ed1
# Version: 1.4.0
# Description: Updated to use GSC_MAPPED_WS_ROOT for absolute path resolution and renamed GSC_WS_HASH to GSC_MAPPED_WS_HASH.
# Language: Bash
# Created-at: 2026-03-07T20:08:00.924Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)


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
# Bash uses \w for current directory
export PS1="(gsc-ws) \w\n$ "

# 4. Welcome Message
clear
cat ${GSC_MAPPED_WS_ROOT}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
