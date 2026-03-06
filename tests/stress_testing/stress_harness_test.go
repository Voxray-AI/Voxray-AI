// Package stress_testing: stress harness for the voice pipeline with configurable
// load, real-life options (chunked audio, multi-turn), and metrics.
package stress_testing

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services/mock"
	"voxray-go/pkg/transport/memory"
)

// ConnectionPattern defines how sessions are started (burst = all at once, ramp = staggered).
type ConnectionPattern int

const (
	ConnectionBurst ConnectionPattern = iota
	ConnectionRampUp
)

// StressConfig configures a stress run: concurrency, duration/count, mock latencies,
// real-life options (chunked audio, multi-turn), and timeouts.
type StressConfig struct {
	// Concurrency is the number of concurrent workers (sessions).
	Concurrency int
	// TotalSessions is the total number of sessions to run (0 = use Duration instead).
	TotalSessions int
	// Duration limits the stress run by time (0 = use TotalSessions).
	Duration time.Duration
	// PerSessionTimeout is the max time allowed per session to complete (e.g. one or N turns).
	PerSessionTimeout time.Duration

	// Mock latencies (optional). When set, mocks sleep before responding to simulate real APIs.
	STTLatencyMs int
	LLMLatencyMs int
	TTSLatencyMs int

	// RealisticAudioChunks: if true, send audio as 20ms chunks (640 bytes at 16kHz mono) until ≥500ms.
	RealisticAudioChunks bool
	// TurnsPerSession: number of user→bot exchanges per session (1 = single shot).
	TurnsPerSession int
	// PerSessionMocks: if true, each session gets its own STT/LLM/TTS mock instances (default true for realistic).
	PerSessionMocks bool
	// ConnectionPattern: burst (all start at once) or ramp-up (staggered).
	ConnectionPattern ConnectionPattern
	// RampUpDuration: when ConnectionRampUp, spread session starts over this duration.
	RampUpDuration time.Duration
	// TransportLatencyIn simulates network RTT on ingress (delay before each SendInput). Optional.
	TransportLatencyIn time.Duration
	// TransportLatencyOut simulates network RTT on egress (delay after each read from Out). Optional.
	TransportLatencyOut time.Duration

	// Optional SLO-style assertions (when set, RunStress does not enforce; call AssertSLO on result to fail if thresholds are missed).
	// MinSuccessRate: minimum success count/total (e.g. 0.5 for 50%). 0 = disabled.
	MinSuccessRate float64
	// MaxP95LatencyMs: maximum allowed P95 latency in ms. 0 = disabled.
	MaxP95LatencyMs float64
	// MinSessionsPerSec: minimum sessions per second. 0 = disabled.
	MinSessionsPerSec float64
}

// DefaultStressConfig returns a config suitable for quick stress (shared mocks, single turn, single blob).
func DefaultStressConfig() StressConfig {
	return StressConfig{
		Concurrency:          500,
		TotalSessions:        0,
		Duration:             30 * time.Second,
		PerSessionTimeout:    15 * time.Second,
		PerSessionMocks:      true,
		TurnsPerSession:      1,
		RealisticAudioChunks: false,
		ConnectionPattern:    ConnectionBurst,
		RampUpDuration:       2 * time.Second,
	}
}

// RealisticStressConfig returns a config that mirrors production: chunked audio, optional Turn, multi-turn, per-session mocks, latencies.
func RealisticStressConfig() StressConfig {
	c := DefaultStressConfig()
	c.RealisticAudioChunks = true
	c.TurnsPerSession = 3
	c.PerSessionMocks = true
	c.STTLatencyMs = 100
	c.LLMLatencyMs = 150
	c.TTSLatencyMs = 80
	return c
}

