#!/usr/bin/env pwsh
#requires -Version 5.1

<#
.SYNOPSIS
    Build a Windows air-gapped package for the SECSIM design scaffold.

.DESCRIPTION
    This script builds the Vite frontend, cross-compiles the Go backend as a
    Windows executable, and assembles a self-contained package that can run
    without access to the source tree.

.PARAMETER OutputDir
    Directory where build artifacts will be written. Default: ./dist

.PARAMETER SkipFrontend
    Skip the frontend build and use the existing frontend/dist folder.

.PARAMETER SkipDeps
    Skip npm/go dependency download steps.

.PARAMETER Architecture
    Target Windows architecture: amd64, arm64, or 386.

.PARAMETER Compress
    Create a .zip archive for the assembled package.

.PARAMETER Port
    Default HTTP port used by the generated start.bat file.
#>

[CmdletBinding()]
param(
    [string]$OutputDir = "./dist",
    [switch]$SkipFrontend,
    [switch]$SkipDeps,
    [ValidateSet("amd64", "arm64", "386")]
    [string]$Architecture = "amd64",
    [switch]$Compress,
    [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

$Colors = @{
    Success = "Green"
    Info    = "Cyan"
    Warning = "Yellow"
    Error   = "Red"
}

function Write-Status {
    param(
        [string]$Message,
        [ValidateSet("Success", "Info", "Warning", "Error")]
        [string]$Type = "Info"
    )

    Write-Host "[$Type] $Message" -ForegroundColor $Colors[$Type]
}

function Test-Command {
    param([string]$Command)

    return [bool](Get-Command -Name $Command -ErrorAction SilentlyContinue)
}

function Resolve-OrCreateDirectory {
    param([string]$BaseDir, [string]$Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        $candidate = $Path
    } else {
        $candidate = Join-Path $BaseDir $Path
    }

    if (-not (Test-Path $candidate)) {
        New-Item -ItemType Directory -Path $candidate -Force | Out-Null
    }

    return (Resolve-Path $candidate).Path
}

function Assert-ExitCode {
    param([string]$Action)

    if ($LASTEXITCODE -ne 0) {
        Write-Status "$Action failed" "Error"
        exit 1
    }
}

Write-Status "Starting SECSIM air-gapped build" "Info"
Write-Status "Target architecture: $Architecture" "Info"
Write-Status "Default port: $Port" "Info"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$frontendDir = Join-Path $scriptDir "frontend"
$backendDir = Join-Path $scriptDir "backend"
$frontendDistDir = Join-Path $frontendDir "dist"
$outputRoot = Resolve-OrCreateDirectory -BaseDir $scriptDir -Path $OutputDir

Write-Status "Checking prerequisites" "Info"
foreach ($command in @("node", "npm", "go")) {
    if (-not (Test-Command $command)) {
        Write-Status "$command is not installed or not in PATH" "Error"
        exit 1
    }
}

Write-Status "Node.js version: $(node --version)" "Success"
Write-Status "Go version: $(go version)" "Success"

if (-not $SkipFrontend) {
    Write-Status "Building frontend" "Info"
    Push-Location $frontendDir
    try {
        if (-not $SkipDeps) {
            if (-not (Test-Path "node_modules")) {
                Write-Status "Installing npm dependencies" "Info"
                npm ci --prefer-offline --no-audit --progress=false
                Assert-ExitCode "npm ci"
            } else {
                Write-Status "node_modules already present, skipping npm install" "Info"
            }
        }

        npm run build
        Assert-ExitCode "frontend build"
    } finally {
        Pop-Location
    }
} else {
    Write-Status "Skipping frontend build" "Warning"
}

if (-not (Test-Path $frontendDistDir)) {
    Write-Status "Frontend dist directory not found at $frontendDistDir" "Error"
    Write-Host "Run without -SkipFrontend to build the frontend first." -ForegroundColor Yellow
    exit 1
}

Write-Status "Frontend assets ready" "Success"

Write-Status "Building Go backend" "Info"
$timestamp = Get-Date -Format "yyyyMMdd"
$binaryName = "secsim_windows_${Architecture}_${timestamp}.exe"
$binaryPath = Join-Path $outputRoot $binaryName

Push-Location $backendDir
try {
    if (-not $SkipDeps -and (Test-Path "go.mod")) {
        $hasVendor = Test-Path "vendor"
        if (-not $hasVendor) {
            Write-Status "Downloading Go dependencies" "Info"
            go mod download
            Assert-ExitCode "go mod download"
        } else {
            Write-Status "vendor directory present, skipping go mod download" "Info"
        }
    }

    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = $Architecture
    $ldflags = "-s -w"
    $buildArgs = @("build", "-trimpath", "-ldflags", $ldflags, "-o", $binaryPath, "./cmd/secsimd")

    if (Test-Path "vendor") {
        $buildArgs = @("build", "-mod=vendor", "-trimpath", "-ldflags", $ldflags, "-o", $binaryPath, "./cmd/secsimd")
    }

    & go @buildArgs
    Assert-ExitCode "go build"
} finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}

if (-not (Test-Path $binaryPath)) {
    Write-Status "Backend binary was not created at $binaryPath" "Error"
    exit 1
}

$binaryInfo = Get-Item $binaryPath
$binarySizeMb = [math]::Round($binaryInfo.Length / 1MB, 2)
Write-Status "Built $binaryName ($binarySizeMb MB)" "Success"

Write-Status "Assembling package" "Info"
$packageName = "secsim-airgapped-${timestamp}"
$packageDir = Join-Path $outputRoot $packageName
if (Test-Path $packageDir) {
    Remove-Item -Recurse -Force $packageDir
}

$webDistTarget = Join-Path $packageDir "web\dist"
New-Item -ItemType Directory -Path $webDistTarget -Force | Out-Null

$packageBinary = Join-Path $packageDir "secsim.exe"
Copy-Item $binaryPath $packageBinary -Force
Copy-Item -Path (Join-Path $frontendDistDir "*") -Destination $webDistTarget -Recurse -Force

$startScript = @"
@echo off
setlocal
cd /d "%~dp0"
if "%SECSIM_ADDR%"=="" set SECSIM_ADDR=:$Port
title SECSIM
echo.
echo Starting SECSIM on http://localhost:$Port
echo.
secssim.exe
endlocal
"@
$startScript | Out-File -FilePath (Join-Path $packageDir "start.bat") -Encoding ASCII

$readme = @"
SECSIM - Air-Gapped Package
==========================

Quick start
-----------
1. Double-click start.bat
2. Open http://localhost:$Port

Contents
--------
- secsim.exe        - Go backend
- web\dist\         - built frontend assets
- start.bat         - launcher script

Runtime knobs
-------------
- SECSIM_ADDR       - override bind address and port (default :$Port)
- SECSIM_WEB_DIST   - override frontend asset directory

Notes
-----
This package does not require the source tree. The executable looks for
frontend assets in web\dist next to secsim.exe unless SECSIM_WEB_DIST is set.
"@
$readme | Out-File -FilePath (Join-Path $packageDir "README.txt") -Encoding ASCII

if ($Compress) {
    $zipPath = Join-Path $outputRoot "$packageName.zip"
    if (Test-Path $zipPath) {
        Remove-Item -Force $zipPath
    }
    Compress-Archive -Path "$packageDir\*" -DestinationPath $zipPath -Force
    Write-Status "Created archive: $zipPath" "Success"
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  BUILD COMPLETED SUCCESSFULLY" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Binary:  $binaryPath" -ForegroundColor White
Write-Host "Package: $packageDir" -ForegroundColor White
Write-Host ""
Write-Host "To run on Windows:" -ForegroundColor Cyan
Write-Host "  1. Open the package folder" -ForegroundColor Yellow
Write-Host "  2. Run start.bat" -ForegroundColor Yellow
Write-Host ""
