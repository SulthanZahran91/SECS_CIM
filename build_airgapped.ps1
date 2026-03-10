#!/usr/bin/env pwsh
#requires -Version 5.1

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$designScript = Join-Path $scriptDir "design/build-airgapped.ps1"

if (-not (Test-Path $designScript)) {
    Write-Error "Expected design/build-airgapped.ps1 at $designScript"
    exit 1
}

& $designScript @args
exit $LASTEXITCODE
