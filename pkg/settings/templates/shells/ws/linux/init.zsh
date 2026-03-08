# Component: GitSense Workspace Shell Init (Zsh)
# Block-UUID: bf926dbd-b19f-45af-86d3-739fda4a3fd0
# Parent-UUID: 5a8f9c2d-3e4b-4a1f-8b7d-9c0e1f2a3b4c
# Version: 1.2.0
# Description: Implemented hierarchical sourcing (zshrc -> gsc-ws.zsh -> gsc-init) and added 'p' variable for project root access.
# Language: Zsh
# Created-at: 2026-03-08T16:30:23.301Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0)


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
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"
p="{{GSC_PROJECT_ROOT}}"

# 3. Aliases
alias .save='gsc ws save'
alias .undo='gsc ws undo'
alias .diff='gsc ws diff'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 4. Prompt Protection
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
