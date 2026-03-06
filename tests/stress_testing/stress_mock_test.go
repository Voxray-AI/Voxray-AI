// Package stress_testing contains stress tests for the voice pipeline using mock STT, LLM, TTS
// and in-memory transport. Same flow as web/index.html: mic → STT → LLM → TTS → speaker.
package stress_testing

import (
	"context"
	"runtime"
	"testing"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services/mock"
	"voxray-go/pkg/transport/memory"
)

// minAudioBytes is the minimum audio to trigger STT (voice.MinSTTBufferMs = 500ms at 16kHz mono 16-bit).
const minAudioBytes = 16000

// runMockSession runs one pipeline session: in-memory transport + mock STT/LLM/TTS, same chain as WebRTC flow.
// Pushes synthetic audio, reads until one TTSAudioRawFrame or timeout. Returns true if TTS was received.
func runMockSession(ctx context.Context, mockSTT *mock.STT, mockLLM *mock.LLM, mockTTS *mock.TTS) (gotTTS bool, err error) {
	tr := memory.NewTransport()
	pl := pipeline.New()
	pl.Add(voice.NewSTTProcessor("stt", mockSTT, 16000, 1))
	pl.Add(voice.NewLLMProcessorWithSystemPrompt("llm", mockLLM, "You are a helpful voice assistant. Reply briefly."))
	pl.Add(voice.NewTTSProcessor("tts", mockTTS, 24000))
	pl.Add(pipeline.NewSink("sink", tr.Output()))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := pipeline.NewRunner(pl, tr, frames.NewStartFrame())
	go func() {
		_ = runner.Run(runCtx)
	}()

	// Enough audio to exceed STT buffer (500ms at 16kHz mono)
	pcm := make([]byte, minAudioBytes)
	audioFrame := frames.NewAudioRawFrame(pcm, 16000, 1, 0)
	if !tr.SendInput(ctx, audioFrame) {
		return false, ctx.Err()
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline:
			cancel()
			return gotTTS, nil
		case f, ok := <-tr.Out():
			if !ok {
				return gotTTS, nil
			}
			switch f.(type) {
			case *frames.TTSAudioRawFrame:
				gotTTS = true
				cancel()
				<-runner.Done()
				return true, nil
			case *frames.ErrorFrame:
				cancel()
				<-runner.Done()
				return false, nil
			}
		}
	}
}

// TestMockPipeline_SingleSession verifies one session completes: synthetic audio → mock STT → mock LLM → mock TTS → output.
func TestMockPipeline_SingleSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mockSTT := mock.NewSTTWithTranscript("hello")
	mockLLM := mock.NewLLMWithResponse("hi there")
	mockTTS := mock.NewTTS()

	gotTTS, err := runMockSession(ctx, mockSTT, mockLLM, mockTTS)
	if err != nil {
		t.Fatalf("session error: %v", err)
	}
	if !gotTTS {
		t.Fatal("expected at least one TTSAudioRawFrame from mock pipeline")
	}
}

