# Voxray-AI

Config-driven framework for building real-time AI voice agents.

STT → LLM → TTS pipelines with WebSocket and WebRTC streaming.

Built for low-latency conversational systems and industrial-scale adoption.

Voxray-AI is the Go server (`voxray-go`) that runs configurable voice pipelines and exposes **WebSocket** (`/ws`) and **SmallWebRTC** (`/webrtc/offer`) transports for **voice-ai** and **real-time-ai** agents built in **Go/golang**. For architecture and pipeline details, see [Architecture](docs/ARCHITECTURE.md).

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

## Table of contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Examples](#examples)
- [Use cases](#use-cases)
- [Roadmap](#roadmap)
- [Contributor onboarding](#contributor-onboarding)
- [Documentation](#documentation)
- [License](#license)
- [Contributing](#contributing)

## Overview

Voxray-AI is a **config-driven Go server** for building **real-time voice agents** over **WebSocket** and **WebRTC**. It wires together **speech-to-text (STT)**, **LLM**, and **text-to-speech (TTS)** providers into low-latency streaming pipelines, so you can build production-ready **voice-ai**, **voice-agents**, and **conversational-ai** systems without hand-rolling audio plumbing. Pipelines, providers, and transports are defined via JSON config, making it easy to swap services and deploy to your own infrastructure.

## Features

- **Pipelines:** Low-latency STT → LLM → TTS voice pipeline with configurable providers and models
- **Transports:** WebSocket and WebRTC (SmallWebRTC) support for real-time streaming audio
- **Providers:** Multiple STT, LLM, and TTS backends (e.g. OpenAI, Groq, Sarvam, AWS, Google, Anthropic)
- **Framework & plugins:** Plugin system for custom processors and aggregators; built as an extensible **ai-framework**
- **Config-driven:** JSON configuration for pipelines and transports; API keys via config or environment variables
- **Voice over WebRTC:** Optional CGO build for Opus encoding and WebRTC TTS audio, tuned for low-latency conversational systems

## Architecture

At a high level, Voxray-AI receives audio from Web or native clients over **WebSocket** or **WebRTC**, runs it through a configurable **STT → LLM → TTS** pipeline, and streams audio responses back over the same transport. Each stage (STT, LLM, TTS) is pluggable, so you can mix and match providers while keeping a consistent, low-latency real-time pipeline.

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

For a deeper dive into the internals and pipeline design, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md).


## Requirements

- **Go 1.25+** (see [go.mod](go.mod))
- For **voice over WebRTC (TTS)** and Opus: **CGO** enabled and a **C compiler** (e.g. `gcc`) on PATH

### C compiler (Windows)

CGO needs **gcc** on your PATH. Use one of:

- **WinLibs (winget):**  
  `winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements`  
  Restart your terminal (or add the WinLibs `mingw64\bin` folder to PATH), then run `gcc --version` to confirm.

- **MSYS2:**  
  Install [MSYS2](https://www.msys2.org/), open **MSYS2 UCRT64**, run:  
  `pacman -S mingw-w64-ucrt-x86_64-toolchain`  
  Add `C:\msys64\ucrt64\bin` (or your MSYS2 path) to PATH, then verify with `gcc --version`.

Without CGO, WebRTC TTS will report *opus encoder unavailable (build without cgo); TTS audio cannot be sent* and the server may return **503** for WebRTC offers. The server also returns **503** when at capacity (session or memory cap; see [Session capacity and admission control](#session-capacity-and-admission-control)).

## Installation

Clone the repository, then build and run as below.

### Default build (no WebRTC TTS / no Opus)

```bash
go build -o voxray ./cmd/voxray
```

Or with Make (Linux/macOS):

```bash
make build
make run
```

### Build with voice (WebRTC TTS, Opus)

Requires **CGO** and **gcc** on PATH (see [Requirements](#requirements)).

**Windows (PowerShell, from repo root):**

- Build once, then run:
  ```powershell
  .\scripts\build-voice.ps1
  .\voxray.exe -config config.json
  ```
- Or run without a separate build:
  ```powershell
  .\scripts\run-voice.ps1 -config config.json
  ```

**Linux/macOS:**

```bash
make build-voice
./voxray -config config.json
```

Or in one step:

```bash
make run-voice ARGS="-config config.json"
```

**Manual (any OS):** Set `CGO_ENABLED=1` and ensure `gcc` is on PATH, then:

```bash
CGO_ENABLED=1 go build -o voxray ./cmd/voxray
./voxray -config config.json
```

or:

```bash
CGO_ENABLED=1 go run ./cmd/voxray -config config.json
```

After a voice build, WebRTC offers succeed and TTS audio is sent over the peer connection.

## Getting Started

End-to-end steps to run the server and connect a client:

1. **Prerequisites** — Go 1.25+; for WebRTC TTS/Opus, CGO and gcc (see [Requirements](#requirements)).
2. **Build** — From repo root: `go build -o voxray ./cmd/voxray` (or `make build`). For voice/WebRTC: `.\scripts\build-voice.ps1` (Windows) or `make build-voice` (Linux/macOS).
3. **Config** — `cp config.example.json config.json` and set `api_keys` or environment variables (e.g. `OPENAI_API_KEY`). See [examples/voice/README.md](examples/voice/README.md) for provider/model examples.
4. **Run** — `./voxray -config config.json` (Windows: `.\voxray.exe -config config.json`).
5. **Connect** — WebSocket: `http://localhost:8080/ws`. WebRTC: `http://localhost:8080/webrtc/offer`.
6. **Try the WebRTC client** — From repo root: `cd tests/frontend && python -m http.server 3000`, then open **http://localhost:3000/webrtc-voice.html** in your browser. Set Server URL to `http://localhost:8080` and click Start. See [tests/frontend/README.md](tests/frontend/README.md) for details.

## Configuration

Configuration is JSON. Copy [config.example.json](config.example.json) to `config.json` and set providers, models, and API keys. Unknown keys (e.g. `_comment`) are ignored; keys can often be overridden via environment variables.

- **[config.example.json](config.example.json)** — structure and available options
- **[examples/voice/README.md](examples/voice/README.md)** — provider/model examples, `transport: "both"`, `webrtc_ice_servers`
- **[tests/frontend/README.md](tests/frontend/README.md)** — WebRTC voice client usage

### Conversation recording to S3

Voxray can record the **entire mixed conversation audio per session** and upload it **asynchronously** to an **S3 bucket** using a simple worker pool.

- **Config block (`config.json`)**:
  ```json
  "recording": {
    "enable": true,
    "bucket": "your-recordings-bucket",
    "base_path": "recordings/",
    "format": "wav",
    "worker_count": 4
  }
  ```
  - **`enable`**: turn recording on for all sessions.
  - **`bucket`**: S3 bucket name where recordings are stored.
  - **`base_path`**: key prefix inside the bucket (default `recordings/`).
  - **`format`**: file format/extension (currently `wav` for 16‑bit PCM mono).
  - **`worker_count`**: number of background uploader workers (thread pool size).

- **Environment overrides** (optional):
  - `VOXRAY_RECORDING_ENABLE=true`
  - `VOXRAY_RECORDING_BUCKET=your-recordings-bucket`
  - `VOXRAY_RECORDING_BASE_PATH=recordings/`
  - `VOXRAY_RECORDING_FORMAT=wav`
  - `VOXRAY_RECORDING_WORKER_COUNT=4`

Each call/session is written to a local WAV file and, when the session ends, a background job enqueues an S3 upload using the configured bucket and base path with a key like:

```text
<base_path>/yyyy/mm/dd/<session-id>.wav
```

AWS credentials and region are resolved via the standard AWS SDK v2 configuration (environment variables, shared config/credentials files, IAM role, etc.).

### Transcript logging to Postgres/MySQL

Voxray can persist **per-message text transcripts** (user and assistant) for each session to a **Postgres** or **MySQL** database.

- **Config block (`config.json`)**:
  ```json
  "transcripts": {
    "enable": true,
    "driver": "postgres",
    "dsn": "postgres://user:pass@localhost:5432/voxray?sslmode=disable",
    "table_name": "call_transcripts"
  }
  ```
  or:
  ```json
  "transcripts": {
    "enable": true,
    "driver": "mysql",
    "dsn": "user:pass@tcp(localhost:3306)/voxray?parseTime=true",
    "table_name": "call_transcripts"
  }
  ```
- **Expected schema** (Postgres example):
  ```sql
  CREATE TABLE call_transcripts (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL,
    role        TEXT NOT NULL, -- "user" or "assistant"
    text        TEXT NOT NULL,
    seq         BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  ```

Environment overrides:

- `VOXRAY_TRANSCRIPTS_ENABLE=true`
- `VOXRAY_TRANSCRIPTS_DRIVER=postgres` (or `mysql`)
- `VOXRAY_TRANSCRIPTS_DSN=...`
- `VOXRAY_TRANSCRIPTS_TABLE=call_transcripts`

### Session capacity and admission control

You can limit concurrent voice sessions and reject new connections when limits are reached (HTTP 503 and `{"error":"server at capacity"}`).

- **Fixed cap**: Set `max_concurrent_sessions` (integer). When the limit is reached, new connections are rejected.
- **Memory-based caps** (optional):
  - `session_cap_memory_percent`: reject when system memory used % is at or above this (e.g. 80). Uses hysteresis so acceptance resumes when usage drops below (threshold − `session_cap_memory_hysteresis_percent`).
  - `session_cap_process_memory_mb`: reject when process heap (MB) is at or above this.
  - `session_cap_memory_hysteresis_percent`: default 5; only applies when `session_cap_memory_percent` is set.
- **Environment overrides**: `VOXRAY_SESSION_CAP_MEMORY_PERCENT`, `VOXRAY_SESSION_CAP_PROCESS_MEMORY_MB`, `VOXRAY_SESSION_CAP_MEMORY_HYSTERESIS_PERCENT`.
- **Scope**: Applies to WebSocket (`/ws`), WebRTC (`/webrtc/offer`), telephony (`/telephony/ws`), runner (`/start`, `/sessions/{id}/api/offer`), and Daily flows.

See [config.example.json](config.example.json) for the `_comment_cap` and the full list of keys.

### Prometheus metrics

- **Endpoint**: the server exposes a Prometheus-compatible scrape endpoint at `/metrics` on the same host/port as `/ws` and `/webrtc/offer`.
- **Config flag**: metrics collection is controlled by `metrics_enabled` in `config.json`:
  - `"metrics_enabled": true` (default when omitted) enables recording of HTTP, WebRTC, STT, LLM, TTS, and session capacity metrics and exports them at `/metrics`.
  - `"metrics_enabled": false` disables recording; `/metrics` remains reachable but returns `204 No Content` so Prometheus scrape configs do not break.
- **Metric areas**: HTTP, WebRTC, STT, LLM, TTS, recording queue, and **session capacity** (`active_sessions` gauge, `sessions_rejected_total` counter with label `reason`: `fixed_cap`, `memory_system`, `memory_process`).
- **Scalability**: metrics are process-local (per instance); Prometheus aggregates across instances using its own `instance`/`pod` labels, and high-cardinality labels like `session_id` are safely handled via hashing/sampling.

You can set the config path with the `-config` flag or the `VOXRAY_CONFIG` environment variable.

## Examples

- **Minimal voice pipeline**: see [examples/voice/README.md](examples/voice/README.md) for sample `config.json` files that wire STT, LLM, and TTS providers into end-to-end voice pipelines.
- **WebRTC voice client**: see [tests/frontend/README.md](tests/frontend/README.md) for a browser-based WebRTC client that connects to Voxray-AI and streams audio in real time.

Example `config.json` snippet for a simple voice agent:

```json
{
  "transport": "both",
  "stt": { "provider": "openai", "model": "gpt-4o-mini-transcribe" },
  "llm": { "provider": "openai", "model": "gpt-4.1-mini" },
  "tts": { "provider": "openai", "voice": "alloy" }
}
```

## Use cases

- **AI call centers / IVR**: conversational-ai agents that handle inbound and outbound calls with low latency.
- **In-app voice copilots**: embed real-time voice-agents inside SaaS or productivity apps using WebSocket or WebRTC.
- **Operations and support bots**: agentic voicebots for support, ops, and internal tooling that run in your own infra.
- **Realtime monitoring and control**: voice interfaces for dashboards, observability tools, and control systems.
- **On-prem / VPC assistants**: self-hosted voice-ai stacks where data must stay within your cloud or datacenter.

## Roadmap

- More built-in STT/LLM/TTS providers and opinionated presets for common stacks.
- Deeper observability, tracing, and debugging tools for real-time pipelines.
- Additional starter templates and example agents for popular voice-agent scenarios.
- Expanded documentation on scaling, deployment patterns, and production hardening.

## Contributor onboarding

- **[docs/skills/README.md](docs/skills/README.md)** — Skills map, reading order, and role-based onboarding for new contributors (Core Stack, AI/ML, Infrastructure, DevOps, Key Concepts, Concurrency).

## Documentation

### Repository layout (package READMEs)

- [pkg/pipeline/README.md](pkg/pipeline/README.md) — pipeline, runner, source/sink, task, registry
- [pkg/transport/README.md](pkg/transport/README.md) — WebSocket, WebRTC, in-memory transports
- [pkg/services/README.md](pkg/services/README.md) — LLM, STT, TTS interfaces and provider factory
- [pkg/recording/README.md](pkg/recording/README.md) — conversation recording and S3 upload
- [pkg/metrics/README.md](pkg/metrics/README.md) — Prometheus metrics
- [pkg/config/README.md](pkg/config/README.md) — configuration and env overrides
- [pkg/processors/README.md](pkg/processors/README.md) — voice, echo, filters, aggregators
- [pkg/runner/README.md](pkg/runner/README.md) — session store and runner args
- [pkg/utils/README.md](pkg/utils/README.md) — backoff, notifier, sentence, aggregators
- [pkg/frames/README.md](pkg/frames/README.md) — frame types and serialization
- [pkg/audio/README.md](pkg/audio/README.md) — VAD, turn detection, codecs, resample
- [scripts/README.md](scripts/README.md) — build, run, and maintenance scripts
- [docs/README.md](docs/README.md) — documentation index and reading order

### Docs and examples

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — high-level architecture and pipeline
- [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md) — system view and entry points
- [examples/voice/README.md](examples/voice/README.md) — minimal voice pipeline and config samples
- [tests/frontend/README.md](tests/frontend/README.md) — WebRTC voice client
- [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md) — connectivity and transports
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — deployment notes
- [docs/EXTENSIONS.md](docs/EXTENSIONS.md) — extensions and plugins
- [docs/FRAMEWORKS.md](docs/FRAMEWORKS.md) — framework integration
- [docs/WEBSOCKET_SERVICES.md](docs/WEBSOCKET_SERVICES.md) — WebSocket service reconnection

## License

License: see repository.

## Contributing

Contributions are welcome. Open an issue or pull request to get started.
