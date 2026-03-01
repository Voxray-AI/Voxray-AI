package observers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/observers"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/filters"
)

func TestNoopObserver_OnFrameProcessed(t *testing.T) {
	var o observers.NoopObserver
	// Should not panic.
	o.OnFrameProcessed("test", frames.NewStartFrame(), processors.Downstream)
}

func TestNewCompositeObserver(t *testing.T) {
	c := observers.NewCompositeObserver()
	if c == nil {
		t.Fatal("NewCompositeObserver returned nil")
	}
	// With no observers, OnFrameProcessed should not panic.
	c.OnFrameProcessed("p", frames.NewTextFrame("x"), processors.Downstream)
}

func TestNewCompositeObserver_Delegates(t *testing.T) {
	called := false
	o := &mockObserver{onProcessed: func() { called = true }}
	c := observers.NewCompositeObserver(o)
	c.OnFrameProcessed("p", frames.NewStartFrame(), processors.Downstream)
	if !called {
		t.Error("CompositeObserver did not call child OnFrameProcessed")
	}
}

// TestObservingProcessor_NotifiesObserver mirrors upstream observer tests: push frames, assert observer is called.
func TestObservingProcessor_NotifiesObserver(t *testing.T) {
	var count int
	var mu sync.Mutex
	ob := &mockObserver{onProcessed: func() {
		mu.Lock()
		count++
		mu.Unlock()
	}}
	inner := filters.NewIdentityFilter("id")
	wrap := observers.WrapWithObserver(inner, ob)
	ctx := context.Background()
	wrap.Setup(ctx)
	defer wrap.Cleanup(ctx)

	wrap.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	wrap.ProcessFrame(ctx, frames.NewTextFrame("hi"), processors.Downstream)
	mu.Lock()
	n := count
	mu.Unlock()
	if n != 2 {
		t.Errorf("expected observer OnFrameProcessed to be called 2 times, got %d", n)
	}
}

// TestTurnTrackingObserver_OnTurnStarted mirrors upstream: StartFrame triggers turn started callback.
func TestTurnTrackingObserver_OnTurnStarted(t *testing.T) {
	var started int
	ob := observers.NewTurnTrackingObserver(observers.OnTurnStarted(func(turnCount int) {
		started = turnCount
	}))
	ob.OnFrameProcessed("p", frames.NewStartFrame(), processors.Downstream)
	if started != 1 {
		t.Errorf("expected OnTurnStarted(1), got %d", started)
	}
}

// TestUserBotLatencyObserver_OnLatencyMeasured mirrors upstream: user stopped then bot started triggers latency callback.
func TestUserBotLatencyObserver_OnLatencyMeasured(t *testing.T) {
	var latencySecs float64
	var called bool
	ob := observers.NewUserBotLatencyObserver(observers.OnLatencyMeasured(func(secs float64) {
		called = true
		latencySecs = secs
	}))
	ob.OnFrameProcessed("p", frames.NewUserStoppedSpeakingFrame(), processors.Downstream)
	time.Sleep(10 * time.Millisecond) // ensure measurable latency
	ob.OnFrameProcessed("p", frames.NewBotStartedSpeakingFrame(), processors.Downstream)
	if !called {
		t.Error("expected OnLatencyMeasured callback to be called")
	}
	if latencySecs <= 0 {
		t.Errorf("expected positive latency, got %f", latencySecs)
	}
}

// TestTurnTrackingObserver_MultipleTurns mirrors upstream: first turn starts on StartFrame; second turn starts after end (CancelFrame) then UserStartedSpeakingFrame.
func TestTurnTrackingObserver_MultipleTurns(t *testing.T) {
	var counts []int
	ob := observers.NewTurnTrackingObserver(observers.OnTurnStarted(func(turnCount int) {
		counts = append(counts, turnCount)
	}))
	ob.OnFrameProcessed("p", frames.NewStartFrame(), processors.Downstream)
	if len(counts) != 1 || counts[0] != 1 {
		t.Fatalf("first StartFrame should trigger OnTurnStarted(1), got %v", counts)
	}
	ob.OnFrameProcessed("p", frames.NewCancelFrame("test"), processors.Downstream)
	ob.OnFrameProcessed("p", frames.NewUserStartedSpeakingFrame(), processors.Downstream)
	if len(counts) != 2 {
		t.Errorf("expected OnTurnStarted called 2 times (turn 1 then turn 2), got %d: %v", len(counts), counts)
	}
	if len(counts) >= 2 && counts[1] != 2 {
		t.Errorf("second turn count should be 2, got %d", counts[1])
	}
}

// TestUserBotLatencyObserver_NoUserStop_NoCallback mirrors upstream: bot start without prior user stop should not trigger latency callback.
func TestUserBotLatencyObserver_NoUserStop_NoCallback(t *testing.T) {
	var called bool
	ob := observers.NewUserBotLatencyObserver(observers.OnLatencyMeasured(func(_ float64) {
		called = true
	}))
	ob.OnFrameProcessed("p", frames.NewBotStartedSpeakingFrame(), processors.Downstream)
	if called {
		t.Error("OnLatencyMeasured should not be called when bot starts without prior user stop")
	}
}

type mockObserver struct {
	onProcessed func()
}

func (m *mockObserver) OnFrameProcessed(_ string, _ frames.Frame, _ processors.Direction) {
	if m.onProcessed != nil {
		m.onProcessed()
	}
}