// TestMockPipeline_Stress runs many concurrent sessions (same pipeline flow as web/index.html) using the harness.
// Per-session mocks avoid contention so sessions complete reliably and the test finishes within timeout.
// For a full stress run use: go test -timeout 2m -run TestMockPipeline_Stress and increase Duration or TotalSessions.
func TestMockPipeline_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := DefaultStressConfig()
	cfg.Concurrency = 50
	cfg.Duration = 8 * time.Second
	cfg.PerSessionTimeout = 4 * time.Second
	cfg.PerSessionMocks = true  // avoid shared-mock contention so test doesn't hang
	cfg.TotalSessions = 15     // bounded run so test always finishes

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := RunStress(ctx, cfg)
	if err != nil {
		t.Fatalf("RunStress: %v", err)
	}
	if result.TotalSessions == 0 {
		t.Fatal("expected at least one session to run")
	}
	cfg.MinSuccessRate = 0.5
	cfg.MaxP95LatencyMs = 5000 // 5s max P95 for this bounded run
	if err := result.AssertSLO(cfg); err != nil {
		t.Errorf("stress SLO: %v", err)
	}
	t.Logf("stress (concurrency=%d): %d/%d success, %d failures, p95=%.0f ms, %.1f sessions/sec",
		cfg.Concurrency, result.SuccessCount, result.TotalSessions, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
	t.Logf(`<testcase name="%s" type="pipeline_burst" concurrency="%d" totalSessions="%d" success="%d" failed="%d" p95_ms="%.0f" sessions_per_sec="%.1f"/>`,
		t.Name(), cfg.Concurrency, result.TotalSessions, result.SuccessCount, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
}

// TestMockPipeline_StressRampUp uses the same harness but exercises the ConnectionRampUp
// pattern to stagger session starts over a short ramp duration.
func TestMockPipeline_StressRampUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ramp-up stress test in short mode")
	}

	cfg := DefaultStressConfig()
	cfg.Concurrency = 50
	cfg.Duration = 0
	cfg.PerSessionTimeout = 4 * time.Second
	cfg.PerSessionMocks = true
	cfg.TotalSessions = 15
	cfg.TurnsPerSession = 1
	cfg.ConnectionPattern = ConnectionRampUp
	cfg.RampUpDuration = 3 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := RunStress(ctx, cfg)
	if err != nil {
		t.Fatalf("RunStress: %v", err)
	}
	if result.TotalSessions == 0 {
		t.Fatal("expected at least one session to run")
	}

	cfg.MinSuccessRate = 0.5
	cfg.MaxP95LatencyMs = 5000 // 5s max P95 for this bounded run
	if err := result.AssertSLO(cfg); err != nil {
		t.Errorf("ramp-up stress SLO: %v", err)
	}
	t.Logf("ramp-up stress (concurrency=%d, ramp=%s): %d/%d success, %d failures, p95=%.0f ms, %.1f sessions/sec",
		cfg.Concurrency, cfg.RampUpDuration, result.SuccessCount, result.TotalSessions, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
	t.Logf(`<testcase name="%s" type="pipeline_ramp" concurrency="%d" rampUp="%s" totalSessions="%d" success="%d" failed="%d" p95_ms="%.0f" sessions_per_sec="%.1f"/>`,
		t.Name(), cfg.Concurrency, cfg.RampUpDuration.String(), result.TotalSessions, result.SuccessCount, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
}

// TestStressHarness_Realistic runs the harness with real-life options: chunked audio, per-session mocks, multi-turn; asserts on success rate and optional p99.
// Uses TotalSessions so the run is bounded and always finishes; use -timeout 60s or more when running with other tests.
func TestStressHarness_Realistic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping realistic stress test in short mode")
	}

	cfg := RealisticStressConfig()
	cfg.Concurrency = 20
	cfg.TotalSessions = 15 // bounded run so test always finishes
	cfg.Duration = 0
	cfg.PerSessionTimeout = 5 * time.Second
	cfg.TurnsPerSession = 2
	cfg.RealisticAudioChunks = true
	cfg.PerSessionMocks = true

	// Allow enough time: 15 sessions with 2 turns and mock latencies can take ~30s under load.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := RunStress(ctx, cfg)
	if err != nil {
		t.Fatalf("RunStress: %v", err)
	}
	if result.TotalSessions == 0 {
		t.Fatal("expected at least one session to run")
	}
	cfg.MinSuccessRate = 0.5
	cfg.MaxP95LatencyMs = 5000 // 5s max P95 for short realistic run
	cfg.MinSessionsPerSec = 0.5
	if err := result.AssertSLO(cfg); err != nil {
		t.Errorf("realistic stress SLO: %v", err)
	}
	if result.LatencySamples > 0 && result.LatencyMsP99 > 0 && result.LatencyMsP99 < 1e6 {
		t.Logf("realistic stress: p99 latency %.0f ms", result.LatencyMsP99)
	}
	t.Logf("realistic stress (concurrency=%d): %d/%d success, %d failures, p95=%.0f ms, %.1f sessions/sec",
		cfg.Concurrency, result.SuccessCount, result.TotalSessions, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
	t.Logf(`<testcase name="%s" type="pipeline_realistic" concurrency="%d" totalSessions="%d" success="%d" failed="%d" p95_ms="%.0f" p99_ms="%.0f" sessions_per_sec="%.1f"/>`,
		t.Name(), cfg.Concurrency, result.TotalSessions, result.SuccessCount, result.FailureCount, result.LatencyMsP95, result.LatencyMsP99, result.SessionsPerSec)
}

// TestMockPipeline_NoGoroutineLeak runs a fixed number of sessions and checks that goroutine count
// does not grow unbounded (within a small allowed delta to account for test harness).
func TestMockPipeline_NoGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine leak check in short mode")
	}

	// Allow a small delta: some goroutines may still be shutting down when we sample.
	const maxGoroutineDelta = 15

	runtime.GC()
	before := runtime.NumGoroutine()

	cfg := DefaultStressConfig()
	cfg.Concurrency = 5
	cfg.TotalSessions = 10
	cfg.PerSessionMocks = true
	cfg.PerSessionTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	_, err := RunStress(ctx, cfg)
	cancel()
	if err != nil {
		t.Fatalf("RunStress: %v", err)
	}

	runtime.GC()
	// Give goroutines a moment to exit after context cancel.
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()
	delta := after - before
	if delta > maxGoroutineDelta {
		t.Errorf("goroutine leak check: goroutines before=%d after=%d (delta %d > %d)", before, after, delta, maxGoroutineDelta)
	}
	t.Logf("goroutine leak check: before=%d after=%d delta=%d", before, after, delta)
}

// BenchmarkMockPipeline_10Workers runs 10 concurrent sessions per iteration; each iteration runs until 20 sessions complete.
func BenchmarkMockPipeline_10Workers(b *testing.B) {
	ctx := context.Background()
	cfg := DefaultStressConfig()
	cfg.Concurrency = 10
	cfg.PerSessionMocks = true
	cfg.TotalSessions = 20
	cfg.Duration = 0
	cfg.PerSessionTimeout = 10 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := RunStress(runCtx, cfg)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(result.SessionsPerSec, "sessions/sec")
		_ = result
	}
}

// BenchmarkMockPipeline_50Workers runs 50 concurrent sessions per iteration; each iteration runs until 100 sessions complete.
func BenchmarkMockPipeline_50Workers(b *testing.B) {
	ctx := context.Background()
	cfg := DefaultStressConfig()
	cfg.Concurrency = 50
	cfg.PerSessionMocks = true
	cfg.TotalSessions = 100
	cfg.Duration = 0
	cfg.PerSessionTimeout = 15 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		result, err := RunStress(runCtx, cfg)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(result.SessionsPerSec, "sessions/sec")
		_ = result
	}
}
