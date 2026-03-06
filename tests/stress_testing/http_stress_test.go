// Package stress_testing: HTTP-layer stress test using a test-only mock offer endpoint.
// Stresses session creation and HTTP handling without real WebRTC/Pion.
package stress_testing

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services/mock"
	"voxray-go/pkg/transport"
	"voxray-go/pkg/transport/memory"
)

// httpClientTimeout is the per-request timeout for the stress test HTTP client (industry practice: avoid hanging on slow responses).
const httpClientTimeout = 10 * time.Second

// startMockOfferServer starts a test HTTP server with POST /test/webrtc/offer
// that creates a memory transport and runs the mock pipeline per request. It
// returns the full URL for the offer endpoint. The server is shut down via
// t.Cleanup when the test completes.
func startMockOfferServer(t *testing.T, ctx context.Context) string {
	t.Helper()

	onTransport := func(c context.Context, tr transport.Transport) {
		pl := pipeline.New()
		pl.Add(voice.NewSTTProcessor("stt", mock.NewSTTWithTranscript("hello"), 16000, 1))
		pl.Add(voice.NewLLMProcessorWithSystemPrompt("llm", mock.NewLLMWithResponse("hi"), "You are a helpful assistant. Reply briefly."))
		pl.Add(voice.NewTTSProcessor("tts", mock.NewTTS(), 24000))
		pl.Add(pipeline.NewSink("sink", tr.Output()))
		runner := pipeline.NewRunner(pl, tr, frames.NewStartFrame())
		go func() {
			_ = runner.Run(c)
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/test/webrtc/offer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tr := memory.NewTransport()
		go onTransport(ctx, tr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"answer": "mock"})
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		_ = server.Shutdown(ctx)
		_ = listener.Close()
	})

	return "http://" + listener.Addr().String() + "/test/webrtc/offer"
}

// TestHTTPStress_MockOfferEndpoint starts a test HTTP server with POST /test/webrtc/offer
// that creates a memory transport and runs the mock pipeline per request. It then fires
// many concurrent POSTs and asserts on success rate (connection-storm style load).
func TestHTTPStress_MockOfferEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HTTP stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	addr := startMockOfferServer(t, ctx)
	client := &http.Client{Timeout: httpClientTimeout}

	const concurrency = 50
	const totalRequests = 250
	var successCount atomic.Int32
	var errorCount atomic.Int32   // request/connection errors
	var non2xxCount atomic.Int32  // HTTP non-200 responses
	var latenciesMu sync.Mutex
	var latenciesMs []float64

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < totalRequests/concurrency; j++ {
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader([]byte(`{"offer":"mock"}`)))
				req.Header.Set("Content-Type", "application/json")
				reqStart := time.Now()
				resp, err := client.Do(req)
				elapsedMs := time.Since(reqStart).Seconds() * 1000
				if err != nil {
					errorCount.Add(1)
					continue
				}
				_ = resp.Body.Close()
				latenciesMu.Lock()
				latenciesMs = append(latenciesMs, elapsedMs)
				latenciesMu.Unlock()
				if resp.StatusCode == http.StatusOK {
					successCount.Add(1)
				} else {
					non2xxCount.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	got := successCount.Load()
	if got < int32(totalRequests)/2 {
		t.Errorf("HTTP stress: only %d/%d requests got 200 (expected at least half)", got, totalRequests)
	}

	// Report metrics (industry practice: response times, error rates, RPS) and a JUnit-style line.
	rps := 0.0
	if elapsed > 0 {
		rps = float64(totalRequests) / elapsed
	}
	failures := non2xxCount.Load() + errorCount.Load()
	attempted := got + failures
	t.Logf("HTTP stress (concurrency=%d): %d ok, %d failed (non-2xx=%d, errors=%d), attempted=%d/%d planned; RPS %.1f",
		concurrency, got, failures, non2xxCount.Load(), errorCount.Load(), attempted, totalRequests, rps)
	t.Logf(`<testcase name="%s" type="http_burst" concurrency="%d" totalRequests="%d" attempted="%d" success="%d" failed="%d" non2xx="%d" errors="%d" rps="%.1f"/>`,
		t.Name(), concurrency, totalRequests, attempted, got, failures, non2xxCount.Load(), errorCount.Load(), rps)
	if len(latenciesMs) > 0 {
		sort.Float64s(latenciesMs)
		p50 := httpStressPercentile(latenciesMs, 0.50)
		p95 := httpStressPercentile(latenciesMs, 0.95)
		p99 := httpStressPercentile(latenciesMs, 0.99)
		t.Logf("HTTP stress latency ms: p50=%.0f p95=%.0f p99=%.0f", p50, p95, p99)
	}
}

