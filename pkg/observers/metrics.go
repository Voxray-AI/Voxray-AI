// Package observers provides optional metrics (latency, token usage) and OpenTelemetry stub.
package observers

import (
	"sync"
	"time"
)

// Metrics holds simple counters and latencies for pipeline/processors.
type Metrics struct {
	mu            sync.Mutex
	InputLatency  time.Duration // time from input to first output
	TokenCount    int64         // LLM tokens (approximate)
	CharCount     int64         // TTS characters
	FrameCount    int64         // frames processed
	LastProcessed time.Time
}

// NewMetrics returns a new Metrics.
func NewMetrics() *Metrics { return &Metrics{} }

// RecordLatency sets the input-to-output latency.
func (m *Metrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InputLatency = d
}

// AddTokens adds to the token count (LLM).
func (m *Metrics) AddTokens(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TokenCount += n
}

// AddChars adds to the character count (TTS).
func (m *Metrics) AddChars(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CharCount += n
}

// IncFrames increments the frame count.
func (m *Metrics) IncFrames() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FrameCount++
	m.LastProcessed = time.Now()
}

// Snapshot returns a copy of current metrics.
func (m *Metrics) Snapshot() (latency time.Duration, tokens, chars, frames int64, last time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.InputLatency, m.TokenCount, m.CharCount, m.FrameCount, m.LastProcessed
}

// OpenTelemetry stub: no-op export. Replace with otel export when integrating.
var OTELExport = func(metrics *Metrics) {
	// Stub: e.g. otel.Meter().Record(...)
	_ = metrics
}
