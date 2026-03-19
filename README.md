# Voxray-AI

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> Build production-ready AI voice agents with a single JSON config.
> WebSocket & WebRTC · STT → LLM → TTS · Low-latency · Self-hostable

Config-driven Go server for building real-time voice agents. Wire together speech-to-text, LLM, and text-to-speech providers into low-latency streaming pipelines — no audio plumbing required.

---

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Features](#features)
- [Supported Providers](#supported-providers)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Environment Variables](#environment-variables)
- [Examples](#examples)
- [Use Cases](#use-cases)
- [Roadmap](#roadmap)
- [Documentation](#documentation)
- [License](#license)
- [Contributing](#contributing)

---

## Overview

Voxray-AI (`voxray-go`) is a **config-driven Go server** for building **real-time voice agents** over **WebSocket** and **WebRTC**. It wires together **STT**, **LLM**, and **TTS** providers into low-latency streaming pipelines. Pipelines, providers, and transports are defined via JSON config, making it easy to swap services and deploy to your own infrastructure.

For architecture and pipeline details, see [Architecture](docs/ARCHITECTURE.md).

---

## Quick Start

Get the server running end-to-end in under 5 minutes.

**1. Prerequisites**

```bash
go version    # Go 1.25+ required (see go.mod)
gcc --version # only needed for WebRTC/Opus — see Requirements
```

**2. Clone and build**

```bash
git clone https://github.com/your-org/voxray-ai.git
cd voxray-ai
go build -o voxray ./cmd/voxray
# or: make build
```

**3. Configure**

```bash
cp config.example.json config.json
# Set your API keys in config.json or via environment variables (e.g. OPENAI_API_KEY)
```

**4. Run**

```bash
./voxray -config config.json
# Windows: .\voxray.exe -config config.json
```

You can override config with flags: `-config`, `-transport` (webrtc, daily, twilio, telnyx, plivo, exotel), `-port`, `-proxy` (public hostname for telephony webhooks), `-dialin` (Daily PSTN; requires transport=daily). Use `-init` to scaffold `config.json` and dirs then exit, or run `voxray init [config-path]`.

**5. Connect**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws` | GET | WebSocket transport (upgrade) |
| `/webrtc/offer` | POST | WebRTC signaling (SDP offer/answer) |
| `/health` | GET | Liveness |
| `/ready` | GET | Readiness |
| `/start` | POST | Create session (runner-style WebRTC) |
| `/sessions/:id/offer`, `/api/v1/sessions/:id/offer` | POST, PATCH | Session SDP offer (after `/start`) |
| `/telephony/ws` | GET | Telephony media WebSocket (when `runner_transport` is Twilio/Telnyx/Plivo/Exotel) |
| `/swagger/` | GET | Swagger UI (when built with swag) |
| `/metrics` | GET | Prometheus metrics |

Runner and telephony behavior are detailed in [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md).

**6. Try the WebRTC browser client (optional)**

```bash
cd tests/frontend && python -m http.server 3000
# Open http://localhost:3000/webrtc-voice.html, set Server URL to http://localhost:8080, click Start
```

See [tests/frontend/README.md](tests/frontend/README.md) for details.

---

## Features

- **Low-latency pipelines** — STT → LLM → TTS with configurable providers and models
- **Dual transports** — WebSocket (`/ws`) and WebRTC via SmallWebRTC (`/webrtc/offer`)
- **Telephony & Daily.co** — Twilio, Telnyx, Plivo, Exotel, and Daily.co (rooms + optional PSTN dial-in); media over WebSocket after provider webhooks
- **MCP tool integration** — optional MCP server (configurable command/args) so the LLM can call tools
- **Wide provider support** — OpenAI, Anthropic, Groq, Sarvam, AWS, Google, ElevenLabs, and more
- **Plugin system** — custom processors and aggregators via an extensible framework
- **Config-driven** — JSON configuration for all pipeline stages; API keys via config or environment variables
- **Conversation recording** — mixed audio per session, uploaded asynchronously to S3
- **Transcript logging** — per-message text logs to Postgres or MySQL
- **Observability** — Prometheus metrics at `/metrics`
- **Voice over WebRTC** — optional CGO/Opus build for real-time TTS audio delivery

---

## Supported Providers

Provider sets and capability matrix are defined in [pkg/services](pkg/services/README.md) (`SupportedSTTProviders`, `SupportedLLMProviders`, `SupportedTTSProviders` in `factory.go`). Summary:

| Stage   | Provider      | Notes                                      |
| :------ | :------------- | :----------------------------------------- |
| **STT** | OpenAI        | Whisper via OpenAI API (e.g. `gpt-4o-mini-transcribe`) |
|         | Groq          | —                                          |
|         | Sarvam        | Indian languages                           |
|         | ElevenLabs    | —                                          |
|         | AWS           | Amazon Transcribe                          |
|         | Google        | Cloud Speech-to-Text                       |
|         | Whisper       | Direct Whisper integration                  |
|         | Camb          | —                                          |
|         | Gradium       | —                                          |
|         | Soniox        | —                                          |
| **LLM** | OpenAI        | GPT-4.1, GPT-4o, etc.                      |
|         | Groq          | —                                          |
|         | Grok          | —                                          |
|         | Cerebras      | —                                          |
|         | AWS           | Amazon Bedrock                             |
|         | Mistral       | —                                          |
|         | DeepSeek      | —                                          |
|         | Anthropic     | Claude                                     |
|         | Google        | Gemini                                     |
|         | Google Vertex | ADC-based authentication                   |
|         | Ollama        | Local/self-hosted models                   |
|         | Qwen          | —                                          |
|         | AsyncAI       | —                                          |
|         | Fish          | —                                          |
|         | Inworld       | —                                          |
|         | Minimax       | —                                          |
|         | Moondream     | —                                          |
|         | OpenPipe      | —                                          |
| **TTS** | OpenAI        | `alloy`, `nova`, etc.                      |
|         | Groq          | —                                          |
|         | Sarvam        | Indian languages                           |
|         | ElevenLabs    | —                                          |
|         | AWS           | Amazon Polly                               |
|         | Google        | Cloud Text-to-Speech                       |
|         | Hume          | —                                          |
|         | Inworld       | —                                          |
|         | Minimax       | —                                          |
|         | Neuphonic     | —                                          |
|         | XTTS          | Self-hosted Coqui XTTS                     |

---

## Architecture

Audio is received from web or native clients over **WebSocket** or **WebRTC**, processed through a configurable **STT → LLM → TTS** pipeline, and streamed back over the same transport. Each stage is pluggable — mix and match providers while keeping a consistent, low-latency pipeline.

```mermaid
flowchart TB
  subgraph Client["Client"]
    Browser["Browser / Native app"]
  end
  subgraph Server["Server"]
    HTTP["HTTP\n/ws, /webrtc/offer\n/metrics"]
  end
  subgraph Transport["Transport"]
    WS["WebSocket"]
    WebRTC["SmallWebRTC"]
  end
  subgraph Pipeline["Pipeline"]
    Runner["Runner"]
    Chain["Processors\nVAD → STT → LLM → TTS → Sink"]
  end
  subgraph Providers["External providers"]
    STT_API["STT API"]
    LLM_API["LLM API"]
    TTS_API["TTS API"]
  end
  Browser --> WS
  Browser --> WebRTC
  WS --> HTTP
  WebRTC --> HTTP
  HTTP --> Runner
  Runner --> Chain
  Chain --> STT_API
  Chain --> LLM_API
  Chain --> TTS_API
  Chain --> WS
  Chain --> WebRTC
```

> Audio flows from clients (browser, runner, telephony, or Daily.co) into the server via WebSocket, SmallWebRTC, or telephony WebSocket. The runner wires each transport to the same pipeline (VAD → STT → LLM → TTS); external STT/LLM/TTS are called from [pkg/services](pkg/services/README.md). See [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md) and [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md).

For a deeper dive, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md).

---

## Requirements

**Go 1.25+** is the only hard requirement for the default (WebSocket-only) build.

```bash
go version    # should be 1.25+ (see go.mod)
```

For **voice over WebRTC (TTS audio via Opus)**, CGO and a C compiler (`gcc`) must also be on your PATH:

```bash
gcc --version # only needed for WebRTC/Opus builds
```

### C compiler on Windows

CGO requires `gcc` on your PATH. Two options:

**WinLibs (winget):**
```powershell
winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements
# Restart terminal, then verify:
gcc --version
```

**MSYS2:**

Install [MSYS2](https://www.msys2.org/), open **MSYS2 UCRT64**, then:
```bash
pacman -S mingw-w64-ucrt-x86_64-toolchain
```
Add `C:\msys64\ucrt64\bin` to PATH and verify with `gcc --version`.

> Without CGO, WebRTC TTS will report *opus encoder unavailable (build without cgo)* and the server returns **503** for WebRTC offers.

---

## Installation

The default build has no external dependencies. The voice/WebRTC build requires CGO and gcc (see [Requirements](#requirements)).

### Default build (WebSocket only, no Opus)

```bash
go build -o voxray ./cmd/voxray
# or:
make build && make run
```

### Build with voice (WebRTC TTS + Opus)

**Linux / macOS:**
```bash
make build-voice
./voxray -config config.json
# or in one step:
make run-voice ARGS="-config config.json"
```

**Windows (PowerShell):**
```powershell
# Build once, then run:
.\scripts\build-voice.ps1
.\voxray.exe -config config.json

# Or build and run in one step:
.\scripts\run-voice.ps1 -config config.json
```

**Manual (any OS):**
```bash
CGO_ENABLED=1 go build -o voxray ./cmd/voxray
./voxray -config config.json
# or:
CGO_ENABLED=1 go run ./cmd/voxray -config config.json
```

After a voice build, WebRTC offers succeed and TTS audio is delivered over the peer connection.

---

## Configuration

Set the config path via the `-config` flag or the `VOXRAY_CONFIG` environment variable. Copy [config.example.json](config.example.json) to `config.json` to get started.

### Top-level keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `transport` | string | `"websocket"` | `"websocket"`, `"smallwebrtc"`, or `"both"` |
| `host` | string | `"0.0.0.0"` | Bind host |
| `port` | int | `8080` | Bind port |
| `stt_provider` | string | — | STT provider name (e.g. `"openai"`) |
| `llm_provider` | string | — | LLM provider name (e.g. `"openai"`) |
| `tts_provider` | string | — | TTS provider name (e.g. `"openai"`) |
| `api_keys` | object | — | Map of provider → API key |
| `metrics_enabled` | bool | `true` | Expose Prometheus `/metrics` |
| `webrtc_ice_servers` | array | — | ICE server config for WebRTC |
| `rtc_max_duration_secs` | float | `0` | Max lifetime for RTC/WebSocket voice sessions after first inbound audio; `0` disables |
| `recording` | object | — | S3 conversation recording (see below) |
| `transcripts` | object | — | Database transcript logging (see below) |
| `mcp` | object | — | MCP server: `command`, `args`, `tools_filter` (see [pkg/config/README.md](pkg/config/README.md)) |

### Additional config

| Key | Description |
|-----|-------------|
| `provider` | Default provider for STT/LLM/TTS when task-specific (`stt_provider`, etc.) not set |
| `runner_transport` | `webrtc` \| `daily` \| `twilio` \| `telnyx` \| `plivo` \| `exotel` \| `livekit` \| `""` |
| `runner_port`, `proxy_host`, `dialin` | Runner and telephony (e.g. public hostname for webhooks; Daily PSTN dial-in) |
| `plugins`, `plugin_options` | Pipeline plugins and options (see [docs/EXTENSIONS.md](docs/EXTENSIONS.md)) |
| `turn_detection`, `turn_stop_secs`, `turn_pre_speech_ms`, `turn_max_duration_secs`, `vad_*`, `user_turn_stop_timeout_secs`, `user_idle_timeout_secs`, `turn_async` | Turn detection and VAD |
| `allow_interruptions`, `interruption_strategy`, `min_words` | Barge-in / interruption behavior |
| `cors_allowed_origins`, `max_request_body_bytes`, `server_api_key` | Server and optional API key auth |
| `legacy_errors`, `shutdown_upload_timeout_secs` | Compatibility and shutdown tuning |

See [config.example.json](config.example.json) and [examples/voice/README.md](examples/voice/README.md) for all options.

### Recording (S3)

Voxray can record the full mixed conversation audio per session and upload it asynchronously to S3.

```json
"recording": {
  "enable": true,
  "bucket": "your-recordings-bucket",
  "base_path": "recordings/",
  "format": "wav",
  "worker_count": 4
}
```

| Field | Description |
|-------|-------------|
| `enable` | Turn recording on for all sessions |
| `bucket` | S3 bucket name |
| `base_path` | Key prefix inside the bucket (default: `recordings/`) |
| `format` | File format — currently `wav` (16-bit PCM mono) |
| `worker_count` | Background uploader thread pool size |

Each session is written locally and, on session end, a background job uploads it to:
```
<base_path>/yyyy/mm/dd/<session-id>.wav
```

AWS credentials are resolved via the standard AWS SDK v2 chain (env vars, shared config, IAM role, etc.).

### Transcripts (Postgres / MySQL)

Persist per-message text transcripts (user and assistant) to a relational database.

**Postgres:**
```json
"transcripts": {
  "enable": true,
  "driver": "postgres",
  "dsn": "postgres://user:pass@localhost:5432/voxray?sslmode=disable",
  "table_name": "call_transcripts"
}
```

**MySQL:**
```json
"transcripts": {
  "enable": true,
  "driver": "mysql",
  "dsn": "user:pass@tcp(localhost:3306)/voxray?parseTime=true",
  "table_name": "call_transcripts"
}
```

**Expected schema (Postgres):**
```sql
CREATE TABLE call_transcripts (
  id          BIGSERIAL PRIMARY KEY,
  session_id  TEXT NOT NULL,
  role        TEXT NOT NULL,   -- "user" or "assistant"
  text        TEXT NOT NULL,
  seq         BIGINT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Prometheus metrics

The server exposes a Prometheus-compatible scrape endpoint at `/metrics` on the same host/port as `/ws` and `/webrtc/offer`.

- `"metrics_enabled": true` (default) — records HTTP, WebRTC, STT, LLM, and TTS metrics.
- `"metrics_enabled": false` — disables recording; `/metrics` returns `204 No Content` so Prometheus scrape configs don't break.

Metrics are process-local; Prometheus aggregates across instances using `instance`/`pod` labels.

---

## Environment Variables

All config values can be overridden via environment variables. Unknown keys in config JSON are silently ignored.

### Server

| Variable | Description |
|----------|-------------|
| `VOXRAY_CONFIG` | Path to config file (alternative to `-config` flag) |
| `VOXRAY_HOST` | Bind host |
| `VOXRAY_PORT` / `PORT` | Bind port |
| `VOXRAY_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `VOXRAY_JSON_LOGS` | `true` to emit structured JSON logs |
| `VOXRAY_CORS_ORIGINS` | Comma-separated allowed CORS origins |
| `VOXRAY_MAX_BODY_BYTES` | Max HTTP request body size in bytes |
| `VOXRAY_SERVER_API_KEY` | Server-level API key for auth |
| `VOXRAY_PIPELINE_INPUT_QUEUE_CAP` | Input queue capacity for pipeline |
| `VOXRAY_WS_WRITE_COALESCE_*` | WebSocket write coalescing settings |
| `VOXRAY_VAD_BATCH_SIZE` | VAD processor batch size |
| `VOXRAY_DAILY_DIALIN_WEBHOOK_SECRET` | Daily.co dial-in webhook secret |

### Recording

| Variable | Description |
|----------|-------------|
| `VOXRAY_RECORDING_ENABLE` | `true` to enable S3 recording |
| `VOXRAY_RECORDING_BUCKET` | S3 bucket name |
| `VOXRAY_RECORDING_BASE_PATH` | Key prefix inside the bucket |
| `VOXRAY_RECORDING_FORMAT` | File format (e.g. `wav`) |
| `VOXRAY_RECORDING_WORKER_COUNT` | Uploader thread pool size |
| `VOXRAY_RECORDING_QUEUE_CAP` | Upload job queue capacity |
| `VOXRAY_RECORDING_MAX_RETRIES` | Max upload retry attempts |

### Transcripts

| Variable | Description |
|----------|-------------|
| `VOXRAY_TRANSCRIPTS_ENABLE` | `true` to enable transcript logging |
| `VOXRAY_TRANSCRIPTS_DRIVER` | `postgres` or `mysql` |
| `VOXRAY_TRANSCRIPTS_DSN` | Database connection string |
| `VOXRAY_TRANSCRIPTS_TABLE` | Target table name |

---

## Examples

For provider/model-specific examples, see [examples/voice/README.md](examples/voice/README.md).
For the browser-based WebRTC client, see [tests/frontend/README.md](tests/frontend/README.md).

### Complete example `config.json`

Copy this, fill in your API keys, and run:

```json
{
  "transport": "both",
  "host": "0.0.0.0",
  "port": 8080,
  "metrics_enabled": true,

  "stt_provider": "openai",
  "stt_model": "gpt-4o-mini-transcribe",

  "llm_provider": "openai",
  "model": "gpt-4.1-mini",

  "tts_provider": "openai",
  "tts_voice": "alloy",

  "api_keys": {
    "openai": "YOUR_OPENAI_API_KEY"
  },

  "webrtc_ice_servers": [
    "stun:stun.l.google.com:19302"
  ]
}
```

Run with:
```bash
./voxray -config config.json
```

Then connect at `http://localhost:8080/ws` (WebSocket) or `http://localhost:8080/webrtc/offer` (WebRTC).

---

## Use Cases

- **AI call centers / IVR** — conversational agents for inbound and outbound calls with low latency
- **In-app voice copilots** — embed voice agents inside SaaS or productivity apps via WebSocket or WebRTC
- **Operations and support bots** — voicebots for support, ops, and internal tooling on your own infrastructure
- **Realtime monitoring and control** — voice interfaces for dashboards, observability tools, and control systems
- **On-prem / VPC assistants** — self-hosted voice-AI stacks where data must stay within your cloud or datacenter

---

## Roadmap

**Near-term**
- [ ] More built-in STT/LLM/TTS providers and opinionated presets for common stacks
- [ ] Deeper observability, tracing, and debugging tools for real-time pipelines

**Planned**
- [ ] Deployment templates (Docker, Kubernetes)
- [ ] Additional starter agent examples for popular voice-agent scenarios
- [ ] Expanded documentation on scaling, deployment patterns, and production hardening

---

## Documentation

### Repository layout

| Package | README |
|---------|--------|
| `pkg/pipeline` | [Pipeline, runner, source/sink, task, registry](pkg/pipeline/README.md) |
| `pkg/transport` | [WebSocket, WebRTC, in-memory transports](pkg/transport/README.md) |
| `pkg/services` | [LLM, STT, TTS interfaces and provider factory](pkg/services/README.md) |
| `pkg/recording` | [Conversation recording and S3 upload](pkg/recording/README.md) |
| `pkg/metrics` | [Prometheus metrics](pkg/metrics/README.md) |
| `pkg/config` | [Configuration and env overrides](pkg/config/README.md) |
| `pkg/processors` | [Voice, echo, filters, aggregators](pkg/processors/README.md) |
| `pkg/runner` | [Session store and runner args](pkg/runner/README.md) |
| `pkg/utils` | [Backoff, notifier, sentence, aggregators](pkg/utils/README.md) |
| `pkg/frames` | [Frame types and serialization](pkg/frames/README.md) |
| `pkg/audio` | [VAD, turn detection, codecs, resample](pkg/audio/README.md) |
| `scripts` | [Build, run, and maintenance scripts](scripts/README.md) |

### Docs

- [docs/README.md](docs/README.md) — documentation index and reading order
- [docs/API_CLIENT.md](docs/API_CLIENT.md) — client integration (REST, WebSocket, auth, WebRTC)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — high-level architecture and pipeline
- [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md) — system view and entry points
- [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md) — connectivity and transports
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — deployment notes
- [docs/EXTENSIONS.md](docs/EXTENSIONS.md) — extensions and plugins
- [docs/FRAMEWORKS.md](docs/FRAMEWORKS.md) — framework integration
- [docs/WEBSOCKET_SERVICES.md](docs/WEBSOCKET_SERVICES.md) — WebSocket service reconnection
- [examples/voice/README.md](examples/voice/README.md) — minimal voice pipeline and config samples
- [tests/frontend/README.md](tests/frontend/README.md) — WebRTC voice client

The OpenAPI spec is generated from the codebase (`make swagger`); Swagger UI is served at `/swagger/` when available.

---

## License

This project is licensed under the [Apache License 2.0](LICENSE).

---

## Contributing

Contributions are welcome! Quick development setup:

```bash
go test ./...          # run all tests
make lint              # lint (or: ./scripts/pre-commit.sh)
make swagger           # regenerate API docs (requires swag)
make evals             # run eval scenarios (optional)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for full setup, testing, style, and pull request guidelines.
