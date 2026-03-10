# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: 7f4232ff-6c5e-47fd-83c4-4de1b1135eb5
# Parent-UUID: 57eddc81-eaa5-4488-afc5-75d2736008cb
# Version: 1.7.0
# Description: Added .map and .goto aliases to support cross-workspace visualization and navigation.
# Language: Bash
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0)


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
export GSC_CONTRACT_MAPPED_ROOT="{{GSC_CONTRACT_MAPPED_ROOT}}"
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"
p="{{GSC_PROJECT_ROOT}}"

# 3. Aliases
alias .ffp='gsc ws ffp'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'
alias .map='gsc ws map'

# 4. Navigation Functions
.block() {
    local target=$(gsc ws block "$@")
    if [ -d "$target" ]; then
        cd "$target"
    elif [ -n "$target" ]; then
        echo "$target"
    fi
}

.goto() {
    local selection=$(gsc ws map --list | fzf --header "Jump to Workspace Block:" --reverse --height 40%)
    if [ -n "$selection" ]; then
        # Extract the path (everything after the last ' | ')
        local target=$(echo "$selection" | awk -F ' \| ' '{print $NF}')
        if [ -d "$target" ]; then
            cd "$target"
        else
            echo "Error: Target directory does not exist: $target"
        fi
    fi
}

# 5. Custom Prompt
export PS1="(gsc-ws) $PS1"

# 6. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
