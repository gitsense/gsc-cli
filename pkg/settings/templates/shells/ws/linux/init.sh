# Component: GitSense Workspace Shell Init (Bash)
# Block-UUID: cfe41e1d-be4a-45ad-9b68-85fc8e366fd3
# Parent-UUID: ee1ec9b5-70d5-4866-b218-b3104201f9b8
# Version: 1.9.0
# Description: Added .switch alias to support switching between message workspaces using fzf.
# Language: Bash
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0)


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
        gsc ws "$selection"
    fi
}

# 5. Custom Prompt
export PS1="(gsc-ws) $PS1"

# 6. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
