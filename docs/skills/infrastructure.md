## 3. Infrastructure

This section covers **where and how Voxray runs**, including deployment topology, external dependencies (databases, storage, queues, telephony, Daily), networking, and secrets management.

---

### 3.1 Runtime & Deployment Model

- **Go binary server — Intermediate**
  - **Entry**: `cmd/voxray/main.go` compiles into a single `voxray` binary (or `voxray.exe` on Windows) that loads `config.json`, configures logging, sets up recording/transcripts, registers processors, and starts HTTP/WebSocket servers via `pkg/server.StartServers`.
  - **Process model**:
    - One OS process per instance; **one goroutine per active connection** (`pipeline.Runner`), plus background goroutines for queues (e.g. audio input queue, recording uploader workers).
    - No internal job scheduler; background work is tied to connection lifetime (e.g. STT/LLM/TTS streams, telephony calls) and a global S3 worker pool.
  - **Skill implication**: Contributors should understand Go’s single‑binary deployment model and how per‑connection goroutines interact with process‑wide resources (metrics registry, recording uploader, transcript store).

- **Docker & containerization — Beginner/Intermediate**
  - **Files**: `Dockerfile`, `docker-compose.yml`.
  - **Dockerfile**:
    - Multi‑stage build from `golang:1.25-alpine`, downloading modules and building a static binary with `CGO_ENABLED=0 GOOS=linux`.
    - Runtime image `alpine:3.20` with a non‑root `voxray` user; exposes port 8080 and runs `/voxray -config /app/config.json`.
  - **docker-compose**:
    - Single `voxray` service, building from the Dockerfile, binding `8080:8080`, and mounting `./config.json` into the container.
    - Optional Redis service is commented; can be enabled for horizontal scaling (see below).
  - **Skill implication**: Basic Docker literacy is sufficient (building and running the container, mounting config, adjusting ports); deeper expertise is only required when customizing images or running multi‑service setups.

- **Configuration & 12‑factor overrides — Intermediate**
  - **Primary file**: `config.json` (often copied from `config.example.json`).
  - **Loader**: `pkg/config.LoadConfig` parses JSON and then `ApplyEnvOverrides` applies environment variables such as `VOXRAY_PORT`, `VOXRAY_HOST`, `VOXRAY_LOG_LEVEL`, `VOXRAY_TLS_*`, `VOXRAY_SERVER_API_KEY`, `VOXRAY_DAILY_DIALIN_WEBHOOK_SECRET`, `VOXRAY_RECORDING_*` (including `VOXRAY_RECORDING_QUEUE_CAP`, `VOXRAY_RECORDING_MAX_RETRIES`), `VOXRAY_TRANSCRIPTS_*`, `VOXRAY_CORS_ORIGINS`, `VOXRAY_PIPELINE_INPUT_QUEUE_CAP`, `VOXRAY_WS_WRITE_COALESCE_MS`, `VOXRAY_WS_WRITE_COALESCE_MAX_FRAMES`, body‑size limits, and API keys. Resolved API keys are cached to avoid repeated env lookups.
  - **Skill implication**: Contributors adding new infra‑level options must:
    - Extend the `Config` struct with clear JSON tags.
    - Add safe defaults and environment overrides.
    - Consider production behavior (e.g. security defaults, backward compatibility when fields are omitted).

---

### 3.2 Data Stores & Persistence

- **Postgres / MySQL (transcripts) — Intermediate**
  - **Files**: `pkg/transcripts/sqlstore.go`, `pkg/config/config.go` (`TranscriptConfig`), README transcript section.
  - **Role**: Persist **per‑message text transcripts** for each session to a SQL table, capturing `session_id`, `role`, `text`, `seq`, and `created_at`.
  - **Behavior**:
    - At startup, when `cfg.Transcripts.Enable` is true and both `Driver` (`postgres` or `mysql`) and `DSN` are set, `run` constructs a `transcripts.SQLStore` and shares it across sessions.
    - `TranscriptObserver` (`pkg/observers/transcript.go`) listens on downstream frames:
      - On finalized `TranscriptionFrame`, writes a `"user"` row.
      - On aggregated `LLMTextFrame` buffered until `TTSSpeakFrame` or `End`/`Cancel`, writes an `"assistant"` row.
    - Table name defaults to `call_transcripts` but is configurable and validated by a regex to avoid SQL injection.
  - **Skill implication**:
    - Comfort with `database/sql`, DSNs, and how to enforce safe SQL patterns (parameter placeholders, validated identifiers).
    - Understanding that **transcript logging is best‑effort**: failures are surfaced via errors from `SaveMessage`, but the core voice pipeline continues.