// TestHTTPStress_MockOfferEndpoint_RampUpConcurrency starts the same test HTTP server
// but ramps up concurrency over a short duration by staggering worker start times.
// This exercises a gentler connection pattern while keeping total work and SLOs similar.
func TestHTTPStress_MockOfferEndpoint_RampUpConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HTTP ramp-up HTTP stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	addr := startMockOfferServer(t, ctx)
	client := &http.Client{Timeout: httpClientTimeout}

	const concurrency = 50
	const totalRequests = 250
	const rampUpDuration = 3 * time.Second

	var successCount atomic.Int32
	var errorCount atomic.Int32   // request/connection errors
	var non2xxCount atomic.Int32  // HTTP non-200 responses
	var latenciesMu sync.Mutex
	var latenciesMs []float64

	start := time.Now()
	var wg sync.WaitGroup
	delayStep := rampUpDuration / time.Duration(concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer wg.Done()

			// Stagger worker start to ramp up concurrency.
			if delayStep > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(workerIndex) * delayStep):
				}
			}

			for j := 0; j < totalRequests/concurrency; j++ {
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader([]byte(`{"offer":"mock"}`)))
				req.Header.Set("Content-Type", "application/json")
				reqStart := time.Now()
				resp, err := client.Do(req)
				elapsedMs := time.Since(reqStart).Seconds() * 1000
				if err != nil {
					errorCount.Add(1)
					continue
				}
				_ = resp.Body.Close()
				latenciesMu.Lock()
				latenciesMs = append(latenciesMs, elapsedMs)
				latenciesMu.Unlock()
				if resp.StatusCode == http.StatusOK {
					successCount.Add(1)
				} else {
					non2xxCount.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	got := successCount.Load()
	if got < int32(totalRequests)/2 {
		t.Errorf("HTTP ramp-up stress: only %d/%d requests got 200 (expected at least half)", got, totalRequests)
	}

	// Report metrics and a JUnit-style line.
	rps := 0.0
	if elapsed > 0 {
		rps = float64(totalRequests) / elapsed
	}
	failures := non2xxCount.Load() + errorCount.Load()
	attempted := got + failures
	t.Logf("HTTP ramp-up stress (concurrency=%d, ramp=%s): %d ok, %d failed (non-2xx=%d, errors=%d), attempted=%d/%d planned; RPS %.1f",
		concurrency, rampUpDuration, got, failures, non2xxCount.Load(), errorCount.Load(), attempted, totalRequests, rps)
	t.Logf(`<testcase name="%s" type="http_ramp" concurrency="%d" rampUp="%s" totalRequests="%d" attempted="%d" success="%d" failed="%d" non2xx="%d" errors="%d" rps="%.1f"/>`,
		t.Name(), concurrency, rampUpDuration.String(), totalRequests, attempted, got, failures, non2xxCount.Load(), errorCount.Load(), rps)
	if len(latenciesMs) > 0 {
		sort.Float64s(latenciesMs)
		p50 := httpStressPercentile(latenciesMs, 0.50)
		p95 := httpStressPercentile(latenciesMs, 0.95)
		p99 := httpStressPercentile(latenciesMs, 0.99)
		t.Logf("HTTP ramp-up stress latency ms: p50=%.0f p95=%.0f p99=%.0f", p50, p95, p99)
	}
}

func httpStressPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
