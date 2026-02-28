# Build voila with CGO enabled so the Opus encoder is included (required for WebRTC TTS).
# Requires: gcc on PATH. Run from repo root: .\scripts\build-voice.ps1
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
go build -o voila.exe ./cmd/voila
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built voila.exe with Opus (CGO). Run: .\voila.exe -config config.json"
