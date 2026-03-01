package gated_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregators/gated"
)

type collectProcessor struct {
	mu     *sync.Mutex
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

// TestGatedAggregator mirrors upstream test_aggregators.py GatedAggregator:
// gate opens on OutputAudioRawFrame, closes on LLMFullResponseStartFrame; start closed.
// Buffered frames are released when gate opens, in order: opening frame then buffer.
func TestGatedAggregator(t *testing.T) {
	gateOpen := func(f frames.Frame) bool {
		_, ok := f.(*frames.OutputAudioRawFrame)
		return ok
	}
	gateClose := func(f frames.Frame) bool {
		_, ok := f.(*frames.LLMFullResponseStartFrame)
		return ok
	}
	proc := gated.New("gated", gateOpen, gateClose, false, processors.Downstream)
	collector := &collectProcessor{mu: &sync.Mutex{}}
	proc.SetNext(collector)
	ctx := context.Background()

	// Send: LLMFullResponseStart (closes gate, buffer), Text, Text, OutputAudioRawFrame (opens gate).
	// Expected downstream: OutputAudioRawFrame first, then LLMFullResponseStart, Text, Text.
	proc.ProcessFrame(ctx, frames.NewLLMFullResponseStartFrame(), processors.Downstream)
	proc.ProcessFrame(ctx, frames.NewTextFrame("Hello, "), processors.Downstream)
	proc.ProcessFrame(ctx, frames.NewTextFrame("world."), processors.Downstream)
	audioBase := frames.NewAudioRawFrame([]byte("hello"), 16000, 1, 0)
	proc.ProcessFrame(ctx, &frames.OutputAudioRawFrame{AudioRawFrame: *audioBase}, processors.Downstream)

	if len(collector.frames) != 4 {
		t.Fatalf("expected 4 downstream frames, got %d", len(collector.frames))
	}
	if _, ok := collector.frames[0].(*frames.OutputAudioRawFrame); !ok {
		t.Errorf("first frame should be OutputAudioRawFrame, got %T", collector.frames[0])
	}
	if _, ok := collector.frames[1].(*frames.LLMFullResponseStartFrame); !ok {
		t.Errorf("second frame should be LLMFullResponseStartFrame, got %T", collector.frames[1])
	}
	if tf, ok := collector.frames[2].(*frames.TextFrame); !ok || tf.Text != "Hello, " {
		t.Errorf("third frame should be TextFrame 'Hello, ', got %v", collector.frames[2])
	}
	if tf, ok := collector.frames[3].(*frames.TextFrame); !ok || tf.Text != "world." {
		t.Errorf("fourth frame should be TextFrame 'world.', got %v", collector.frames[3])
	}
}

// TestGatedAggregator_StartOpen verifies that when StartOpen is true, frames pass through until gate closes.
func TestGatedAggregator_StartOpen(t *testing.T) {
	gateOpen := func(f frames.Frame) bool {
		_, ok := f.(*frames.TextFrame)
		return ok && f.(*frames.TextFrame).Text == "open"
	}
	gateClose := func(f frames.Frame) bool {
		_, ok := f.(*frames.EndFrame)
		return ok
	}
	proc := gated.New("gated", gateOpen, gateClose, true, processors.Downstream)
	collector := &collectProcessor{mu: &sync.Mutex{}}
	proc.SetNext(collector)
	ctx := context.Background()

	proc.ProcessFrame(ctx, frames.NewTextFrame("first"), processors.Downstream)
	proc.ProcessFrame(ctx, frames.NewTextFrame("second"), processors.Downstream)

	if len(collector.frames) != 2 {
		t.Fatalf("expected 2 frames when gate start open, got %d", len(collector.frames))
	}
}
