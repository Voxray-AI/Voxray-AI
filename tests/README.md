### Test layout and conventions

- **Unit tests**
  - Co-located next to implementation files as `*_test.go` inside `pkg/**` and `cmd/**`.
  - Use Go's standard `testing` package plus `testify` for assertions and helpers.

- **Integration tests**
  - Live under `tests/integration/` as standalone Go packages.
  - Exercise interactions between multiple packages (e.g. `pipeline`, `processors`, `frames`).

- **End-to-end (e2e) tests**
  - Live under `tests/e2e/`.
  - Intended for CLI- or service-level flows (e.g. wrapping `cmd/realtime-demo` in a harness).

- **Test data**
  - Shared fixtures go under `tests/testdata/`.
  - Package-specific fixtures can also use local `testdata/` folders next to the code when they are not shared.

