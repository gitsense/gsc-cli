<#
Component: GitSense Workspace Shell Init (PowerShell)
Block-UUID: e9221d02-34cb-4902-a586-3cb7ce6aa36d
Parent-UUID: a60220f9-0902-4afe-b6db-e99d99ba0639
Version: 1.10.0
Description: Updated .switch to use 'cd' instead of 'gsc ws' to preserve the current shell session.
Language: PowerShell
Created-at: 2026-03-08T16:30:23.301Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0)
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
$env:GSC_CONTRACT_MAPPED_ROOT = "{{GSC_CONTRACT_MAPPED_ROOT}}"
$env:GSC_SCRIPTS_DIR = "{{GSC_SCRIPTS_DIR}}"

# The 'p' variable: Dead simple access to your project root.
$p = "{{GSC_PROJECT_ROOT}}"

# 3. Functions
function .ffp { gsc ws ffp @args }
function .send { gsc ws send @args }
function .help { Get-Content "$env:GSC_SCRIPTS_DIR/.gsc-welcome" }
function .map { gsc ws map @args }

function .block {
    $target = gsc ws block $args
    if (Test-Path $target -PathType Container) {
        Set-Location $target
    } elseif ($target) {
        Write-Output $target
    }
}

function .goto {
    $selection = gsc ws map --list | fzf --header "Jump to Workspace Block:" --reverse --height 40%
    if ($selection) {
        # Extract the relative path (everything after the last ' | ')
        # Using -split with a regex that matches the literal delimiter
        $parts = $selection -split '\s\|\s'
        $rel_path = $parts[-1]
        # Prepend the mapped root to get the absolute path
        $target = Join-Path $env:GSC_CONTRACT_MAPPED_ROOT $rel_path
        if (Test-Path $target -PathType Container) {
            Set-Location $target
        } else {
            Write-Error "Target directory does not exist: $target"
        }
    }
}

function .switch {
    $selection = Get-ChildItem $env:GSC_CONTRACT_MAPPED_ROOT -Directory | Select-Object -ExpandProperty Name | fzf --header "Switch Workspace:" --reverse --height 40%
    if ($selection) {
        Set-Location "$env:GSC_CONTRACT_MAPPED_ROOT\$selection"
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
