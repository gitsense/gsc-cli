# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: abc3b7ca-7e9a-4319-8b2e-2b3f2c7bdebd
# Parent-UUID: a1344ae9-db91-416c-a706-d754be6346fd
# Version: 1.4.0
# Description: Added .block shell function to enable workspace navigation.
# Language: Zsh
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)


# 1. User Environment Loading
# Source the user's main zshrc to preserve tools, themes, and plugins.
if [[ -f "$HOME/.zshrc" ]]; then
    source "$HOME/.zshrc"
fi

# Source the optional GitSense User Workspace Profile for custom aliases.
if [[ -f "$HOME/.gitsense/gsc-ws.zsh" ]]; then
    source "$HOME/.gitsense/gsc-ws.zsh"
fi

# 2. Environment Variables & Context
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"

# The 'p' variable: Dead simple access to your project root.
p="{{GSC_PROJECT_ROOT}}"

# 3. Aliases
alias .ffp='gsc ws ffp'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 4. Block Navigation Function
.block() {
    local target=$(gsc ws block "$@")
    if [[ -d "$target" ]]; then
        cd "$target"
    elif [[ -n "$target" ]]; then
        echo "$target"
    fi
}

# 5. Prompt Protection
# Use a precmd hook to ensure (gsc-ws) is prepended to the prompt, surviving themes.
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
