<#
Component: GitSense Workspace Shell Init
Block-UUID: ab9ad6b8-390c-4fa6-819b-cea2d12985f0
Parent-UUID: d2b179cc-120f-428d-921e-8b945893c427
Version: 1.2.0
Description: Updated to use GSC_MAPPED_WS_ROOT for absolute path resolution and renamed GSC_WS_HASH to GSC_MAPPED_WS_HASH.
Language: PowerShell
Created-at: 2026-03-06T05:20:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0)
#>


# GitSense Workspace Shell Init
# Generated for Workspace: {{GSC_MAPPED_WS_HASH}}

# 1. Environment Variables
$env:GSC_CHAT_ID = "{{GSC_CHAT_ID}}"
$env:GSC_MAPPED_WS_HASH = "{{GSC_MAPPED_WS_HASH}}"
$env:GSC_PROJECT_ROOT = "{{GSC_PROJECT_ROOT}}"
$env:GSC_CONTRACT_UUID = "{{GSC_CONTRACT_UUID}}"
$env:GSC_MAPPED_WS_ROOT = "{{GSC_MAPPED_WS_ROOT}}"

# 2. Functions (PowerShell requires functions for commands with arguments)
function save { gsc ws save @args }
function undo { gsc ws undo @args }
function diff { gsc ws diff @args }
function help { Get-Content "$env:GSC_MAPPED_WS_ROOT/.gsc-welcome" }

# 3. Custom Prompt
function prompt {
    Write-Host "(gsc-ws) " -NoNewline -ForegroundColor Cyan
    Write-Host "$($PWD.Path)" -NoNewline
    Write-Host ">"
    return " "
}

# 4. Welcome Message
Clear-Host
Get-Content "$env:GSC_MAPPED_WS_ROOT/.gsc-welcome"

# 5. Navigate to Target Directory
Set-Location "{{TARGET_DIR}}"