// StressResult holds aggregates and percentiles from a stress run.
type StressResult struct {
	SuccessCount   int
	FailureCount   int
	TotalSessions  int
	DurationSec    float64
	LatencyMsMin   float64
	LatencyMsMax   float64
	LatencyMsMean  float64
	LatencyMsP50   float64
	LatencyMsP95   float64
	LatencyMsP99   float64
	SessionsPerSec float64
	// LatencySamples is the number of successful sessions used for percentile computation.
	LatencySamples int
}

// SuccessRate returns success count / total (0 if total 0).
func (r *StressResult) SuccessRate() float64 {
	if r.TotalSessions == 0 {
		return 0
	}
	return float64(r.SuccessCount) / float64(r.TotalSessions)
}

// AssertSLO checks optional SLO thresholds (min success rate, max P95 latency, min sessions/sec).
// Only thresholds set to a non-zero value in cfg are checked. Returns an error describing the first failure.
func (r *StressResult) AssertSLO(cfg StressConfig) error {
	if cfg.MinSuccessRate > 0 && r.SuccessRate() < cfg.MinSuccessRate {
		return fmt.Errorf("SLO: success rate %.2f below minimum %.2f", r.SuccessRate(), cfg.MinSuccessRate)
	}
	if cfg.MaxP95LatencyMs > 0 && r.LatencySamples > 0 && r.LatencyMsP95 > cfg.MaxP95LatencyMs {
		return fmt.Errorf("SLO: P95 latency %.0f ms exceeds maximum %.0f ms", r.LatencyMsP95, cfg.MaxP95LatencyMs)
	}
	if cfg.MinSessionsPerSec > 0 && r.SessionsPerSec < cfg.MinSessionsPerSec {
		return fmt.Errorf("SLO: sessions/sec %.1f below minimum %.1f", r.SessionsPerSec, cfg.MinSessionsPerSec)
	}
	return nil
}

// 20ms at 16kHz mono 16-bit = 320 samples = 640 bytes.
const chunkSize20ms = 640
const stressSampleRate = 16000
const stressChannels = 1

