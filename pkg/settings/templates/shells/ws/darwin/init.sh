# Component: GitSense Workspace Shell Init
# Block-UUID: 1263b9b7-86ca-4965-b9bf-c95b03ac3ed1
# Parent-UUID: f9ce4bf7-0838-459d-a0d0-3653bdcf4ed7
# Version: 1.3.0
# Description: Updated to use GSC_MAPPED_WS_ROOT for absolute path resolution and renamed GSC_WS_HASH to GSC_MAPPED_WS_HASH.
# Language: Bash
# Created-at: 2026-03-07T01:40:07.644Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)


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
if [ -n "$ZSH_VERSION" ]; then
    # Zsh uses %~ for current directory
    export PS1="(gsc-ws) %~"$'\n'"$ "
else
    # Bash uses \w for current directory
    export PS1="(gsc-ws) \w\n$ "
fi

# 4. Welcome Message
clear
cat ${GSC_MAPPED_WS_ROOT}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
