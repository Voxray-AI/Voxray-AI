// Package observers provides optional metrics (latency, token usage) and OpenTelemetry stub.
package observers

import (
	"sync"
	"time"
)

// LLMTokenUsage holds token usage for an LLM call.
type LLMTokenUsage struct {
	PromptTokens           int
	CompletionTokens       int
	TotalTokens            int
	CacheReadInputTokens   int // optional
	CacheCreationInputTokens int // optional
	ReasoningTokens        int // optional
}

// TurnMetrics holds turn detection metrics.
type TurnMetrics struct {
	IsComplete           bool
	Probability          float64
	E2EProcessingTimeMs  float64
}

// Metrics holds simple counters and latencies for pipeline/processors.
// THREAD SAFETY: mu guards all fields; safe for concurrent use from observer callbacks.
type Metrics struct {
	mu            sync.Mutex
	InputLatency  time.Duration // time from input to first output
	TTFBDuration  time.Duration // time to first byte (e.g. first LLM/TTS output)
	TokenCount    int64         // LLM tokens (approximate; legacy)
	CharCount     int64         // TTS characters
	FrameCount    int64         // frames processed
	LastProcessed time.Time
	// Structured usage (last recorded)
	LastLLMUsage LLMTokenUsage
	LastTurnMetrics TurnMetrics
}

// NewMetrics returns a new Metrics.
func NewMetrics() *Metrics { return &Metrics{} }

// RecordLatency sets the input-to-output latency.
func (m *Metrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InputLatency = d
}

// RecordTTFB sets the time-to-first-byte duration.
func (m *Metrics) RecordTTFB(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TTFBDuration = d
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

// RecordLLMUsage records LLM token usage (and updates legacy TokenCount).
func (m *Metrics) RecordLLMUsage(u LLMTokenUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastLLMUsage = u
	m.TokenCount = int64(u.TotalTokens)
}

// RecordTurnMetrics records turn detection metrics.
func (m *Metrics) RecordTurnMetrics(t TurnMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastTurnMetrics = t
}

// Snapshot returns a copy of current metrics.
func (m *Metrics) Snapshot() (latency time.Duration, tokens, chars, frames int64, last time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.InputLatency, m.TokenCount, m.CharCount, m.FrameCount, m.LastProcessed
}

// SnapshotFull returns latency, TTFB, tokens, chars, frames, last time, last LLM usage, and last turn metrics.
func (m *Metrics) SnapshotFull() (inputLatency, ttfb time.Duration, tokens, chars, frames int64, last time.Time, llm LLMTokenUsage, turn TurnMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.InputLatency, m.TTFBDuration, m.TokenCount, m.CharCount, m.FrameCount, m.LastProcessed, m.LastLLMUsage, m.LastTurnMetrics
}

// OpenTelemetry stub: no-op export. Replace with otel export when integrating.
var OTELExport = func(metrics *Metrics) {
	// Stub: e.g. otel.Meter().Record(...)
	_ = metrics
}
