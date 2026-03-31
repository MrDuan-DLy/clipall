$ErrorActionPreference = "Stop"

$Repo = "MrDuan-DLy/clipall"
$Binary = "clipall.exe"
$InstallDir = "$env:LOCALAPPDATA\clipall"

Write-Host "==> Fetching latest release..." -ForegroundColor Cyan
$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Tag = $Release.tag_name
Write-Host "==> Latest release: $Tag" -ForegroundColor Cyan

# Check if already up to date.
$ExistingBin = Get-Command clipall -ErrorAction SilentlyContinue
if ($ExistingBin) {
    $Current = & clipall --version 2>$null
    if ($Current -eq "clipall $Tag") {
        Write-Host "==> Already up to date ($Tag)" -ForegroundColor Green
        exit 0
    }
    Write-Host "==> Updating: $Current -> $Tag" -ForegroundColor Cyan
} else {
    Write-Host "==> Installing: $Tag" -ForegroundColor Cyan
}

$Asset = "clipall-windows-amd64.exe"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

$OutPath = Join-Path $InstallDir $Binary
Write-Host "==> Downloading $Url..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $Url -OutFile $OutPath -UseBasicParsing

# Add to PATH if not already present.
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "==> Added $InstallDir to user PATH" -ForegroundColor Cyan
}

Write-Host "==> Installed clipall $Tag to $OutPath" -ForegroundColor Green
Write-Host ""
Write-Host "  Run: clipall --peers <hostname>:9876"
Write-Host ""
Write-Host "  Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
