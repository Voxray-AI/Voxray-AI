### Stress testing overview

This package contains **stress and load tests** for the voice pipeline and HTTP layer, using **mock STT/LLM/TTS services** and **in‑memory transports**. The goal is to exercise:

- **Pipeline behavior under concurrent sessions** (burst and ramp‑up patterns)
- **End‑to‑end voice flow** (audio → STT → LLM → TTS)
- **HTTP offer endpoint behavior** under connection storms
- **Goroutine / resource usage** over many sessions

All tests live in this directory and are intentionally **skipped when running with `-short`** so that CI can use `go test -short ./...` for fast runs.

### Layout

- `stress_mock_test.go`  
  - Pipeline‑level stress tests and helpers:
    - `TestMockPipeline_SingleSession`: sanity check that one mock session completes.
    - `TestMockPipeline_Stress` / `TestMockPipeline_StressRampUp`: many concurrent sessions using `RunStress` with burst vs. ramp‑up connection patterns.
    - `TestStressHarness_Realistic`: more realistic config (chunked audio, multi‑turn, per‑session mocks, mock latencies).
    - `TestMockPipeline_NoGoroutineLeak`: verifies goroutine counts do not grow unbounded.
    - Benchmarks (`BenchmarkMockPipeline_*`): report sessions/sec for different worker counts.

- `stress_harness_test.go`  
  - Implementation of the **stress harness**:
    - `StressConfig`: knobs for concurrency, duration or total sessions, per‑session timeout, mock latencies, connection pattern (burst vs. ramp‑up), and optional SLO thresholds.
    - `StressResult`: aggregate metrics (success/failure counts, percentiles, sessions/sec).
    - `RunStress`: main entry point for running a stress scenario with the pipeline.

- `http_stress_test.go`  
  - HTTP‑layer stress against a **test‑only mock offer endpoint**:
    - `TestHTTPStress_MockOfferEndpoint`: concurrent POST storm to `/test/webrtc/offer`.
    - `TestHTTPStress_MockOfferEndpoint_RampUpConcurrency`: same endpoint with staggered worker start (ramp‑up).
  - Emits detailed `t.Logf` lines including **RPS and latency percentiles** and a JUnit‑style `<testcase .../>` line for log parsing.

- `run_stress.sh`  
  - Convenience script to run the main stress tests from the repo root or this directory.

### Running the stress tests

From the **repo root**, to run everything under `tests/stress_testing`:

```bash
go test -timeout 2m ./tests/stress_testing/...
```

Run only the main stress tests (pipeline + harness + HTTP):

```bash
go test -timeout 2m -run 'TestHTTPStress_MockOfferEndpoint|TestMockPipeline_Stress|TestStressHarness_Realistic|TestMockPipeline_NoGoroutineLeak' ./tests/stress_testing/
```

Run a single focused test, for example the HTTP burst test:

```bash
go test -timeout 2m -run TestHTTPStress_MockOfferEndpoint ./tests/stress_testing/
```

Or use the helper script:

```bash
./tests/stress_testing/run_stress.sh
```

### Configuring SLOs and scenarios

The `StressConfig` type in `stress_harness_test.go` exposes several useful knobs:

- **Load shape**: `Concurrency`, `TotalSessions`, `Duration`, `ConnectionPattern`, `RampUpDuration`
- **Session behavior**: `PerSessionTimeout`, `TurnsPerSession`, `RealisticAudioChunks`, `PerSessionMocks`
- **Mock latencies**: `STTLatencyMs`, `LLMLatencyMs`, `TTSLatencyMs`
- **Transport behavior**: `TransportLatencyIn`, `TransportLatencyOut`

To enforce basic SLOs in a test, set the desired thresholds on the `StressConfig` and call:

```go
result, err := RunStress(ctx, cfg)
// ...
if err := result.AssertSLO(cfg); err != nil {
    t.Fatalf("stress SLO: %v", err)
}
```

This lets you guard on **minimum success rate**, **maximum P95 latency**, and **minimum sessions/sec** for the scenario under test.

