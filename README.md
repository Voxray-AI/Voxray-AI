# Voxray

**Real-time voice pipeline server (STT → LLM → TTS) with WebSocket and WebRTC.**

Voxray is the Go server (`voxray-go`) that runs configurable voice pipelines and exposes **WebSocket** (`/ws`) and **SmallWebRTC** (`/webrtc/offer`) transports. For architecture and pipeline details, see [Architecture](docs/ARCHITECTURE.md).

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

## Table of contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Documentation](#documentation)
- [License](#license)
- [Contributing](#contributing)

## Features

- **Voice pipeline:** STT → LLM → TTS with configurable providers and models
- **Transports:** WebSocket and WebRTC (SmallWebRTC) support
- **Multiple providers** for STT, LLM, and TTS (e.g. OpenAI, Groq, Sarvam, AWS, Google, Anthropic)
- **Plugin system** for custom processors and aggregators
- **Config-driven** setup via JSON; API keys via config or environment
- **Optional CGO build** for Opus encoding and WebRTC TTS audio

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

Without CGO, WebRTC TTS will report *opus encoder unavailable (build without cgo); TTS audio cannot be sent* and the server may return **503** for WebRTC offers.

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

## Quick start

1. Copy the example config and set your API keys (or use env vars):
   ```bash
   cp config.example.json config.json
   ```
2. Run the server:
   ```bash
   ./voxray -config config.json
   ```
   On Windows: `.\voxray.exe -config config.json`
3. **Endpoints:** WebSocket at `/ws`, WebRTC at `/webrtc/offer`.

For sample configs and provider/model examples see [examples/voice/README.md](examples/voice/README.md). For a WebRTC voice client see [tests/frontend/README.md](tests/frontend/README.md).

## Configuration

Configuration is JSON. Copy [config.example.json](config.example.json) to `config.json` and set providers, models, and API keys. Unknown keys (e.g. `_comment`) are ignored; keys can often be overridden via environment variables.

- **[config.example.json](config.example.json)** — structure and available options
- **[examples/voice/README.md](examples/voice/README.md)** — provider/model examples, `transport: "both"`, `webrtc_ice_servers`
- **[tests/frontend/README.md](tests/frontend/README.md)** — WebRTC voice client usage

### Prometheus metrics

- **Endpoint**: the server exposes a Prometheus-compatible scrape endpoint at `/metrics` on the same host/port as `/ws` and `/webrtc/offer`.
- **Config flag**: metrics collection is controlled by `metrics_enabled` in `config.json`:
  - `"metrics_enabled": true` (default when omitted) enables recording of HTTP, WebRTC, STT, LLM, and TTS metrics and exports them at `/metrics`.
  - `"metrics_enabled": false` disables recording; `/metrics` remains reachable but returns `204 No Content` so Prometheus scrape configs do not break.
- **Scalability**: metrics are process-local (per instance); Prometheus aggregates across instances using its own `instance`/`pod` labels, and high-cardinality labels like `session_id` are safely handled via hashing/sampling.

You can set the config path with the `-config` flag or the `VOXRAY_CONFIG` environment variable.

## Documentation

- [docs/README.md](docs/README.md) — documentation index and reading order
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
