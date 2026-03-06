# GitSense Workspace Shell Init
# Generated for Workspace: {{GSC_WS_HASH}}

# 1. Environment Variables
export GSC_CHAT_ID="{{GSC_CHAT_ID}}"
export GSC_WS_HASH="{{GSC_WS_HASH}}"
export GSC_PROJECT_ROOT="{{GSC_PROJECT_ROOT}}"
export GSC_CONTRACT_UUID="{{GSC_CONTRACT_UUID}}"

# 2. Aliases
alias save='gsc ws save'
alias undo='gsc ws undo'
alias diff='gsc ws diff'
alias help='cat .gsc-welcome'

# 3. Custom Prompt
export PS1="(gsc-ws) \w $ "

# 4. Welcome Message
clear
cat .gsc-welcome
