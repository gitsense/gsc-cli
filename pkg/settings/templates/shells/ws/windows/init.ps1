<#
Component: GitSense Workspace Shell Init (PowerShell)
Block-UUID: 178f6827-81af-49df-bb38-165c0378f771
Parent-UUID: cea2a8f7-0f7a-442b-a7ff-6aa2698fc9e9
Version: 1.4.0
Description: Implemented hierarchical sourcing (PROFILE -> gsc-ws.ps1 -> gsc-init) and added 'p' variable for project root access.
Language: PowerShell
Created-at: 2026-03-08T16:30:23.301Z
Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.4.0)
#>


# 1. User Environment Loading
if (Test-Path $PROFILE) {
    . $PROFILE
}

if (Test-Path "$HOME/.gitsense/gsc-ws.ps1") {
    . "$HOME/.gitsense/gsc-ws.ps1"
}

# 2. Environment Variables & Context
$env:GSC_CHAT_ID = "{{GSC_CHAT_ID}}"
$env:GSC_PROJECT_ROOT = "{{GSC_PROJECT_ROOT}}"
$env:GSC_CONTRACT_UUID = "{{GSC_CONTRACT_UUID}}"
$env:GSC_SCRIPTS_DIR = "{{GSC_SCRIPTS_DIR}}"

# The 'p' variable: Dead simple access to your project root.
$p = "{{GSC_PROJECT_ROOT}}"

# 3. Functions
function .save { gsc ws save @args }
function .undo { gsc ws undo @args }
function .diff { gsc ws diff @args }
function .send { gsc ws send @args }
function .help { Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome" }

# 4. Custom Prompt
# Wrap the existing prompt to prepend (gsc-ws)
$oldPrompt = $function:prompt
function prompt {
    Write-Host "(gsc-ws) " -NoNewline -ForegroundColor Cyan
    if ($oldPrompt) {
        & $oldPrompt
    } else {
        Write-Host "$($PWD.Path)>" -NoNewline
        return " "
    }
}

# 5. Initialization
Clear-Host
Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome"
Set-Location "{{TARGET_DIR}}"
