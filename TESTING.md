### Build and test commands

- **Build the main binary**
  - `make build` &rarr; runs `go build -o voxray ./cmd/voxray`
- **Run the main app**
  - `make run` &rarr; runs `go run ./cmd/voxray`
- **Run the full test suite (all features)**
  - `make test` or `go test ./...`

### Test scopes

- **All tests for all features**
  - `go test ./...` (or `make test`)
- **Only core package/unit tests under `tests/pkg`**
  - `go test ./tests/pkg/...`
- **Only integration tests under `tests/integration`**
  - `go test ./tests/integration/...`
- **Only e2e tests under `tests/e2e`** (once real e2e tests are added)
  - `go test ./tests/e2e/...`

Sarvam-specific tests under `tests/pkg/services` are skipped automatically unless
`SARVAM_API_KEY` is set in the environment. They exercise wiring Sarvam as the
`provider`/`stt_provider`/`tts_provider` in `config.Config` and run a small
smoke test against the live API.

### How tests are discovered

- **Centralized package/unit tests**
  - Add `*_test.go` files under `tests/pkg/**`, mirroring the structure of `pkg/**` (e.g. `tests/pkg/pipeline/pipeline_test.go` for `pkg/pipeline`).
  - Tests are written as external packages (e.g. `package pipeline_test`) that import the code under test via `voxray-go/pkg/...`.
  - `go test ./...` automatically finds and runs these tests because `tests` is part of the module tree.
- **Top-level `tests/` folder**
  - Integration tests live under `tests/integration/`.
  - Future e2e/CLI-style tests live under `tests/e2e/`.
  - Shared fixtures can be placed in `tests/testdata/`.
  - Because `tests` is part of the module tree, `go test ./...` also runs tests in these packages.

### Suggested Makefile extensions (conceptual)

For more focused workflows you can add the following targets to the `Makefile` in the future:

- `test-unit` &rarr; `go test ./pkg/...`
- `test-integration` &rarr; `go test ./tests/integration/...`
- `test-e2e` &rarr; `go test ./tests/e2e/...`

The existing `make test` target should remain the aggregate that runs everything.

### CI workflow (conceptual example)

In a GitHub Actions setup you can run all tests on each push/PR with a workflow like:

```yaml
name: CI

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go mod tidy
      - run: go test ./...
```

This guarantees that all unit, integration, and e2e tests under `cmd/**`, `pkg/**`, and `tests/**` are executed in CI.

### Recommended developer workflow

- **Day-to-day development**
  - After implementing or changing a feature, run `make test` to exercise all tests.
- **Focused debugging**
  - Narrow scope with commands like `go test ./pkg/somepackage/...` or `go test ./tests/integration/...`.
- **Before committing or opening a PR**
  - Run `go test ./...` (or `make test`) to ensure all implemented feature tests pass.

