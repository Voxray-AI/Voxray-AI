package gatedcontext_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregators/gatedcontext"
	"voila-go/pkg/utils/notifier"
)

type collectProcessor struct {
	mu     sync.Mutex
	frames []frames.Frame
}

func (c *collectProcessor) ProcessFrame(_ context.Context, f frames.Frame, _ processors.Direction) error {
	c.mu.Lock()
	c.frames = append(c.frames, f)
	c.mu.Unlock()
	return nil
}
func (c *collectProcessor) SetNext(processors.Processor)   {}
func (c *collectProcessor) SetPrev(processors.Processor)   {}
func (c *collectProcessor) Setup(context.Context) error   { return nil }
func (c *collectProcessor) Cleanup(context.Context) error { return nil }
func (c *collectProcessor) Name() string                   { return "collect" }

func (c *collectProcessor) got() []frames.Frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]frames.Frame(nil), c.frames...)
}

// TestGatedContext_StartClosed_HoldsUntilRelease mirrors upstream: frames held until release; after Notify(), buffered frame flows in order.
func TestGatedContext_StartClosed_HoldsUntilRelease(t *testing.T) {
	ctx := context.Background()
	n := notifier.New()
	proc := gatedcontext.New("gated", n, false)
	collector := &collectProcessor{}
	proc.SetNext(collector)
	if err := proc.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer proc.Cleanup(ctx)

	// StartFrame passes through
	_ = proc.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	if len(collector.got()) != 1 {
		t.Fatalf("expected 1 frame (StartFrame), got %d", len(collector.got()))
	}

	// LLMContextFrame is held (gate closed)
	ctxFrame := frames.NewLLMContextFrame(&frames.LLMContext{Messages: []map[string]any{{"role": "user", "content": "held"}}})
	_ = proc.ProcessFrame(ctx, ctxFrame, processors.Downstream)
	if len(collector.got()) != 1 {
		t.Fatalf("LLMContextFrame should be held, still 1 frame, got %d", len(collector.got()))
	}

	// Notify releases the held frame
	n.Notify()
	time.Sleep(50 * time.Millisecond)
	got := collector.got()
	if len(got) != 2 {
		t.Fatalf("after Notify expected 2 frames, got %d", len(got))
	}
	if _, ok := got[1].(*frames.LLMContextFrame); !ok {
		t.Errorf("second frame should be LLMContextFrame, got %T", got[1])
	}
}

// TestGatedContext_StartOpen_FirstFramePasses verifies that when StartOpen is true, first LLMContextFrame passes through immediately.
func TestGatedContext_StartOpen_FirstFramePasses(t *testing.T) {
	ctx := context.Background()
	proc := gatedcontext.New("gated", notifier.New(), true)
	collector := &collectProcessor{}
	proc.SetNext(collector)
	if err := proc.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer proc.Cleanup(ctx)

	ctxFrame := frames.NewLLMContextFrame(&frames.LLMContext{Messages: []map[string]any{{"role": "user", "content": "first"}}})
	_ = proc.ProcessFrame(ctx, ctxFrame, processors.Downstream)
	got := collector.got()
	if len(got) != 1 {
		t.Fatalf("StartOpen: first LLMContextFrame should pass, got %d frames", len(got))
	}
	if _, ok := got[0].(*frames.LLMContextFrame); !ok {
		t.Errorf("expected LLMContextFrame, got %T", got[0])
	}

	// Second LLMContextFrame is held (StartOpen was set to false)
	ctxFrame2 := frames.NewLLMContextFrame(&frames.LLMContext{Messages: []map[string]any{{"role": "user", "content": "second"}}})
	_ = proc.ProcessFrame(ctx, ctxFrame2, processors.Downstream)
	if len(collector.got()) != 1 {
		t.Fatalf("second LLMContextFrame should be held, still 1 frame, got %d", len(collector.got()))
	}
}

// TestGatedContext_EndFrameClearsBuffer verifies that EndFrame clears the held frame and passes through.
func TestGatedContext_EndFrameClearsBuffer(t *testing.T) {
	ctx := context.Background()
	n := notifier.New()
	proc := gatedcontext.New("gated", n, false)
	collector := &collectProcessor{}
	proc.SetNext(collector)
	if err := proc.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer proc.Cleanup(ctx)

	_ = proc.ProcessFrame(ctx, frames.NewLLMContextFrame(&frames.LLMContext{}), processors.Downstream)
	_ = proc.ProcessFrame(ctx, frames.NewEndFrame(), processors.Downstream)
	got := collector.got()
	if len(got) != 1 {
		t.Fatalf("EndFrame should pass through and clear buffer; got %d frames", len(got))
	}
	if _, ok := got[0].(*frames.EndFrame); !ok {
		t.Errorf("expected EndFrame, got %T", got[0])
	}
	// Notify should not push anything (buffer was cleared)
	n.Notify()
	time.Sleep(50 * time.Millisecond)
	if len(collector.got()) != 1 {
		t.Errorf("after EndFrame cleared buffer, Notify should not add frames, got %d", len(collector.got()))
	}
}
