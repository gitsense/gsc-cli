# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: d74ffbec-01fb-4411-9a54-78b57cb46f95
# Parent-UUID: f8bf9f37-2fe5-4c6c-bfc6-91d08cf4d9ae
# Version: 1.12.0
# Description: Updated .switch to use 'cd' instead of 'gsc ws' to preserve the current shell session.
# Language: Bash
# Created-at: 2026-03-10T04:02:26.376Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), Gemini 3 Flash (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0)


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
        # Extract the relative path (everything after the last ' | ')
        local rel_path=$(echo "$selection" | awk -F ' \| ' '{print $NF}')
        # Prepend the mapped root to get the absolute path
        local target="$GSC_CONTRACT_MAPPED_ROOT/$rel_path"
        if [ -d "$target" ]; then
            cd "$target"
        else
            echo "Error: Target directory does not exist: $target"
        fi
    fi
}

.switch() {
    local selection=$(ls -1 "$GSC_CONTRACT_MAPPED_ROOT" | fzf --header "Switch Workspace:" --reverse --height 40%)
    if [ -n "$selection" ]; then
        cd "$GSC_CONTRACT_MAPPED_ROOT/$selection"
    fi
}

# 5. Custom Prompt
export PS1="(gsc-ws) $PS1"

# 6. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
