## 1. Core Stack

This section maps the concrete **languages, runtimes, and non‑AI libraries** used in Voxray-Go to the skills a new contributor needs. Proficiency levels are for someone contributing to production code.

---

### 1.1 Languages & Runtime

- **Go (1.25+) — Advanced**
  - **Role**: Primary language for the entire server (`cmd/voxray`, `pkg/**`, `tests/**`). All runtime behavior (pipelines, transports, providers, metrics, extensions) is implemented in Go.
  - **Key skills in this repo**:
    - **Concurrency & goroutines**: One goroutine per connection (`pipeline.Runner` in `pkg/pipeline/runner.go`), background workers for queues and recording, and observer wrapping. Runner uses a configurable buffered input queue and context-aware select so cancellation drains cleanly. Observers are notified asynchronously so the chain is not blocked. Ability to reason about cancellation via `context.Context` and proper shutdown.
    - **Channels & backpressure**: Use of buffered channels to decouple I/O from processing. Pipeline input queue capacity is configurable (`pipeline_input_queue_cap`, default 256); when full, the transport reader blocks (back-pressure). Transport layer uses single reader/writer goroutines per connection; WebSocket supports optional write coalescing to reduce syscalls.
    - **Interfaces & composition**: Core abstractions are defined as interfaces (`Transport`, `Processor`, `SessionStore`, `LLMService`, `RealtimeService`, etc.), with many implementations wired via factory/registry patterns.
    - **Error handling**: Explicit error returns, wrapping (`fmt.Errorf("...: %w", err)`), and decisions about when to log vs. propagate vs. drop errors (e.g. in observers or best-effort logging paths).
    - **Testing**: Familiarity with `testing` and external test packages (`tests/pkg/**`), along with table-driven tests for providers, metrics, and frame handling.
  - **Why Advanced**: The codebase leans heavily on idiomatic Go patterns (contexts, interfaces, streaming I/O, concurrency). Modifying pipelines, transports, or services safely requires strong Go fluency, not just syntax knowledge.

- **Shell / PowerShell — Beginner**
  - **Role**: Developer tooling (`Makefile`, `scripts/*.sh`, `scripts/*.ps1`) for building, linting, stress testing, and running voice builds on different OSes.
  - **Key skills**:
    - Running `make` targets (`build`, `build-voice`, `run-voice`, `lint`, `evals`) and simple shell commands.
    - On Windows, running PowerShell scripts for CGO/voice builds and memory watching.
  - **Why Beginner**: Contributors primarily need to **invoke** these scripts, not author sophisticated shell logic.

- **Markdown — Beginner**
  - **Role**: Project documentation (`README.md`, `docs/*.md`, `tests/README.md`, `docs/skills/*.md`).
  - **Key skills**: Keeping docs consistent, using headings/tables/links, and embedding Mermaid diagrams for architecture views.
  - **Why Beginner**: Structure is straightforward; focus is on content accuracy, not formatting tricks.

---

### 1.2 Networking & Protocols

- **HTTP & REST (net/http) — Intermediate**
  - **Role**: All entrypoints (`/ws`, `/webrtc/offer`, `/start`, `/sessions/{id}/api/offer`, `/telephony/ws`, `/health`, `/ready`, `/metrics`, `/swagger/`, Daily and telephony webhooks) are HTTP handlers in `pkg/server/server.go`.
  - **Key skills**:
    - Implementing and wrapping `http.Handler` middleware for metrics, CORS, and auth.
    - Managing request bodies safely (e.g. `MaxBytesReader`, JSON decoding with `encoding/json`).
    - Understanding how different endpoints map to transports (WebSocket, WebRTC, telephony) and how HTTP lifecycle interacts with long‑lived connections.
  - **Why Intermediate**: Patterns are idiomatic but you must respect timeouts, body limits, and metrics labeling to avoid performance and observability issues.

- **WebSocket (gorilla/websocket) — Intermediate**
  - **Role**: Primary network transport (`/ws`) for text/audio frames, plus telephony backhaul on `/telephony/ws`.
  - **Key skills**:
    - Understanding how `ConnTransport` wraps a WebSocket connection into the `Transport` interface with `Input`/`Output` channels and `Start`/`Close`.
    - Handling binary vs JSON payloads, and selecting serializers (JSON envelope, protobuf, or RTVI) based on query params.
    - Appreciating reconnection/backoff patterns and long‑lived service connections, as documented in `docs/WEBSOCKET_SERVICES.md`.
  - **Why Intermediate**: Most contributors won’t write WebSocket framing from scratch, but many features (custom serializers, RTT measurements, service reconnection) depend on understanding this layer.

