# Scripts

Index of script groups adapted for this Go repo.

| Script / Dir | Description | How to run |
|--------------|-------------|------------|
| **pre-commit.sh** | Go format + vet check (fails if code not formatted or vet fails) | `./scripts/pre-commit.sh` (from repo root) |
| **fix-lint.sh** | Fix formatting and optional golangci-lint --fix | `./scripts/fix-lint.sh` |
| **mem-watch.sh** | Watch process RSS in GB (Linux/macOS) | `./scripts/mem-watch.sh <PID>` |
| **mem-watch.ps1** | Same on Windows (PowerShell) | `.\scripts\mem-watch.ps1 -PID <pid>` |
| **dtmf/** | Generate DTMF WAV files (ffmpeg or Go) | `./scripts/dtmf/generate_dtmf.sh` or `go run ./cmd/generate-dtmf [out_dir]` |
| **evals/** | Go-native eval runner (LLM scenarios) | `go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json` |
| **daily/** | Tavus/Daily transport tests (upstream only) | See [scripts/daily/README.md](daily/README.md) |
| **krisp/** | Krisp Viva (upstream only) | See [scripts/krisp/README.md](krisp/README.md) |

## Makefile

From repo root, `make lint` runs pre-commit checks, `make lint-fix` runs fix-lint, and `make evals` runs the eval runner with default config.
