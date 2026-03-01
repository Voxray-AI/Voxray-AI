package filters_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/filters"
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
	ff := filters.NewFrameFilter("ff", []string{"TextFrame"})
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
	ff := filters.NewFrameFilter("ff", []string{"TextFrame"})
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
	ff := filters.NewFrameFilter("ff", []string{"TextFrame"})
	ff.SetNext(sink)

	_ = ff.ProcessFrame(ctx, frames.NewTranscriptionFrame("hi", "u1", "", true), processors.Downstream)
	if len(sink.got) != 0 {
		t.Errorf("expected 0 frames (TranscriptionFrame not allowed), got %d", len(sink.got))
	}
}

func TestFrameFilterFromOptions(t *testing.T) {
	opts := json.RawMessage(`{"allowed_types":["TranscriptionFrame","TextFrame"]}`)
	ff := filters.NewFrameFilterFromOptions("ff", opts)
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
	nf := filters.NewNullFilter("nf")
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
	id := filters.NewIdentityFilter("id")
	id.SetNext(sink)

	_ = id.ProcessFrame(ctx, frames.NewTextFrame("x"), processors.Downstream)
	_ = id.ProcessFrame(ctx, frames.NewEndFrame(), processors.Downstream)
	if len(sink.got) != 2 {
		t.Errorf("expected 2 frames, got %d", len(sink.got))
	}
}

// TestWakeCheckFilter_noWakeWord mirrors upstream test_filters.py: no wake phrase -> TranscriptionFrame dropped.
func TestWakeCheckFilter_noWakeWord(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	wc := filters.NewWakeCheckFilter("wake", []string{"Hey, Pipecat"}, 3*time.Second)
	wc.SetNext(sink)

	tf := frames.NewTranscriptionFrame("Phrase 1", "test", "", true)
	_ = wc.ProcessFrame(ctx, tf, processors.Downstream)
	if len(sink.got) != 0 {
		t.Errorf("expected 0 frames when no wake word, got %d", len(sink.got))
	}
}

// TestWakeCheckFilter_wakeWord mirrors upstream: after wake phrase, subsequent TranscriptionFrames pass; last text is "Phrase 1".
func TestWakeCheckFilter_wakeWord(t *testing.T) {
	ctx := context.Background()
	sink := newCollectProcessor("sink")
	wc := filters.NewWakeCheckFilter("wake", []string{"Hey, Pipecat"}, 3*time.Second)
	wc.SetNext(sink)

	_ = wc.ProcessFrame(ctx, frames.NewTranscriptionFrame("Hey, Pipecat", "test", "", true), processors.Downstream)
	_ = wc.ProcessFrame(ctx, frames.NewTranscriptionFrame("Phrase 1", "test", "", true), processors.Downstream)
	if len(sink.got) < 1 {
		t.Fatalf("expected at least 1 frame after wake word, got %d", len(sink.got))
	}
	last := sink.got[len(sink.got)-1]
	tf, ok := last.(*frames.TranscriptionFrame)
	if !ok {
		t.Fatalf("last frame should be TranscriptionFrame, got %T", last)
	}
	if tf.Text != "Phrase 1" {
		t.Errorf("expected last text 'Phrase 1', got %q", tf.Text)
	}
}
