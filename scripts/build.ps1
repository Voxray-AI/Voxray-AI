# Build voxray binary
Set-Location $PSScriptRoot\..
go build -o voxray ./cmd/voxray
