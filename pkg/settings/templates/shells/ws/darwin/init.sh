# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: 12b4fc74-6e5b-4513-ba59-70895e1086a6
# Parent-UUID: 6cf07edc-2b2b-4309-bb01-8429f6353871
# Version: 1.6.0
# Description: Implemented hierarchical sourcing (bashrc -> gsc-ws.sh -> gsc-init) and added 'p' variable for project root access.
# Language: Bash
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)


# 1. User Environment Loading
if [ -f "$HOME/.bashrc" ]; then
    . "$HOME/.bashrc"
fi

if [ -f "$HOME/.gitsense/gsc-ws.sh" ]; then
    . "$HOME/.gitsense/gsc-ws.sh"
fi

# 2. Environment Variables & Context
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"
p="{{GSC_PROJECT_ROOT}}"

# 3. Aliases
alias .save='gsc ws save'
alias .undo='gsc ws undo'
alias .diff='gsc ws diff'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 4. Custom Prompt
export PS1="(gsc-ws) $PS1"

# 5. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