- **Redis (session store) — Intermediate**
  - **Files**: `pkg/runner/session_store.go` (and related), `pkg/server/server.go`, `docs/SYSTEM_ARCHITECTURE.md`, `docker-compose.yml`.
  - **Role**: Optional shared **SessionStore** for **runner modes** (WebRTC sessions, Daily dial‑in), enabling horizontal scaling behind a load balancer.
  - **Behavior**:
    - When `session_store=redis` and `redis_url` are set, `runner.NewSessionStoreFromConfig` builds a Redis‑backed store.
    - `/start` creates a session with configuration and optional ICE options; `/sessions/{id}/api/offer` looks up the session, consumes its body, and starts a SmallWebRTC transport.
    - `/ready` checks Redis health when a Redis store is in use and returns 503 if unreachable.
  - **Skill implication**:
    - Familiarity with using Redis as a **coordination and state store** rather than a general queue.
    - Awareness of failure modes (Redis down → readiness fails; stale sessions → 404 from `/sessions/...`).

- **S3 (recording storage) — Intermediate**
  - **Files**: `pkg/recording/*`, `pkg/config/config.go` (`RecordingConfig`), README recording section.
  - **Role**: Asynchronous upload of **per‑session mixed audio** (WAV) to an S3 bucket using a configurable worker pool and job queue.
  - **Behavior**:
    - When `cfg.Recording.Enable` and `Bucket` are set, `run` creates a `recording.Uploader` with `recording.worker_count`, `recording.queue_cap` (default 32), and `recording.max_retries` (exponential backoff on S3 failure). Jobs hold temp file paths; workers stream from file to S3 (no full WAV in memory).
    - Each new session optionally creates a `ConversationRecorder` writing WAV audio to a temp file; on session end, a job is enqueued to upload to S3 with a key derived from base path, date, and session ID.
    - Prometheus metrics record enqueued/success/failed jobs and queue depth; failure metric is incremented only after all retries are exhausted.
  - **Skill implication**:
    - Understanding of **eventual consistency**: recording uploads may succeed after a session ends; errors are observable via metrics and logs, not via client responses.
    - Basic AWS knowledge (credentials and region resolution via standard SDK mechanisms).

---

### 3.3 Telephony, Daily.co, and External Voice Systems

- **Telephony providers (Twilio, Telnyx, Plivo, Exotel) — Advanced**
  - **Files**: `pkg/server/server.go` (`registerTelephonyRoutes`), `pkg/runner`, `pkg/frames/serialize/*`, `docs/CONNECTIVITY.md`, `docs/SYSTEM_ARCHITECTURE.md`.
  - **Role**: Allow inbound PSTN calls to drive voice pipelines via provider webhooks and WebSocket media.
  - **Behavior**:
    - When `runner_transport` is `twilio|telnyx|plivo|exotel`, `POST /` returns provider‑specific XML/JSON that points a telephony media stream at `/telephony/ws`.
    - `/telephony/ws` upgrades to WebSocket, reads initial messages to detect the provider, selects a provider‑specific serializer, and then translates between media frames (e.g. PCM/Opus) and Voxray frames.
  - **Skill implication**:
    - Experience with telephony concepts (TWiML, media streams, DTMF) and how they map onto Voxray’s frame model.
    - Ability to debug complex interop problems (provider‑specific quirks, network delays, codec mismatches).

- **Daily.co — Intermediate/Advanced**
  - **Files**: `pkg/server/server.go` (`registerDailyRoutes`, `/daily-dialin-webhook`), `pkg/runner/daily`, `docs/SYSTEM_ARCHITECTURE.md`.
  - **Role**: Provide room creation and optional PSTN dial‑in for WebRTC sessions.
  - **Behavior**:
    - `GET /` (when `runner_transport="daily"`) creates a Daily room and redirects the user to it.
    - Optional `POST /daily-dialin-webhook` accepts a dial‑in webhook from Daily, validates a secret, and responds with room URL, token, and a new `sessionId`.
    - The rest of the flow uses runner endpoints and SmallWebRTC just like WebRTC clients.
  - **Skill implication**:
    - Comfort with third‑party room APIs and token handling.
    - Understanding how Daily is layered on top of the same pipeline/transport abstractions as other clients.

---

### 3.4 Networking, Security & Secrets

