package turn

import (
	"context"
	"testing"
)

func TestSilenceAnalyzeEndOfTurnAsyncReturnsSnapshot(t *testing.T) {
	params := Params{}
	a := NewSilenceTurnAnalyzer(params)

	a.mu.Lock()
	a.lastState = Complete
	a.mu.Unlock()

	ctx := context.Background()
	ch := a.AnalyzeEndOfTurnAsync(ctx)
	res, ok := <-ch
	if !ok {
		t.Fatalf("channel closed without result")
	}
	if res.Err != nil {
		t.Fatalf("expected nil error, got %v", res.Err)
	}
	if res.State != Complete {
		t.Fatalf("expected state %v, got %v", Complete, res.State)
	}
}

func TestSilenceAnalyzeEndOfTurnAsyncCancelled(t *testing.T) {
	params := Params{}
	a := NewSilenceTurnAnalyzer(params)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := a.AnalyzeEndOfTurnAsync(ctx)
	res, ok := <-ch
	if !ok {
		t.Fatalf("channel closed without result")
	}
	if res.Err == nil {
		t.Fatalf("expected non-nil error for cancelled context")
	}
	if res.State != Incomplete {
		t.Fatalf("expected state Incomplete on cancellation, got %v", res.State)
	}
}

func TestSilenceAnalyzeEndOfTurnAsyncMultipleCalls(t *testing.T) {
	params := Params{}
	a := NewSilenceTurnAnalyzer(params)

	a.mu.Lock()
	a.lastState = Complete
	a.mu.Unlock()

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		ch := a.AnalyzeEndOfTurnAsync(ctx)
		res, ok := <-ch
		if !ok {
			t.Fatalf("call %d: channel closed without result", i)
		}
		if res.Err != nil {
			t.Fatalf("call %d: expected nil error, got %v", i, res.Err)
		}
		if res.State != Complete {
			t.Fatalf("call %d: expected state %v, got %v", i, Complete, res.State)
		}
	}
}

