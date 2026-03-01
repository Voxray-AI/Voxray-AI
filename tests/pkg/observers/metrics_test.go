package observers_test

import (
	"testing"
	"time"

	"voila-go/pkg/observers"
)

func TestNewMetrics(t *testing.T) {
	m := observers.NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
}

func TestMetrics_RecordAndSnapshot(t *testing.T) {
	m := observers.NewMetrics()
	m.RecordLatency(100 * time.Millisecond)
	m.RecordTTFB(50 * time.Millisecond)
	m.AddTokens(10)
	m.AddChars(20)
	m.IncFrames()
	latency, tokens, chars, frames, last := m.Snapshot()
	if latency != 100*time.Millisecond {
		t.Errorf("Snapshot latency = %v", latency)
	}
	if tokens != 10 || chars != 20 || frames != 1 {
		t.Errorf("Snapshot tokens=%d chars=%d frames=%d", tokens, chars, frames)
	}
	if last.IsZero() {
		t.Error("Snapshot last should be set after IncFrames")
	}
}

func TestMetrics_RecordLLMUsage(t *testing.T) {
	m := observers.NewMetrics()
	m.RecordLLMUsage(observers.LLMTokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	_, tokens, _, _, _ := m.Snapshot()
	if tokens != 3 {
		t.Errorf("Snapshot tokens = %d, want 3", tokens)
	}
}
