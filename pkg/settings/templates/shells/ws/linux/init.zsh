# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: 8aea5b45-5719-431b-b260-5396aa38c393
# Parent-UUID: d88ed6c2-b475-4ff3-bc5e-055e6735c608
# Version: 1.5.0
# Description: Added .map and .goto aliases to support cross-workspace visualization and navigation.
# Language: Zsh
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0)


# 1. User Environment Loading
if [[ -f "$HOME/.zshrc" ]]; then
    source "$HOME/.zshrc"
fi

if [[ -f "$HOME/.gitsense/gsc-ws.zsh" ]]; then
    source "$HOME/.gitsense/gsc-ws.zsh"
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

# 4. Block Navigation Function
.block() {
    local target=$(gsc ws block "$@")
    if [[ -d "$target" ]]; then
        cd "$target"
    elif [[ -n "$target" ]]; then
        echo "$target"
    fi
}

.goto() {
    local selection=$(gsc ws map --list | fzf --header "Jump to Workspace Block:" --reverse --height 40%)
    if [[ -n "$selection" ]]; then
        # Extract the path (everything after the last ' | ')
        local target=$(echo "$selection" | awk -F ' \| ' '{print $NF}')
        if [[ -d "$target" ]]; then
            cd "$target"
        else
            echo "Error: Target directory does not exist: $target"
        fi
    fi
}

# 5. Prompt Protection
_gsc_prompt_hook() {
    if [[ "$PROMPT" != *"(gsc-ws)"* ]]; then
        export PROMPT="%F{cyan}(gsc-ws)%f $PROMPT"
    fi
}

if [[ -n "$precmd_functions" ]]; then
    precmd_functions+=(_gsc_prompt_hook)
else
    export PROMPT="%F{cyan}(gsc-ws)%f %~"$'\n'"%# "
fi

# 6. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