// runOneSession runs a single pipeline session and returns success, duration in ms, and error.
// If cfg.PerSessionMocks is true, mocks are created inside this function; otherwise sharedSTT/LLM/TTS are used.
func runOneSession(ctx context.Context, cfg StressConfig, sharedSTT *mock.STT, sharedLLM *mock.LLM, sharedTTS *mock.TTS) (success bool, durationMs float64, err error) {
	var sttSvc *mock.STT
	var llmSvc *mock.LLM
	var ttsSvc *mock.TTS
	if cfg.PerSessionMocks {
		sttSvc = mock.NewSTTWithTranscript("hello")
		sttSvc.Latency = time.Duration(cfg.STTLatencyMs) * time.Millisecond
		llmSvc = mock.NewLLMWithResponse("hi there")
		llmSvc.Latency = time.Duration(cfg.LLMLatencyMs) * time.Millisecond
		ttsSvc = mock.NewTTS()
		ttsSvc.Latency = time.Duration(cfg.TTSLatencyMs) * time.Millisecond
	} else {
		sttSvc = sharedSTT
		llmSvc = sharedLLM
		ttsSvc = sharedTTS
	}

	tr := memory.NewTransport()
	pl := pipeline.New()
	pl.Add(voice.NewSTTProcessor("stt", sttSvc, stressSampleRate, stressChannels))
	pl.Add(voice.NewLLMProcessorWithSystemPrompt("llm", llmSvc, "You are a helpful voice assistant. Reply briefly."))
	pl.Add(voice.NewTTSProcessor("tts", ttsSvc, 24000))
	pl.Add(pipeline.NewSink("sink", tr.Output()))

	runCtx, cancel := context.WithTimeout(ctx, cfg.PerSessionTimeout)
	defer cancel()

	runner := pipeline.NewRunner(pl, tr, frames.NewStartFrame())
	go func() {
		_ = runner.Run(runCtx)
	}()

	start := time.Now()
	sessionDeadline := start.Add(cfg.PerSessionTimeout)
	turnsDone := 0
	for turnIdx := 0; turnIdx < cfg.TurnsPerSession; turnIdx++ {
		if runCtx.Err() != nil {
			return false, time.Since(start).Seconds() * 1000, runCtx.Err()
		}
		if time.Now().After(sessionDeadline) {
			return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
		}
		if cfg.RealisticAudioChunks {
			// Send 20ms chunks until we've sent at least 500ms (minAudioBytes).
			sent := 0
			for sent < minAudioBytes {
				chunk := make([]byte, chunkSize20ms)
				if sent+chunkSize20ms > minAudioBytes {
					chunk = make([]byte, minAudioBytes-sent)
				}
				audioFrame := frames.NewAudioRawFrame(chunk, stressSampleRate, stressChannels, 0)
				if cfg.TransportLatencyIn > 0 {
					select {
					case <-runCtx.Done():
						return false, time.Since(start).Seconds() * 1000, runCtx.Err()
					case <-time.After(cfg.TransportLatencyIn):
					}
				}
				if !tr.SendInput(runCtx, audioFrame) {
					return false, time.Since(start).Seconds() * 1000, runCtx.Err()
				}
				sent += len(chunk)
			}
		} else {
			pcm := make([]byte, minAudioBytes)
			audioFrame := frames.NewAudioRawFrame(pcm, stressSampleRate, stressChannels, 0)
			if cfg.TransportLatencyIn > 0 {
				select {
				case <-runCtx.Done():
					return false, time.Since(start).Seconds() * 1000, runCtx.Err()
				case <-time.After(cfg.TransportLatencyIn):
				}
			}
			if !tr.SendInput(runCtx, audioFrame) {
				return false, time.Since(start).Seconds() * 1000, runCtx.Err()
			}
		}

		remaining := time.Until(sessionDeadline)
		if remaining <= 0 {
			return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
		}
		deadline := time.After(remaining)
		gotTTS := false
		for !gotTTS {
			if runCtx.Err() != nil {
				return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
			}
			if time.Now().After(sessionDeadline) {
				return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
			}
			select {
			case <-runCtx.Done():
				return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
			case <-ctx.Done():
				return turnsDone > 0, time.Since(start).Seconds() * 1000, ctx.Err()
			case <-deadline:
				return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
			case f, ok := <-tr.Out():
				if !ok {
					return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
				}
				if cfg.TransportLatencyOut > 0 {
					select {
					case <-runCtx.Done():
						return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
					case <-time.After(cfg.TransportLatencyOut):
					}
				}
				switch f.(type) {
				case *frames.TTSAudioRawFrame:
					gotTTS = true
					turnsDone++
				case *frames.ErrorFrame:
					cancel()
					<-runner.Done()
					return turnsDone > 0, time.Since(start).Seconds() * 1000, nil
				}
			}
		}
	}
	cancel()
	<-runner.Done()
	durationMs = time.Since(start).Seconds() * 1000
	return turnsDone >= cfg.TurnsPerSession, durationMs, nil
}

