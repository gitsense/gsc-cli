# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: 5a8f9c2d-3e4b-4a1f-8b7d-9c0e1f2a3b4c
# Parent-UUID: N/A
# Version: 1.0.0
# Description: Native Zsh initialization script for GitSense workspaces on macOS, handling environment variables, aliases, and Zsh-specific prompts.
# Language: Zsh
# Created-at: 2026-03-07T20:10:00.000Z
# Authors: GLM-4.7 (v1.0.0)


# GitSense Workspace Init (Zsh Native)

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

# 3. Custom Prompt (Zsh Syntax)
# %F{cyan} = Cyan Foreground, %f = Reset Color, %# = Prompt Char ($ or #)
export PROMPT="%F{cyan}(gsc-ws)%f %~"$'\n'"%# "

# 4. Welcome Message
clear
cat ${GSC_MAPPED_WS_ROOT}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
