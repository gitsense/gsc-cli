# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: a1344ae9-db91-416c-a706-d754be6346fd
# Parent-UUID: 8b9c0d1e-4f5a-6b7c-8d9e-0f1a2b3c4d5e
# Version: 1.2.0
# Description: Implemented hierarchical sourcing (zshrc -> gsc-ws.zsh -> gsc-init) and added 'p' variable for project root access.
# Language: Zsh
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0)


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
alias .save='gsc ws save'
alias .undo='gsc ws undo'
alias .diff='gsc ws diff'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 4. Prompt Protection
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

# 5. Initialization
clear
cat "${GSC_SCRIPTS_DIR}/.gsc-welcome"
cd "{{TARGET_DIR}}"
