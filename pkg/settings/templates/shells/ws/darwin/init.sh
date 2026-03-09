# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: f2b3fb09-b4ea-412d-b9d9-f9681d171f8e
# Parent-UUID: 859c159e-ca9d-4368-a910-65c010e75840
# Version: 1.8.0
# Description: Removed deprecated aliases (.save, .undo, .diff) and added .ffp alias for 'gsc ws ffp'.
# Language: Bash
# Created-at: 2026-03-09T17:43:11.362Z
# Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0)


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
alias .ffp='gsc ws ffp'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 4. Block Navigation Function
.block() {
    local target=$(gsc ws block "$@")
    if [ -d "$target" ]; then
        cd "$target"
    elif [ -n "$target" ]; then
        echo "$target"
    fi
}

# 5. Custom Prompt
export PS1="(gsc-ws) $PS1"

# 6. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
