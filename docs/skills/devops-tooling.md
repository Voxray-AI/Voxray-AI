## 4. DevOps & Tooling

This section describes how Voxray is **built, tested, linted, monitored, and released**, and what skills are needed to work comfortably with its operational tooling.

---

### 4.1 CI/CD & Release Flow

- **GitHub Actions CI — Beginner/Intermediate**
  - **File**: `.github/workflows/go.yml`.
  - **Behavior**:
    - Triggers on pushes and pull requests to `main`.
    - Uses `actions/setup-go` to install Go `1.20`, then runs `go build ./...` and `go test ./...`.
  - **Notes**:
    - CI is intentionally simple: no matrix, coverage reporting, or lint step wired yet.
    - Release / deployment steps (Docker build/push, Helm, Terraform) are left to downstream infrastructure; there is no in‑repo CD pipeline.
  - **Skill implication**:
    - Basic GitHub Actions knowledge to extend workflows (e.g. adding lint, race detector, stress tests, or Docker build jobs).

---

### 4.2 Build & Run Tooling

- **Makefile — Beginner**
  - **File**: `Makefile`.
  - **Targets**:
    - `build` / `build-voice`: compile the server (`./cmd/voxray`), with `build-voice` enabling CGO for Opus/WebRTC TTS.
    - `run` / `run-voice`: run the server directly (voice version uses CGO).
    - `test`: `go test ./...` across all packages.
    - `tidy`: `go mod tidy`.
    - `proto`: regenerate protobufs for frame wire format.
    - `swagger`: regenerate swagger docs for `/webrtc/offer`.
    - `lint` / `lint-fix`: run linting scripts (see below).
    - `evals`: run the eval runner against `scripts/evals/config/scenarios.json`.
  - **Skill implication**:
    - Familiarity with standard Go build and test commands; the Makefile abstracts them but does not hide any complexity.

- **Scripts — Beginner/Intermediate**
  - **File**: `scripts/README.md`.
  - **Key scripts**:
    - `pre-commit.sh`: gofmt + go vet; used by `make lint`.
    - `fix-lint.sh`: auto‑formatting and optional `golangci-lint --fix`; used by `make lint-fix`.
    - `mem-watch.sh` / `mem-watch.ps1`: monitor process memory (RSS) while running stress tests or live services.
    - `dtmf/` scripts and `cmd/generate-dtmf`: generate DTMF WAV fixtures for tests or IVR scenarios.
    - `evals/`: drive the Go‑native eval runner (`cmd/evals`).
  - **Skill implication**:
    - Ability to interpret and tweak simple shell/PowerShell scripts when adding new checks or debugging build issues.

---

### 4.3 Testing Strategy & Frameworks

- **Unit tests (Go `testing` + `testify`) — Intermediate**
  - **File**: `tests/README.md`.
  - **Layout**:
    - External unit tests under `tests/pkg/**` (packages named `<pkg>_test`) that import `voxray-go/pkg/...` and exercise exported APIs.
    - A few internal tests remain in `pkg/**` when unexported symbols must be exercised (e.g. VAD/turn logic, STT/LLM provider specifics).
    - Build/smoke tests under `tests/cmd/**`, `tests/examples/**`, `tests/docs/**` ensure entrypoints and docs compile.
  - **Frameworks**:
    - Standard `testing` package.
    - `github.com/stretchr/testify` for assertions and helpers, especially in integration or more complex unit tests.
  - **Skill implication**:
    - Comfort writing table‑driven tests and using `testify` to keep assertions readable.

- **Integration & e2e tests — Intermediate**
  - **Files**: `tests/integration/**`, `tests/e2e/**`.
  - **Role**:
    - Verify interactions across packages (e.g. `pipeline` + `processors` + `frames`) and, in some cases, CLI/service‑level behavior (e.g. wrapping `cmd/realtime-demo`).
  - **Skill implication**:
    - Ability to spin up temporary servers with `StartServersWithListener`, send frames over WebSocket, and assert on end‑to‑end behavior.

