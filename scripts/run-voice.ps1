# Run voila with CGO-enabled build so WebRTC TTS (Opus) works. Requires gcc on PATH.
# Usage: .\scripts\run-voice.ps1 -config config.json
$ErrorActionPreference = "Stop"
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
go run ./cmd/voila @args
