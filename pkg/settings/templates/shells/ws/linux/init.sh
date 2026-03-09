# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: 57eddc81-eaa5-4488-afc5-75d2736008cb
# Parent-UUID: 86e01934-d7a3-42b7-9025-d0d0c9af994d
# Version: 1.6.0
# Description: Added .block shell function to enable workspace navigation.
# Language: Bash
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)


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