- **Stress tests — Intermediate/Advanced**
  - **Files**: `tests/stress_testing/**`, `tests/README.md` (Stress tests and CI section).
  - **Behavior**:
    - Stress harness drives HTTP and pipeline load with configurable concurrency, duration, and SLOs.
    - Tests such as `TestHTTPStress_MockOfferEndpoint`, `TestMockPipeline_Stress`, `TestStressHarness_Realistic`, and `TestMockPipeline_NoGoroutineLeak` simulate real‑world load and assert on:
      - Minimum success rate.
      - Maximum P95 latency.
      - Minimum sessions per second.
    - Skipped by default under `go test -short`, so CI can use `go test -short ./...` for fast runs.
  - **Skill implication**:
    - Understanding of performance testing: how to design load profiles, interpret P95 latency, and avoid goroutine leaks.
    - Familiarity with Go’s race detector and profiling tools (pprof) is helpful when extending stress coverage.

- **Frontend/WebRTC tests — Beginner/Intermediate**
  - **Files**: `tests/frontend/README.md`, `tests/frontend/webrtc-voice.html`.
  - **Role**:
    - Provide a minimal, no‑framework WebRTC client for exercising the SmallWebRTC transport and Sarvam+Groq pipeline.
    - Used manually rather than as part of automated CI.
  - **Skill implication**:
    - Basic HTML/JS and browser devtools usage for debugging signaling and media issues.

---

### 4.4 Evals & Quality Gates

- **Go‑native LLM eval runner — Intermediate**
  - **Files**: `cmd/evals`, `scripts/evals/README.md`, `scripts/evals/config/scenarios.json`.
  - **Behavior**:
    - Runs a single‑pipeline (LLM‑only) eval: injects a prompt (as if it were a `TranscriptionFrame`), collects the LLM response, and asserts via substring or regex.
    - Uses the same `config.json` and `api_keys` as the main server, ensuring provider/model parity.
    - Writes structured JSON results and summary output; exits non‑zero if any scenario fails.
  - **Skill implication**:
    - Ability to design meaningful, maintainable scenarios that catch regressions across providers and models.
    - Understanding that evals cover **LLM behavior only**; STT/TTS and full voice flows require separate testing strategies (stress tests, manual WebRTC tests).

---

### 4.5 Linting, Formatting & Code Quality

- **gofmt + go vet — Beginner**
  - **Scripts**: `scripts/pre-commit.sh`, `Makefile` target `lint`.
  - **Behavior**:
    - Ensures code is formatted and passes `go vet` before commits.
  - **Skill implication**:
    - Straightforward: contributors should run `make lint` or configure pre‑commit hooks locally.

- **golangci‑lint (optional) — Intermediate**
  - **Script**: `scripts/fix-lint.sh`, `Makefile` target `lint-fix`.
  - **Behavior**:
    - Optionally runs `golangci-lint --fix` to auto‑correct common issues; falls back gracefully if the binary is not installed.
  - **Skill implication**:
    - Understanding of common static analysis warnings and how to address or suppress them appropriately.

---

### 4.6 Observability Tooling

- **Prometheus monitoring — Intermediate**
  - **Files**: `pkg/metrics/prom.go`, `pkg/server/server.go` (`/metrics` handler), `docs/DEPLOYMENT.md`.
  - **Behavior**:
    - Exposes a Prometheus text endpoint at `/metrics` on the same host/port as `/ws` and `/webrtc/offer`.
    - When `metrics_enabled=false`, still serves `/metrics` but with `204 No Content` so scrape configs remain valid.
    - **Session capacity** is observable via `active_sessions` (gauge) and `sessions_rejected_total` (counter by `reason`); useful for capacity planning and alerting when rejections spike.
  - **Skill implication**:
    - Ability to hook metrics into dashboards and alerts (Prometheus, Grafana), and to add new metrics while managing label cardinality.

- **Logging — Beginner/Intermediate**
  - **Files**: `pkg/logger`, pervasive usage in `cmd/voxray`, `pkg/server`, `pkg/pipeline`, and service adapters.
  - **Behavior**:
    - Supports JSON logs and configurable log levels; typically integrated with external log aggregation (not configured in‑repo).
  - **Skill implication**:
    - Writing logs that are structured and actionable in a real‑time, multi‑provider environment.

---

### 4.7 Onboarding Guidance (DevOps & Tooling)

- **Complexity rating: Medium**
  - The tooling itself is conventional (Go modules, Makefile, simple CI, Prometheus metrics), but there are **multiple layers of tests** (unit, integration, stress, evals) and several entrypoints/scripts that new contributors must learn.
  - A mid‑level engineer should expect to spend **1–2 days** becoming comfortable with the full testing and observability toolchain, especially if they need to extend stress tests or CI workflows. 

