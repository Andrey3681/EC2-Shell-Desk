<#
.SYNOPSIS
    Verifies required and optional Windows development tools for EC2 Shell Desk.
#>

$ErrorActionPreference = 'Continue'

function Test-Tool {
    param(
        [string]$Name,
        [string]$Command,
        [string[]]$VersionArgs,
        [bool]$Required = $true
    )

    $cmdInfo = Get-Command $Command -ErrorAction SilentlyContinue
    if ($null -eq $cmdInfo) {
        $level = if ($Required) { '[MISSING - REQUIRED]' } else { '[MISSING - OPTIONAL]' }
        Write-Host "$level $Name ($Command)" -ForegroundColor $(if ($Required) { 'Red' } else { 'Yellow' })
        return (-not $Required)
    }

    try {
        $output = & $Command @VersionArgs 2>&1 | Select-Object -First 1
        Write-Host "[OK] $Name -> $output" -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host "[WARN] $Name found at $($cmdInfo.Source) but version check failed." -ForegroundColor Yellow
        return $true
    }
}

Write-Host "=== EC2 Shell Desk - Windows Toolchain Doctor ===" -ForegroundColor Cyan

$allOk = $true
$allOk = (Test-Tool -Name 'Git' -Command 'git' -VersionArgs @('--version') -Required $true) -and $allOk
$allOk = (Test-Tool -Name 'Go' -Command 'go' -VersionArgs @('version') -Required $true) -and $allOk
$allOk = (Test-Tool -Name 'Node.js' -Command 'node' -VersionArgs @('-v') -Required $true) -and $allOk
$allOk = (Test-Tool -Name 'npm' -Command 'npm' -VersionArgs @('-v') -Required $true) -and $allOk
$allOk = (Test-Tool -Name 'Wails CLI v2' -Command 'wails' -VersionArgs @('version') -Required $true) -and $allOk
$allOk = (Test-Tool -Name 'Task' -Command 'task' -VersionArgs @('--version') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'gofumpt' -Command 'gofumpt' -VersionArgs @('--version') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'golangci-lint' -Command 'golangci-lint' -VersionArgs @('--version') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'gomarkdoc' -Command 'gomarkdoc' -VersionArgs @('--version') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'Lefthook' -Command 'lefthook' -VersionArgs @('version') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'NSIS (makensis)' -Command 'makensis' -VersionArgs @('/VERSION') -Required $false) -and $allOk
$allOk = (Test-Tool -Name 'PlantUML' -Command 'plantuml' -VersionArgs @('-version') -Required $false) -and $allOk

# Check Microsoft Edge WebView2 Runtime via Windows Registry
$webview2Keys = @(
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
    'HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
    'HKCU:\Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
)
$webviewVersion = $null
foreach ($key in $webview2Keys) {
    if (Test-Path $key) {
        $props = Get-ItemProperty -Path $key -ErrorAction SilentlyContinue
        if ($props.pv) {
            $webviewVersion = $props.pv
            break
        }
    }
}

if ($webviewVersion) {
    Write-Host "[OK] WebView2 Runtime -> $webviewVersion" -ForegroundColor Green
}
else {
    Write-Host "[WARN] WebView2 Runtime not detected in standard registry paths." -ForegroundColor Yellow
}

if (-not $allOk) {
    Write-Host "`nSome required tools are missing. Run 'task setup' or install them before building." -ForegroundColor Red
    exit 1
}

Write-Host "`nAll required tools are ready." -ForegroundColor Green
exit 0
