# Contributing to Voxray-AI

Voxray-AI is a production-grade streaming voice pipeline server in Go. It wires together:

- Client audio
- WebSocket / WebRTC / telephony transports
- STT (speech-to-text)
- LLM (language model)
- TTS (text-to-speech)

This document explains how to get a local environment running, how to run tests, how the project is structured, and how to extend Voxray-AI with new providers while matching the existing style and review process.

## Table of Contents

- [Prerequisites \& Local Setup](#prerequisites--local-setup)
- [Running Tests](#running-tests)
- [Project Structure](#project-structure)
- [Adding a New Provider](#adding-a-new-provider)
  - [Provider Interfaces (STT, LLM, TTS)](#provider-interfaces-stt-llm-tts)
  - [Provider Checklist](#provider-checklist)
- [Code Style \& Quality](#code-style--quality)
- [Commit Messages (Conventional Commits)](#commit-messages-conventional-commits)
- [Pull Request Process](#pull-request-process)
- [Bug Reports](#bug-reports)
- [Links](#links)

## Prerequisites & Local Setup

### Required tools

- Go **1.25+**
- Git
- `make` (optional but recommended)
- CGO toolchain for WebRTC builds:
  - A working C compiler (e.g. `gcc` or `clang`)
  - On Linux: appropriate `build-essential` packages
  - On macOS: Xcode Command Line Tools
- Docker / Docker Compose (optional, for running dependencies and integration tests)

### Clone the repository

```bash
git clone https://github.com/voxray-ai/voxray-ai.git
cd voxray-ai
```

### Go modules

Voxray-AI uses Go modules. Dependencies are tracked in `go.mod` / `go.sum`.

```bash
go mod tidy
```

### Environment configuration

Configuration is provided via JSON/YAML files and environment variables.

- Base config lives under `configs/` (for example runtime presets).
- Runtime selection is typically via an environment variable such as:

```bash
export VOXRAY_CONFIG=./configs/local.json
```

The config file controls:

- Enabled transports (WebSocket `/ws`, WebRTC `/webrtc/offer`, telephony runners)
- Provider selection and credentials (STT, LLM, TTS)
- Database connections (Postgres/MySQL)
- Redis session store
- S3 bucket and recording options
- TLS and CORS settings

Check the example configs in `configs/` and the main `README.md` for exact keys and values.

### Running the server locally

Run the main Voxray-AI binary:

```bash
go run ./cmd/voxray
```

Common flags and environment variables:

- `VOXRAY_CONFIG` – path to the JSON config used at startup.
- `VOXRAY_LOG_LEVEL` – log verbosity (`debug`, `info`, `warn`, `error`).

With the default local config:

- WebSocket endpoint: `ws://localhost:8080/ws`
- WebRTC offer endpoint: `http://localhost:8080/webrtc/offer`

### Optional: Docker-based setup

If you prefer to run dependencies via Docker (databases, Redis, etc.), use:

```bash
docker-compose up -d
```

Then run the Go server against those services:

```bash
VOXRAY_CONFIG=./configs/docker-local.json go run ./cmd/voxray
```

Refer to `docker-compose.yml` and `Dockerfile` for details on the containerized environment.

## Running Tests

Voxray-AI has both unit tests and heavier integration/e2e tests.

### Fast unit tests

```bash
go test ./...
```

This runs all packages (including tests under `pkg/` and most of `tests/`) and should be used before every commit.

To run tests in a specific package:

```bash
go test ./pkg/transport/...
go test ./pkg/services/...
```

### Integration and end-to-end tests

Heavier tests live under `tests/`, `tests/integration/`, and `tests/e2e/` and may depend on external services running via Docker.

Typical flow:

```bash
docker-compose up -d        # Postgres/MySQL, Redis, etc.

VOXRAY_CONFIG=./configs/test.json \
go test ./tests/... -run TestE2E
```

Consult `TESTING.md` and `tests/README.md` for details on which services are required for specific suites (STT/LLM/TTS providers, telephony runners, etc.).

When adding new tests:

- Prefer fast, deterministic unit tests in `pkg/...`.
- For provider integrations, add coverage under `tests/pkg/services/<provider>/...`.
- For full pipeline scenarios (WebSocket/WebRTC/telephony), add tests under `tests/e2e` or `tests/integration`.

## Project Structure

The layout below is a simplified view of the repository to help you discover where to make changes:

```text
.
├── cmd/
│   └── voxray/                 # Main Voxray-AI server entrypoint
├── pkg/
│   ├── audio/                  # Audio helpers, VAD (Silero), mixing, DTMF
│   ├── config/                 # Configuration loading and validation
│   ├── frames/                 # Wire formats and serialization helpers
│   ├── metrics/                # Prometheus metrics wiring
│   ├── observers/              # Observability (latency, transcripts, metrics)
│   ├── pipeline/               # Core streaming pipeline orchestration
│   ├── processors/             # Pipeline processors (STT, LLM, TTS, filters)
│   ├── recording/              # S3/local recording logic
│   ├── services/               # Provider clients (STT, LLM, TTS, etc.)
│   ├── transport/              # WebSocket, WebRTC, telephony, in-memory
│   ├── realtime/               # Realtime API integrations (e.g. OpenAI)
│   └── utils/                  # Shared utilities and helpers
├── tests/                      # Integration, e2e, and provider tests
├── docs/                       # Documentation, API docs, deployment, skills
├── examples/                   # Example apps and demos
├── scripts/                    # Helper scripts for builds, evals, tooling
├── configs/                    # Example JSON/YAML configuration files
├── Dockerfile                  # Production Docker image
├── docker-compose.yml          # Local/dev services (DB, Redis, etc.)
├── go.mod                      # Go module definition
└── README.md                   # Project overview and quickstart
```

When in doubt, follow existing patterns in the closest neighboring package. For example, new STT providers should align with other STT providers under `pkg/services`.

## Adding a New Provider

Providers are selected via JSON config and wired into the pipeline via registries in `pkg/services` and related packages. A provider can target:

- **STT** (speech-to-text)
- **LLM** (language model)
- **TTS** (text-to-speech)

New providers should live alongside existing ones and implement the relevant interface(s) described below.

### Provider Interfaces (STT, LLM, TTS)

The snippets below show simplified example interfaces. Check the concrete interfaces under `pkg/services` (and related packages) for the exact up-to-date signatures.

#### STT provider interface

```go
package stt

import "context"

type Options struct {
	Language       string
	SampleRateHz   int
	EnablePunctuation bool
	// ... other STT options ...
}

type Result struct {
	Text      string
	IsFinal   bool
	Confidence float32
}

type Stream interface {
	SendAudio(ctx context.Context, pcm []byte) error
	Recv(ctx context.Context) (*Result, error)
	CloseSend(ctx context.Context) error
}

type Provider interface {
	// ID must be stable and match the name used in config.
	ID() string

	// StartStream starts a streaming STT session.
	StartStream(ctx context.Context, opts Options) (Stream, error)
}
```

#### LLM provider interface

```go
package llm

import "context"

type Message struct {
	Role    string
	Content string
}

type Request struct {
	Model       string
	Messages    []Message
	Temperature float32
	// ... additional provider-agnostic options ...
}

type Response struct {
	Text      string
	FinishReason string
	// ... additional metadata ...
}

type TokenHandler func(token string) error

type Provider interface {
	ID() string

	// Chat performs a non-streaming chat completion.
	Chat(ctx context.Context, req *Request) (*Response, error)

	// ChatStream streams tokens back via the callback.
	ChatStream(ctx context.Context, req *Request, onToken TokenHandler) (*Response, error)
}
```

#### TTS provider interface

```go
package tts

import "context"

type Request struct {
	Text       string
	Voice      string
	Language   string
	SampleRate int
	// ... prosody, style, etc. ...
}

type AudioChunk struct {
	PCM       []byte
	SampleRate int
}

type ChunkHandler func(chunk AudioChunk) error

type Provider interface {
	ID() string

	// Synthesize performs a blocking synthesis, returning the full audio buffer.
	Synthesize(ctx context.Context, req *Request) ([]byte, error)

	// SynthesizeStream streams audio chunks via the callback.
	SynthesizeStream(ctx context.Context, req *Request, onChunk ChunkHandler) error
}
```

#### Wiring a new provider

At a high level, creating a new provider involves:

1. Creating a Go package under `pkg/services/<provider_name>/`.
2. Implementing the relevant interface(s) (`stt.Provider`, `llm.Provider`, `tts.Provider`).
3. Registering the provider with the central registry (for example in `pkg/services/stt`, `pkg/services/tts`, or a similar registry module).
4. Adding configuration struct(s) and parsing logic in `pkg/config` or the relevant config module.
5. Updating documentation and tests to cover the new provider.

Use existing providers (e.g. OpenAI, Groq, Google, AWS, Sarvam) as a reference for timeouts, error handling, logging, and metrics.

### Provider Checklist

Before opening a PR that adds or changes a provider, ensure that:

- **Configuration**
  - **No hardcoded API keys or secrets** – everything must come from config or environment variables.
  - Config fields are documented and validated (e.g. region, model, voice, language).
  - Reasonable defaults are provided where appropriate.
- **Implementation**
  - The provider implements the appropriate interface(s) and passes existing compile-time checks.
  - Timeouts and context cancellation are honored for all network calls.
  - Errors from the upstream provider are wrapped with enough context for debugging.
  - Logging uses the existing logging framework and avoids noisy logs in the hot path.
  - Prometheus metrics (latency, error counts, etc.) are recorded consistently with other providers.
- **Testing**
  - Unit tests cover basic success and error paths.
  - If feasible, add integration tests under `tests/pkg/services/<provider>/` or a relevant suite.
  - Tests are resilient to transient external failures (use mocking or recorded fixtures where appropriate).
- **Documentation**
  - Configuration options are documented in `README.md`, `docs/`, or provider-specific docs.
  - Any new environment variables are described (name, purpose, default behavior).
  - Example config snippets are updated to include the provider where relevant.
- **Registration**
  - The provider is registered in the appropriate registry so it can be selected via JSON config.
  - The provider name used in config is consistent across code, docs, and tests.

## Code Style & Quality

Voxray-AI follows standard Go best practices with a focus on correctness, observability, and safe concurrency.

### Formatting and linting

- **gofmt / goimports**: All Go code must be formatted.

```bash
gofmt -w ./cmd ./pkg ./tests
# or
goimports -w ./cmd ./pkg ./tests
```

- **go vet**: Run static analysis:

```bash
go vet ./...
```

- **golangci-lint** (preferred aggregated linter):

```bash
golangci-lint run ./...
```

CI will typically run a subset of these. Local runs before pushing are strongly recommended.

### Context propagation

- Every request-handling entrypoint should accept a `context.Context`.
- Pass the context through to downstream calls (providers, DB, Redis, etc.) instead of using `context.Background`.
- Respect context cancellation and deadlines in all I/O and long-running operations.

### Error handling

- Return errors instead of calling `panic` in normal control flow.
- Wrap errors with additional context when crossing package boundaries.
- Use typed or sentinel errors where it makes behavior clearer.
- Log errors once at the boundary of the pipeline; avoid duplicate logging of the same error.

### Panics

- Panics are reserved for truly unrecoverable programmer errors (e.g. impossible states).
- Public APIs and hot paths must not panic on invalid user input or transient provider errors.

### Concurrency

- Avoid shared mutable state where possible; prefer passing data through channels or pipeline stages.
- Protect shared state with mutexes when necessary and document assumptions.
- Keep goroutine lifecycles tied to a `context.Context`.

## Commit Messages (Conventional Commits)

This project uses the **Conventional Commits** convention to keep history clear and automation-friendly.

### Format

```text
<type>(<optional scope>): <short summary>
```

Common types:

- `feat` – new feature
- `fix` – bug fix
- `docs` – documentation only changes
- `refactor` – code change that neither fixes a bug nor adds a feature
- `test` – adding or updating tests
- `chore` – maintenance, tooling, or non-production code
- `ci` – CI-related changes
- `build` – build or dependency changes
- `perf` – performance improvements

### Examples

```text
feat(pipeline): add silero vad-based auto-mute
fix(transport): handle websocket close frames correctly
docs(providers): document groq tts configuration
refactor(services): unify stt/tts provider registry
test(e2e): cover webrtc offer path with groq + sarvam
chore(ci): enable golangci-lint in main workflow
```

## Pull Request Process

1. **Fork and branch**
   - Fork the repository (if you are an external contributor).
   - Create a branch with a descriptive name, for example:

   ```bash
   git checkout -b feat/webrtc-metrics
   ```

2. **Make focused changes**
   - Keep each PR logically focused (one feature or fix at a time).
   - Update or add tests alongside your changes.

3. **Run checks locally**
   - `go test ./...`
   - `golangci-lint run ./...` (if installed)
   - Any relevant integration or e2e tests for your area.

4. **Update docs**
   - If the behavior, configuration, or public surface changes, update `README.md`, `docs/`, and/or examples.

5. **Push and open a PR**

   ```bash
   git push origin feat/webrtc-metrics
   ```

   Then open a Pull Request on GitHub against the main branch, using:

   - A clear title (ideally matching the main commit message).
   - A description that explains _what_ changed and _why_.
   - Links to related Issues or Discussions, if applicable.

6. **Address review feedback**
   - Respond to comments, push follow-up commits, and keep the PR branch up to date with the target branch.

7. **Merge**
   - A maintainer will handle merging once CI is green and the review is complete.

## Bug Reports

High-quality bug reports make it much easier to diagnose and fix issues in a real-time system like Voxray-AI.

### Before opening an issue

- Check existing **Issues** and **Discussions** to see if the problem is already known.
- Verify that you are running against the latest main branch or a recent release.

### Recommended bug report structure

When opening a GitHub Issue, include the following sections in the description:

1. **Summary**
   - One or two sentences describing the problem.
2. **Environment**
   - Voxray-AI version or commit SHA
   - Go version
   - OS / architecture
   - Deployment mode (local, Docker, Kubernetes, cloud)
3. **Configuration**
   - Relevant portions of your Voxray-AI config (redact secrets).
   - Providers in use (STT, LLM, TTS) and selected models.
4. **Steps to Reproduce**
   - A minimal, reliable sequence of steps that reproduces the issue.
   - Example: “Connect via WebSocket to `/ws` with config X, send audio Y, observe behavior Z.”
5. **Expected Behavior**
   - What you expected Voxray-AI to do.
6. **Actual Behavior**
   - What actually happened, including error messages or incorrect responses.
7. **Logs and Metrics**
   - Relevant log excerpts (sanitized).
   - Any Prometheus metrics or dashboards that help illustrate the problem.
8. **Additional Context**
   - Links to related Issues or Discussions.
   - Any workarounds you have identified.

If the repository defines GitHub Issue templates, follow the most appropriate template and map the sections above to the requested fields.

## Links

- **GitHub Repository**: `https://github.com/voxray-ai/voxray-ai`
- **Issues**: `https://github.com/voxray-ai/voxray-ai/issues`
- **Discussions**: `https://github.com/voxray-ai/voxray-ai/discussions`
- **Discord**: See the invite link in `README.md` or project badges.

Thank you for contributing to Voxray-AI and helping improve the streaming voice stack.

