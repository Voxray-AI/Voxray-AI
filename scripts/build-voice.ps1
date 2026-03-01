# Build voxray with CGO enabled so the Opus encoder is included (required for WebRTC TTS).
# Requires: gcc on PATH, or WinLibs installed via winget (script will try to find it).
# Run from repo root: .\scripts\build-voice.ps1
$ErrorActionPreference = "Stop"
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    # Try WinGet WinLibs location (winget install BrechtSanders.WinLibs.POSIX.UCRT)
    $wingetBin = Get-ChildItem -Path "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Directory -Filter "*WinLibs*" -Recurse -Depth 2 -ErrorAction SilentlyContinue |
        ForEach-Object { Join-Path $_.FullName "mingw64\bin" } |
        Where-Object { Test-Path (Join-Path $_ "gcc.exe") } |
        Select-Object -First 1
    if ($wingetBin) {
        $env:PATH = "$wingetBin;$env:PATH"
        Write-Host "Using gcc from: $wingetBin" -ForegroundColor Cyan
    }
}
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    Write-Host "gcc not found. CGO (and the Opus encoder for WebRTC TTS) requires a C compiler on PATH." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Install one of these, then restart your terminal (or add its bin folder to PATH):" -ForegroundColor Cyan
    Write-Host "  winget: winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements"
    Write-Host "  MSYS2:  https://www.msys2.org/ then in MSYS2 UCRT64: pacman -S mingw-w64-ucrt-x86_64-toolchain"
    Write-Host ""
    Write-Host "After install, verify with: gcc --version" -ForegroundColor Cyan
    exit 1
}
Set-Location $PSScriptRoot\..
$env:CGO_ENABLED = "1"
# Enable optimization for Opus C code (otherwise it's very slow) and suppress known harmless warnings
if (-not $env:CGO_CFLAGS) { $env:CGO_CFLAGS = "-O2 -Wno-stringop-overread" }
go build -o voxray.exe ./cmd/voxray
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built voxray.exe with Opus (CGO). Run: .\voxray.exe -config config.json"