- **WebRTC (pion/webrtc) — Intermediate**
  - **Role**: SmallWebRTC transport for low‑latency audio, used by `/webrtc/offer` and runner sessions.
  - **Key skills**:
    - Knowing how server‑side WebRTC is represented via the SmallWebRTC abstraction and how SDP offers/answers are exchanged over HTTP.
    - Recognizing where ICE servers and STUN configuration live (`config.json` / `cfg.WebRTCICEServers`) and how they impact connectivity.
  - **Why Intermediate**: Many contributors can work without touching WebRTC internals, but changes to transports, runner modes, or latency behavior will require reading Pion‑backed code.

---

### 1.3 Audio & Media Stack (Non‑AI)

- **Opus codecs (pion/opus, godeps/opus, gopus) — Intermediate**
  - **Role**: Encode TTS output for WebRTC in Opus; only available when CGO and a C compiler are present (voice builds).
  - **Key skills**:
    - Understanding when an Opus encoder is available vs. when the server should return `503` and log an informative error.
    - Handling audio buffer sizes, channels, and sample rates (16 kHz vs. 24 kHz) consistently across processors and transports.
  - **Why Intermediate**: Contributors working on TTS, WebRTC, or audio recording must understand how Opus fits into the pipeline, but higher‑level features rarely touch codec details.

- **Audio processing (pkg/audio, VAD/turn) — Advanced**
  - **Role**: Voice Activity Detection (VAD), turn detection, and audio buffering for recording. Used heavily by `voice.TurnProcessor` and the conversation recorder.
  - **Key skills**:
    - Tuning VAD parameters (confidence, start/stop seconds, thresholds, min volume) and understanding how they affect user experience (latency vs. false positives/negatives).
    - Working with different detectors (Silero vs energy‑based) and fallbacks when ML‑based VAD isn’t available.
    - Implementing or modifying turn detection logic (`turn.Params`, `SilenceTurnAnalyzer`) and understanding its relationship to `UserIdleTimeout` and user‑turn stop semantics.
  - **Why Advanced**: Changes here directly affect when the system considers a user done speaking, which is critical to perceived latency and correctness in voice UX.

- **WAV/PCM handling — Intermediate**
  - **Role**: Recording conversation audio to WAV, converting from byte streams into `int16` samples, and writing to disk before S3 upload.
  - **Key skills**:
    - Correct treatment of endianness and sample formats when converting streamed raw bytes to audio samples.
    - Coordinating sample rate and channel configuration between recording buffers and audio processors.
  - **Why Intermediate**: The patterns are straightforward but correctness matters (bad conversions lead to noisy or silent recordings).

---

### 1.4 Observability & Metrics

- **Prometheus client (prometheus/client_golang) — Intermediate**
  - **Role**: Core metrics system, with a dedicated registry and numerous counters, gauges, and histograms in `pkg/metrics/prom.go`.
  - **Key skills**:
    - Understanding labeling strategy (session ID sampling, stage, direction, status, model) and what high‑cardinality labels to avoid.
    - Adding new metrics in a way that respects the existing registry and init pattern, and correctly wiring them in observers or processors.
    - Interpreting metrics for HTTP, WebRTC, STT, LLM, TTS, and recording jobs to diagnose performance issues.
  - **Why Intermediate**: Most contributors will at least read or slightly extend metrics; designing new metrics or debugging label cardinality benefits from some experience.

- **Logging (go.uber.org/zap, std log wrappers) — Beginner/Intermediate**
  - **Role**: Structured and JSON logging configured via `pkg/logger`, used throughout the codebase.
  - **Key skills**:
    - Knowing how log levels are configured (`log_level`, `json_logs`) and when to use `Info` vs `Error` vs `Debug`.
    - Writing log messages that include enough context (IDs, provider names, model names) for debugging in production.
  - **Why Beginner/Intermediate**: Logging usage is not complex, but thoughtful context is important in a multi‑provider real‑time system.

---

### 1.5 HTTP API Documentation & Serialization

- **Swagger / OpenAPI (swaggo/swag, http-swagger) — Beginner/Intermediate**
  - **Role**: Generates and serves API docs for `/webrtc/offer` and related endpoints (`docs/swagger.yaml`, swagger annotations in `cmd/voxray/main.go` and `pkg/server`).
  - **Key skills**:
    - Adding or updating swagger annotations on handlers for new endpoints or fields.
    - Regenerating documentation via `make swagger` and understanding how docs are served from `/swagger/`.
  - **Why Beginner/Intermediate**: The tooling is standard; the main challenge is keeping docs aligned with actual behavior.

