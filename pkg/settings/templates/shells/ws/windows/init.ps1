<#
Component: GitSense Workspace Shell Init
Block-UUID: d2b179cc-120f-428d-921e-8b945893c427
Parent-UUID: N/A
Version: 1.1.0
Description: PowerShell initialization script for shadow workspaces on Windows. Sets environment variables, functions, displays the welcome screen, and navigates to the target directory.
Language: PowerShell
Created-at: 2026-03-06T05:20:00.000Z
Authors: GLM-4.7 (v1.0.0)
#>


# GitSense Workspace Shell Init
# Generated for Workspace: {{GSC_WS_HASH}}

# 1. Environment Variables
$env:GSC_CHAT_ID = "{{GSC_CHAT_ID}}"
$env:GSC_WS_HASH = "{{GSC_WS_HASH}}"
$env:GSC_PROJECT_ROOT = "{{GSC_PROJECT_ROOT}}"
$env:GSC_CONTRACT_UUID = "{{GSC_CONTRACT_UUID}}"

# 2. Functions (PowerShell requires functions for commands with arguments)
function save { gsc ws save @args }
function undo { gsc ws undo @args }
function diff { gsc ws diff @args }
function help { Get-Content .gsc-welcome }

# 3. Custom Prompt
function prompt {
    Write-Host "(gsc-ws) " -NoNewline -ForegroundColor Cyan
    Write-Host "$($PWD.Path)" -NoNewline
    Write-Host ">"
    return " "
}

# 4. Welcome Message
Clear-Host
Get-Content .gsc-welcome

# 5. Navigate to Target Directory
Set-Location "{{TARGET_DIR}}"
