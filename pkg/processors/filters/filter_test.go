package filters

import (
	"context"
	"encoding/json"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// collectProcessor appends all frames it receives to a slice for tests.
type collectProcessor struct {
	*processors.BaseProcessor
	got []frames.Frame
}

func (c *collectProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	c.got = append(c.got, f)
	return c.BaseProcessor.ProcessFrame(ctx, f, dir)
}

func newCollectProcessor(name string) *collectProcessor {
	return &collectProcessor{BaseProcessor: processors.NewBaseProcessor(name)}
}

func TestFrameFilter_lifecycleAlwaysPasses(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	ff := NewFrameFilter("ff", []string{"TextFrame"})
	ff.SetNext(sink)

	for _, f := range []frames.Frame{
		frames.NewStartFrame(),
		frames.NewEndFrame(),
		frames.NewCancelFrame("test"),
	} {
		_ = ff.ProcessFrame(ctx, f, processors.Downstream)
	}
	if len(sink.got) != 3 {
		t.Errorf("expected 3 lifecycle frames, got %d", len(sink.got))
	}
}

func TestFrameFilter_allowedTypePasses(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	ff := NewFrameFilter("ff", []string{"TextFrame"})
	ff.SetNext(sink)

	tf := frames.NewTextFrame("hello")
	_ = ff.ProcessFrame(ctx, tf, processors.Downstream)
	if len(sink.got) != 1 || sink.got[0].FrameType() != "TextFrame" {
		t.Errorf("expected 1 TextFrame, got %d: %v", len(sink.got), sink.got)
	}
}

func TestFrameFilter_blockedTypeDropped(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	ff := NewFrameFilter("ff", []string{"TextFrame"})
	ff.SetNext(sink)

	_ = ff.ProcessFrame(ctx, frames.NewTranscriptionFrame("hi", "u1", "", true), processors.Downstream)
	if len(sink.got) != 0 {
		t.Errorf("expected 0 frames (TranscriptionFrame not allowed), got %d", len(sink.got))
	}
}

func TestFrameFilterFromOptions(t *testing.T) {
	opts := json.RawMessage(`{"allowed_types":["TranscriptionFrame","TextFrame"]}`)
	ff := NewFrameFilterFromOptions("ff", opts)
	if ff == nil {
		t.Fatal("NewFrameFilterFromOptions returned nil")
	}
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	ff.SetNext(sink)
	tf := frames.NewTranscriptionFrame("wake", "u1", "", true)
	_ = ff.ProcessFrame(ctx, tf, processors.Downstream)
	if len(sink.got) != 1 {
		t.Errorf("expected 1 frame from opts-configured filter, got %d", len(sink.got))
	}
}

func TestNullFilter_onlyLifecyclePasses(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	nf := NewNullFilter("nf")
	nf.SetNext(sink)

	_ = nf.ProcessFrame(ctx, frames.NewTextFrame("no"), processors.Downstream)
	_ = nf.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	if len(sink.got) != 1 || sink.got[0].FrameType() != "StartFrame" {
		t.Errorf("expected 1 StartFrame only, got %d: %v", len(sink.got), sink.got)
	}
}

func TestIdentityFilter_passesAll(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	id := NewIdentityFilter("id")
	id.SetNext(sink)

	_ = id.ProcessFrame(ctx, frames.NewTextFrame("x"), processors.Downstream)
	_ = id.ProcessFrame(ctx, frames.NewEndFrame(), processors.Downstream)
	if len(sink.got) != 2 {
		t.Errorf("expected 2 frames, got %d", len(sink.got))
	}
}
