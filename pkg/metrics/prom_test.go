package metrics

import (
	"testing"
)

func TestSampledSessionID_Empty(t *testing.T) {
	if got := SampledSessionID("", 1); got != "" {
		t.Fatalf("expected empty session id for empty input, got %q", got)
	}
}

func TestSampledSessionID_HashStable(t *testing.T) {
	id := "session-123"
	a := SampledSessionID(id, 1)
	b := SampledSessionID(id, 1)
	if a == "" || b == "" {
		t.Fatalf("expected non-empty hashed session id")
	}
	if a != b {
		t.Fatalf("expected stable hash, got %q and %q", a, b)
	}
}