- **HTTP endpoints & topology — Intermediate**
  - **Files**: `pkg/server/server.go`, `docs/CONNECTIVITY.md`, `docs/SYSTEM_ARCHITECTURE.md`, `docs/DEPLOYMENT.md`.
  - **Core endpoints**:
    - `/ws` — WebSocket transport (JSON, protobuf, or RTVI).
    - `/webrtc/offer` — SmallWebRTC signaling endpoint (accepts JSON `{"offer": "<SDP>"}` and returns `{"answer": "<SDP>"}`).
    - `/start` and `/sessions/{id}/api/offer` — Runner endpoints for session‑based WebRTC flows (including Daily).
    - `/telephony/ws` and `POST /` — Telephony media and webhook.
    - `/metrics` — Prometheus metrics; `/health` and `/ready` — liveness/readiness.
    - `/swagger/` — Swagger UI; `/` — web client or JSON status, depending on mode.
  - **Topology**:
    - In production, one or more Voxray instances sit behind a load balancer or reverse proxy (often terminating TLS).
    - For WebRTC and telephony, consider whether you need sticky sessions or rely on Redis session store to route offers/calls to any instance.

- **TLS & CORS — Intermediate**
  - **TLS**:
    - Either terminate TLS inside Voxray (set `tls_enable`, `tls_cert_file`, `tls_key_file`) or at a reverse proxy.
    - Important for `wss://` and HTTPS requirements for browsers and telephony providers.
  - **CORS**:
    - `setCORS` in `pkg/server/server.go` uses `cors_allowed_origins` (or `VOXRAY_CORS_ORIGINS`) to reflect allowed origins or fall back to `*` when unset, primarily for WebSocket upgrades and REST endpoints.
  - **Skill implication**:
    - Familiarity with standard web security practices (TLS, CORS, body size limits via `MaxBytesReader`) and how misconfiguration can expose endpoints unintentionally.

- **API keys & secrets management — Intermediate**
  - **Sources**:
    - Environment variables (e.g. `OPENAI_API_KEY`, `GROQ_API_KEY`, `DAILY_API_KEY`) read via `cfg.GetAPIKey`; resolved values are cached.
    - `config.json` `api_keys` section for local development; not recommended for committing secrets.
  - **Usage**:
    - Provider adapters fetch keys at runtime and pass them to SDK clients or HTTP requests.
    - `docs/DEPLOYMENT.md` emphasizes using env vars or a secrets manager in production.
  - **Skill implication**:
    - Understanding of safe secret handling: never hardcoding keys, avoiding logging secrets, and ensuring that config examples are sanitized.

- **Auth for client access — Beginner/Intermediate**
  - **Files**: `pkg/server/server.go` (`requireAPIKey`).
  - **Behavior**:
    - Optional `server_api_key` config value; when set, `/ws`, `/webrtc/offer`, `/start`, `/sessions/...` and some other endpoints require either `X-API-Key` or `Authorization: Bearer <key>`.
  - **Skill implication**:
    - Basic API key auth knowledge; more advanced auth (OAuth, JWT) would be added on top if required.

---

### 3.5 Observability & Monitoring

- **Prometheus metrics — Intermediate**
  - **Files**: `pkg/metrics/prom.go`, `pkg/server/server.go`, `pkg/observers`, `docs/DEPLOYMENT.md`.
  - **Scope**:
    - HTTP metrics (requests, latencies, active connections, errors/timeouts).
    - WebRTC metrics (peer connection counts, bytes in/out, connection failures, reconnection attempts).
    - STT, LLM, and TTS metrics (errors, fallbacks, time‑to‑first‑token, total latency, streaming lag).
    - Recording queue metrics (jobs enqueued/succeeded/failed, queue depth).
  - **Topology**:
    - `/metrics` exports a Prometheus text endpoint; metrics are per‑process and aggregated by Prometheus across instances.
    - Session IDs are **hashed and sampled** (`SampledSessionID`) to avoid high cardinality.
  - **Skill implication**:
    - Ability to read and extend metrics responsibly (choosing labels, sampling strategies).
    - Understanding which metrics to consult when debugging latency or failure spikes.

---

### 3.6 Scaling & Topology Notes

- **Single‑node mode — Low complexity**
  - One Voxray process, in‑memory session store, no Redis. Suitable for development and small deployments.
  - Recording uploads and transcript logging still use external services (S3, Postgres/MySQL) if enabled.

- **Horizontally scaled mode — Medium complexity**
  - Multiple Voxray instances behind a load balancer.
  - Redis configured as session store (`session_store=redis`, `redis_url`); health/readiness incorporate Redis status.
  - Prometheus scrapes each instance; metrics are aggregated in the Prometheus backend.
  - Telephony and Daily flows may require careful consideration of load‑balancer routing (e.g. stickiness vs. Redis session coordination).

---

### 3.7 Onboarding Guidance (Infrastructure)

- **Required skills (Medium complexity)**:
  - Solid grasp of HTTP servers, TLS, and CORS in Go.
  - Working knowledge of Docker and basic orchestration (docker‑compose or Kubernetes).
  - Familiarity with at least one SQL database and S3‑like object storage.
- **Optional but valuable**:
  - Prior experience with Redis as a shared session store.
  - Telephony and WebRTC deployment experience, especially for NAT traversal and call quality.

