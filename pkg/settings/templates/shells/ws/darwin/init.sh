# Component: GitSense Workspace Shell Init
# Block-UUID: 6cf07edc-2b2b-4309-bb01-8429f6353871
# Parent-UUID: 73fc5657-d62b-43e2-98ad-413ec0947399
# Version: 1.5.0
# Description: Moved init scripts to parent mapped directory, updated aliases to use dot prefix (e.g., .save), and replaced GSC_MAPPED_WS_ROOT with GSC_SCRIPTS_DIR.
# Language: Bash
# Created-at: 2026-03-07T20:08:00.924Z
# Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)


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
# Bash uses \w for current directory
export PS1="(gsc-ws) \w\n$ "

# 4. Welcome Message
clear
cat ${GSC_SCRIPTS_DIR}/.gsc-welcome

# 5. Navigate to Target Directory
cd "{{TARGET_DIR}}"
