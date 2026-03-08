# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: 8b9c0d1e-4f5a-6b7c-8d9e-0f1a2b3c4d5e
# Parent-UUID: 5a8f9c2d-3e4b-4a1f-8b7d-9c0e1f2a3b4c
# Version: 1.1.0
# Description: Updated to use dot-prefixed aliases and GSC_SCRIPTS_DIR for context-aware initialization.
# Language: Zsh
# Created-at: 2026-03-07T20:10:00.000Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)


# GitSense Workspace Init (Zsh Native)

# 1. Environment Variables
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"

# 2. Aliases
alias .save='gsc ws save'
alias .undo='gsc ws undo'
alias .diff='gsc ws diff'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 3. Custom Prompt (Zsh Syntax)
# %F{cyan} = Cyan Foreground, %f = Reset Color, %# = Prompt Char ($ or #)
export PROMPT="%F{cyan}(gsc-ws)%f %~"$'\n'"%# "

# 4. Welcome Message
clear
cat ${GSC_SCRIPTS_DIR}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
