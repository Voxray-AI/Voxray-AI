# Build voila binary
Set-Location $PSScriptRoot\..
go build -o voila ./cmd/voila
