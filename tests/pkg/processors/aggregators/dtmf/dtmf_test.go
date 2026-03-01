package dtmf_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregators/dtmf"
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

func mustDTMF(d frames.KeypadEntry) *frames.InputDTMFFrame {
	f, err := frames.NewInputDTMFFrame(d)
	if err != nil {
		panic(err)
	}
	return f
}

// TestDTMFAggregator_FlushOnTermination mirrors upstream test_dtmf_aggregator: digits accumulate, flush on #.
func TestDTMFAggregator_FlushOnTermination(t *testing.T) {
	proc := dtmf.New("dtmf", 5*time.Second, frames.KeypadPound, "DTMF: ")
	collector := &collectProcessor{mu: &sync.Mutex{}}
	proc.SetNext(collector)
	ctx := context.Background()

	_ = proc.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	_ = proc.ProcessFrame(ctx, mustDTMF("1"), processors.Downstream)
	_ = proc.ProcessFrame(ctx, mustDTMF("2"), processors.Downstream)
	_ = proc.ProcessFrame(ctx, mustDTMF("#"), processors.Downstream)

	var transcription *frames.TranscriptionFrame
	for _, f := range collector.frames {
		if tf, ok := f.(*frames.TranscriptionFrame); ok {
			transcription = tf
			break
		}
	}
	if transcription == nil {
		t.Fatalf("expected one TranscriptionFrame from DTMF flush, got %d frames", len(collector.frames))
	}
	// Implementation includes termination digit in flushed text
	if transcription.Text != "DTMF: 12#" {
		t.Errorf("expected TranscriptionFrame text 'DTMF: 12#', got %q", transcription.Text)
	}
}

// TestDTMFAggregator_StartAndEndFrame passes StartFrame through and flushes on EndFrame.
func TestDTMFAggregator_StartAndEndFrame(t *testing.T) {
	proc := dtmf.New("dtmf", 5*time.Second, frames.KeypadPound, "DTMF: ")
	collector := &collectProcessor{mu: &sync.Mutex{}}
	proc.SetNext(collector)
	ctx := context.Background()

	_ = proc.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	_ = proc.ProcessFrame(ctx, mustDTMF("9"), processors.Downstream)
	_ = proc.ProcessFrame(ctx, frames.NewEndFrame(), processors.Downstream)

	var gotStart, gotEnd bool
	var transcription *frames.TranscriptionFrame
	for _, f := range collector.frames {
		switch v := f.(type) {
		case *frames.StartFrame:
			gotStart = true
		case *frames.EndFrame:
			gotEnd = true
		case *frames.TranscriptionFrame:
			transcription = v
		}
	}
	if !gotStart {
		t.Error("expected StartFrame downstream")
	}
	if !gotEnd {
		t.Error("expected EndFrame downstream")
	}
	if transcription == nil || transcription.Text != "DTMF: 9" {
		if transcription != nil {
			t.Errorf("expected TranscriptionFrame 'DTMF: 9', got %q", transcription.Text)
		} else {
			t.Error("expected TranscriptionFrame from flush on EndFrame")
		}
	}
}
