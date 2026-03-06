### Test layout and conventions

- **Unit tests**
  - Live under `tests/pkg/**` as external tests (`package <pkg>_test`, import `voxray-go/pkg/...`). Run with `go test ./tests/pkg/...`.
  - Build/smoke tests for `cmd/`, `examples/`, and `docs` live under `tests/cmd/**`, `tests/examples/**`, and `tests/docs/` and ensure those packages compile (e.g. via `go build`).
  - A few tests remain in `pkg/**` where they rely on unexported APIs (e.g. `pkg/audio/turn`, `pkg/audio/vad`, `pkg/processors/voice`, `pkg/services/anthropic`, `pkg/services/google`).
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
  - The Groq voice E2E pipeline test (`tests/pkg/pipeline/groq_voice_e2e_test.go`) expects a small spoken-phrase WAV file at `tests/testdata/hello.wav` and a valid Groq API key (`GROQ_API_KEY` or `config.json`); it is skipped automatically if these are not present.

- **Config (provider-agnostic, multi-provider)**
  - Optional per-task providers: `stt_provider`, `llm_provider`, `tts_provider` override the default `provider` for that task only (e.g. OpenAI for STT, Groq for LLM/TTS, Sarvam for STT/TTS).
  - Optional per-task model/voice: `stt_model`, `tts_model`, `tts_voice`; `model` is the chat/LLM model. Omitted values use each provider’s defaults.

Example Groq-centric config:

```json
{
  "host": "localhost",
  "port": 8080,
  "provider": "groq",
  "stt_provider": "openai",
  "llm_provider": "groq",
  "tts_provider": "groq",
  "model": "llama-3.1-8b-instant",
  "stt_model": "whisper-1",
  "tts_model": "canopylabs/orpheus-v1-english",
  "tts_voice": "alloy",
  "plugins": ["echo"],
  "api_keys": { "openai": "<openai_key>", "groq": "<groq_key>" }
}
```

Example Sarvam-centric config:

```json
{
  "host": "localhost",
  "port": 8080,
  "provider": "sarvam",
  "stt_provider": "sarvam",
  "llm_provider": "openai",
  "tts_provider": "sarvam",
  "model": "gpt-3.5-turbo",
  "stt_model": "saarika:v2.5",
  "tts_model": "bulbul:v2",
  "tts_voice": "anushka",
  "plugins": ["echo"],
  "api_keys": {
    "openai": "<openai_key>",
    "sarvam": "<sarvam_api_key>"
  }
}
```

Example WebRTC voice (Sarvam STT/TTS + Groq LLM):

```json
{
  "host": "localhost",
  "port": 8080,
  "transport": "both",
  "webrtc_ice_servers": ["stun:stun.l.google.com:19302"],
  "provider": "groq",
  "stt_provider": "sarvam",
  "llm_provider": "groq",
  "tts_provider": "sarvam",
  "model": "llama-3.1-8b-instant",
  "stt_model": "saarika:v2.5",
  "tts_model": "bulbul:v2",
  "tts_voice": "anushka",
  "plugins": ["echo"],
  "api_keys": {
    "groq": "<groq_key>",
    "sarvam": "<sarvam_api_key>"
  }
}
```

### Live WebRTC voice (Sarvam + Groq)

To run the **live** WebRTC voice integration (mic in, TTS out) in the browser:

1. **Config** — Use a config with `transport: "both"`, Sarvam STT/TTS, Groq LLM, and API keys (e.g. the "Example WebRTC voice" above, or your `config.json` with `api_keys` for `groq` and `sarvam`).
2. **Run the server** — From the repo root (so `web/` is served when present):
   ```bash
   go run ./cmd/voxray --config config.json
   ```
3. **Use the browser client** — Open `http://localhost:<port>/` (e.g. `http://localhost:8090/`). Click **Connect**, then **Start mic**. Speak into your computer mic; TTS audio is played when received from the pipeline.

Notes:

- When the `web/` directory exists, the server serves the browser client at `/` and the WebRTC signaling endpoint at `/webrtc/offer`.
- Transports: WebSocket (`/ws`) and SmallWebRTC (RTP/Opus audio tracks via `/webrtc/offer`).

### Stress tests and CI

Stress tests live in `tests/stress_testing/` (`http_stress_test.go`, `stress_harness_test.go`, `stress_mock_test.go`, `run_stress.sh`). They are skipped when running with the `-short` flag so CI can use `go test -short ./...` for fast runs.

**Run all stress tests:**

```bash
go test -timeout 2m ./tests/stress_testing/...
```

**Run only the four main stress tests:**

```bash
go test -timeout 2m -run 'TestHTTPStress_MockOfferEndpoint|TestMockPipeline_Stress|TestStressHarness_Realistic|TestMockPipeline_NoGoroutineLeak' ./tests/stress_testing/
```

**Run a single stress test (e.g. HTTP layer):**

```bash
go test -timeout 2m -run TestHTTPStress_MockOfferEndpoint ./tests/stress_testing/
```

**Run via script (from repo root or tests/stress_testing):**

```bash
./tests/stress_testing/run_stress.sh
```

Stress tests assert on success rate and optional SLOs (min success rate, max P95 latency, min sessions/sec) via `StressResult.AssertSLO(cfg)`. To enforce stricter thresholds in CI, set `StressConfig.MinSuccessRate`, `MaxP95LatencyMs`, and/or `MinSessionsPerSec` in the test and call `result.AssertSLO(cfg)` after `RunStress`.
