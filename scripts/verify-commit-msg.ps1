<#
.SYNOPSIS
    Validates Git commit messages against Conventional Commits 1.0.0.
#>
param(
    [Parameter(Mandatory = $true)]
    [string]$CommitMsgFile
)

if (-not (Test-Path $CommitMsgFile)) {
    Write-Error "Commit message file not found: $CommitMsgFile"
    exit 1
}

$lines = Get-Content -Path $CommitMsgFile -Encoding UTF8 | Where-Object { $_ -notmatch '^\s*#' }
$header = ($lines | Select-Object -First 1)

if ([string]::IsNullOrWhiteSpace($header)) {
    Write-Host "ERROR: Commit message header cannot be empty." -ForegroundColor Red
    exit 1
}

# Allow standard Git merge/revert auto-headers
if ($header -match '^(Merge |Revert ")') {
    exit 0
}

$pattern = '^(feat|fix|docs|refactor|test|perf|style|build|ci|chore|revert)(\([a-z0-9-]+\))?(!)?:\s[a-z0-9].{2,99}$'

if ($header -notmatch $pattern) {
    Write-Host "ERROR: Commit message does not follow Conventional Commits 1.0.0:" -ForegroundColor Red
    Write-Host "  Received: $header" -ForegroundColor Yellow
    Write-Host "  Expected: <type>(<scope>): <short imperative English summary>" -ForegroundColor Cyan
    Write-Host "  Types:    feat, fix, docs, refactor, test, perf, style, build, ci, chore, revert" -ForegroundColor Cyan
    Write-Host "  Example:  feat(vault): encrypt imported PEM keys with AES-256-GCM" -ForegroundColor Green
    exit 1
}

if ($header.EndsWith('.')) {
    Write-Host "ERROR: Commit summary must not end with a period." -ForegroundColor Red
    exit 1
}

exit 0