- **JSON & Protobuf serialization — Intermediate**
  - **Role**: Frames are serialized to JSON envelopes for WebSocket, or to protobuf messages for binary WebSocket and some telephony flows.
  - **Key skills**:
    - Understanding the JSON envelope format (frame `type` + `data`) and how it maps to Go frame types in `pkg/frames`.
    - Knowing when to use protobuf serializers (e.g. `?format=protobuf`, telephony backhaul) and ensuring message schemas remain compatible with external clients.
  - **Why Intermediate**: Most contributors will touch frame definitions or serializers at some point; mistakes here break wire compatibility.

---

### 1.6 Configuration & Plugin System

- **Config management (encoding/json, env overrides) — Intermediate**
  - **Role**: `pkg/config` parses `config.json`, applies environment overrides (`ApplyEnvOverrides`), and exposes helpers for derived behavior (VAD params, metrics enablement, TLS, session store).
  - **Key skills**:
    - Safely adding config fields that can be overridden via environment variables, with sensible defaults and validation.
    - Understanding how config drives transport modes, runner transports, providers, models, recording, transcripts, MCP, and plugins.
  - **Why Intermediate**: In a config‑driven system, most feature work involves adding or modifying config options without breaking existing setups.

- **Processor registry & plugins (custom processors) — Advanced**
  - **Role**: `pkg/pipeline/registry.go` and `cmd/voxray/main.go` register processors under names (e.g. `echo`, `logger`, `llmfullresponse`, `external_chain`, `rtvi`) so `cfg.Plugins` and `PluginOptions` can dynamically build pipelines.
  - **Key skills**:
    - Implementing new processors that satisfy the `Processor` interface and registering them with appropriate options.
    - Reasoning about upstream vs downstream directions in processors, and how frames can move both ways (e.g. IVR and voicemail extensions using upstream frames).
    - Ensuring processors behave correctly under backpressure, cancellation, and error frames.
  - **Why Advanced**: Extending the plugin system or adding complex processors requires deep familiarity with pipeline semantics and frame lifecycles.

---

### 1.7 External Integrations (Non‑AI)

- **Redis (redis/go-redis/v9) — Intermediate**
  - **Role**: Optional session store backend (`SessionStore` in `pkg/runner`) used for horizontal scaling and runner modes.
  - **Key skills**:
    - Understanding how sessions are persisted and retrieved (e.g. for `/start` and `/sessions/{id}/api/offer` in runner mode).
    - Handling connectivity issues and how they affect readiness (`/ready` checks Redis health).
  - **Why Intermediate**: Core voice flows work without Redis, but scaling out and runner features rely on it.

- **Postgres/MySQL (database/sql + lib/pq, go-sql-driver/mysql) — Intermediate**
  - **Role**: Transcript storage via `pkg/transcripts/sqlstore.go`, with configurable DSN and table name.
  - **Key skills**:
    - Safely building SQL insert statements with parameter placeholders (Postgres vs MySQL) and validating table names.
    - Handling connection lifecycles (open, ping, close) and surfacing errors correctly.
  - **Why Intermediate**: The data model is simple, but contributors must be careful about config validation and error propagation to avoid silent failures.

- **AWS S3 (aws-sdk-go-v2/service/s3) — Intermediate**
  - **Role**: Asynchronous upload of conversation recordings from local WAV files to S3 with a configurable worker pool and job queue in `pkg/recording`. Jobs reference temp file paths; workers stream to S3 (no full WAV in memory). Retries use exponential backoff up to `recording.max_retries`.
  - **Key skills**:
    - Understanding the job queue pattern (enqueue on session end, worker goroutines performing uploads) and metrics instrumentation for successes/failures.
    - Reasoning about eventual consistency and retry strategies for uploads.
  - **Why Intermediate**: Contributors altering recording behavior need to understand how S3 operations interact with session lifecycles and observability.

---

### 1.8 Summary & Onboarding Guidance (Core Stack)

- **Core must‑have skills**:
  - Advanced Go (concurrency, interfaces, context, testing) for server/pipeline work.
  - Intermediate HTTP/WebSocket understanding to work with transports and endpoints.
  - Intermediate familiarity with Prometheus metrics and JSON/protobuf framing to maintain observability and wire compatibility.
- **Nice‑to‑have / specialized skills**:
  - Audio/VAD and WebRTC internals for voice UX and transport specialists.
  - Redis/Postgres/MySQL/S3 familiarity for infra‑oriented contributors.
  - Experience with plugin architectures and streaming processors for adding new pipeline components.

