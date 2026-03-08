<#
Component: GitSense Workspace Shell Init
Block-UUID: cea2a8f7-0f7a-442b-a7ff-6aa2698fc9e9
Parent-UUID: ab9ad6b8-390c-4fa6-819b-cea2d12985f0
Version: 1.3.0
Description: Moved init scripts to parent mapped directory, updated aliases to use dot prefix (e.g., .save), and replaced GSC_MAPPED_WS_ROOT with GSC_SCRIPTS_DIR.
Language: PowerShell
Created-at: 2026-03-06T05:20:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
#>


# GitSense Workspace Shell Init

# 1. Environment Variables
$env:GSC_CHAT_ID = "{{GSC_CHAT_ID}}"
$env:GSC_PROJECT_ROOT = "{{GSC_PROJECT_ROOT}}"
$env:GSC_CONTRACT_UUID = "{{GSC_CONTRACT_UUID}}"
$env:GSC_SCRIPTS_DIR = "{{GSC_SCRIPTS_DIR}}"

# 2. Functions (PowerShell requires functions for commands with arguments)
function .save { gsc ws save @args }
function .undo { gsc ws undo @args }
function .diff { gsc ws diff @args }
function .send { gsc ws send @args }
function .help { Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome" }

# 3. Custom Prompt
function prompt {
    Write-Host "(gsc-ws) " -NoNewline -ForegroundColor Cyan
    Write-Host "$($PWD.Path)" -NoNewline
    Write-Host ">"
    return " "
}

# 4. Welcome Message
Clear-Host
Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome"

# 5. Navigate to Target Directory
Set-Location "{{TARGET_DIR}}"
