package aggregator_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregator"
)

func TestNew(t *testing.T) {
	p := aggregator.New("agg", "", 0)
	if p == nil || p.Name() != "agg" {
		t.Errorf("New: name = %q", p.Name())
	}
	if p.SentenceEnd != ".!?" {
		t.Errorf("SentenceEnd = %q, want .!?", p.SentenceEnd)
	}
	p2 := aggregator.New("", "", 100)
	if p2.Name() != "Aggregator" {
		t.Errorf("default name = %q", p2.Name())
	}
	if p2.MaxBuffer != 100 {
		t.Errorf("MaxBuffer = %d", p2.MaxBuffer)
	}
}

func TestAggregator_FlushOnSentenceEnd(t *testing.T) {
	collector := &collectProcessor{mu: &sync.Mutex{}}
	agg := aggregator.New("agg", ".!?", 0)
	agg.SetNext(collector)
	ctx := context.Background()

	// Send "Hello " then "world." -> should emit one TextFrame "Hello world." on the period.
	_ = agg.ProcessFrame(ctx, frames.NewTextFrame("Hello "), processors.Downstream)
	_ = agg.ProcessFrame(ctx, frames.NewTextFrame("world."), processors.Downstream)

	if len(collector.frames) < 1 {
		t.Fatalf("expected at least one aggregated frame, got %d", len(collector.frames))
	}
	var found bool
	for _, f := range collector.frames {
		if tf, ok := f.(*frames.TextFrame); ok && tf.Text == "Hello world." {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TextFrame with 'Hello world.', got %v", collector.frames)
	}
}

func TestAggregator_FlushOnMaxBuffer(t *testing.T) {
	collector := &collectProcessor{mu: &sync.Mutex{}}
	agg := aggregator.New("agg", ".!?", 5) // flush after 5 runes
	agg.SetNext(collector)
	ctx := context.Background()

	_ = agg.ProcessFrame(ctx, frames.NewTextFrame("abcde"), processors.Downstream)
	// No sentence end; should flush due to MaxBuffer=5.
	if len(collector.frames) < 1 {
		t.Fatalf("expected flush on max buffer, got %d frames", len(collector.frames))
	}
}

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
