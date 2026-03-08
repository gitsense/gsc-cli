# Component: GitSense Workspace Shell Init
# Block-UUID: 9e68c4c0-bf5a-4226-91a9-656aa63f41b4
# Parent-UUID: f31d0e06-888b-4b2d-8eed-519aa45c234c
# Version: 1.3.0
# Description: Moved init scripts to parent mapped directory, updated aliases to use dot prefix (e.g., .save), and replaced GSC_MAPPED_WS_ROOT with GSC_SCRIPTS_DIR.
# Language: Bash
# Created-at: 2026-03-06T05:20:00.000Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)


# GitSense Workspace Shell Init

# 1. Environment Variables
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"
export GSC_SCRIPTS_DIR="{{GSC_SCRIPTS_DIR}}"

# 2. Aliases
alias .save='gsc ws save'
alias .undo='gsc ws undo'
alias .diff='gsc ws diff'
alias .send='gsc ws send'
alias .help='cat ${GSC_SCRIPTS_DIR}/.gsc-welcome'

# 3. Custom Prompt
export PS1="(gsc-ws) \w\n$ "

# 4. Welcome Message
clear
cat ${GSC_SCRIPTS_DIR}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
