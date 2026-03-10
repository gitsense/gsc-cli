<#
Component: GitSense Workspace Shell Init (PowerShell)
Block-UUID: 07a7409e-9f64-41c3-ac2f-98352ce9daf0
Parent-UUID: 82ac45f9-8d9b-4d8c-95ea-29e1323f37e1
Version: 1.8.0
Description: Updated .goto to prepend GSC_CONTRACT_MAPPED_ROOT to the relative path returned by 'gsc ws map --list'.
Language: PowerShell
Created-at: 2026-03-08T16:30:23.301Z
Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0)
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