// RunStress runs the stress test according to cfg and returns aggregated results.
func RunStress(ctx context.Context, cfg StressConfig) (*StressResult, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.PerSessionTimeout <= 0 {
		cfg.PerSessionTimeout = 15 * time.Second
	}
	if cfg.TurnsPerSession <= 0 {
		cfg.TurnsPerSession = 1
	}

	var sharedSTT *mock.STT
	var sharedLLM *mock.LLM
	var sharedTTS *mock.TTS
	if !cfg.PerSessionMocks {
		sharedSTT = mock.NewSTTWithTranscript("hello")
		sharedLLM = mock.NewLLMWithResponse("hi")
		sharedTTS = mock.NewTTS()
	}

	var totalSessions atomic.Int64
	var successCount atomic.Int64
	var latencyMu sync.Mutex
	var latencies []float64

	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		workerIndex := w
		go func(workerIndex int) {
			defer wg.Done()

			if cfg.ConnectionPattern == ConnectionRampUp && cfg.RampUpDuration > 0 {
				step := cfg.RampUpDuration / time.Duration(cfg.Concurrency)
				delay := time.Duration(workerIndex) * step
				if delay > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(delay):
					}
				}
			}

			for {
				if ctx.Err() != nil {
					return
				}
				if cfg.TotalSessions > 0 && totalSessions.Load() >= int64(cfg.TotalSessions) {
					return
				}
				if cfg.Duration > 0 && time.Since(start) >= cfg.Duration {
					return
				}
				totalSessions.Add(1)
				ok, durMs, _ := runOneSession(ctx, cfg, sharedSTT, sharedLLM, sharedTTS)
				if ok {
					successCount.Add(1)
					latencyMu.Lock()
					latencies = append(latencies, durMs)
					latencyMu.Unlock()
				}
			}
		}(workerIndex)
	}

	wg.Wait()
	elapsed := time.Since(start).Seconds()
	total := int(totalSessions.Load())
	success := int(successCount.Load())

	result := &StressResult{
		SuccessCount:   success,
		FailureCount:   total - success,
		TotalSessions:  total,
		DurationSec:    elapsed,
		LatencySamples: len(latencies),
	}
	if elapsed > 0 && total > 0 {
		result.SessionsPerSec = float64(total) / elapsed
	}

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		result.LatencyMsMin = latencies[0]
		result.LatencyMsMax = latencies[len(latencies)-1]
		var sum float64
		for _, v := range latencies {
			sum += v
		}
		result.LatencyMsMean = sum / float64(len(latencies))
		result.LatencyMsP50 = percentile(latencies, 0.50)
		result.LatencyMsP95 = percentile(latencies, 0.95)
		result.LatencyMsP99 = percentile(latencies, 0.99)
	}

	return result, nil
}

// percentile returns the approximate percentile from a sorted slice (index-based; suitable for stress-test metrics).
// For very large samples or production SLOs, histogram-based percentiles are often used instead.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// TestRunStress_RampUpPattern ensures that the ConnectionRampUp pattern schedules
// sessions over the configured ramp duration while still achieving a healthy
// success rate for a small bounded run.
func TestRunStress_RampUpPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ramp-up harness test in short mode")
	}

	cfg := DefaultStressConfig()
	cfg.Concurrency = 500
	cfg.TotalSessions = 8
	cfg.Duration = 0
	cfg.PerSessionTimeout = 5 * time.Second
	cfg.PerSessionMocks = true
	cfg.TurnsPerSession = 1
	cfg.ConnectionPattern = ConnectionRampUp
	cfg.RampUpDuration = time.Duration(cfg.Concurrency) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := RunStress(ctx, cfg)
	if err != nil {
		t.Fatalf("RunStress: %v", err)
	}
	if result.TotalSessions != cfg.TotalSessions {
		t.Fatalf("expected %d total sessions, got %d", cfg.TotalSessions, result.TotalSessions)
	}
	if result.SuccessCount == 0 || result.SuccessRate() <= 0.5 {
		t.Fatalf("expected >0 successes and success rate > 0.5, got success=%d rate=%.2f", result.SuccessCount, result.SuccessRate())
	}
	t.Logf("ramp-up harness (concurrency=%d, ramp=%s): %d/%d success, %d failures, p95=%.0f ms, %.1f sessions/sec",
		cfg.Concurrency, cfg.RampUpDuration, result.SuccessCount, result.TotalSessions, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
	t.Logf(`<testcase name="%s" type="harness_ramp" concurrency="%d" rampUp="%s" totalSessions="%d" success="%d" failed="%d" p95_ms="%.0f" sessions_per_sec="%.1f"/>`,
		t.Name(), cfg.Concurrency, cfg.RampUpDuration.String(), result.TotalSessions, result.SuccessCount, result.FailureCount, result.LatencyMsP95, result.SessionsPerSec)
}
