<#
Component: GitSense Workspace Shell Init (PowerShell)
Block-UUID: 0ea827ce-f74f-4419-8c7c-5ebec683f128
Parent-UUID: 2c7d2211-356b-45c2-8d46-18d063d7af8f
Version: 1.6.0
Description: Added .block shell function to enable workspace navigation.
Language: PowerShell
Created-at: 2026-03-08T16:30:23.301Z
Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
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
function .ffp { gsc ws ffp @args }
function .send { gsc ws send @args }
function .help { Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome" }

function .block {
    $target = gsc ws block $args
    if (Test-Path $target -PathType Container) {
        Set-Location $target
    } elseif ($target) {
        Write-Output $target
    }
}

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
