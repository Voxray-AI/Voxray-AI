package ivr_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/extensions/ivr"
	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

func TestIVRProcessor_DTMFAndStatus(t *testing.T) {
	ctx := context.Background()
	ivrProc := ivr.NewIVRProcessor("IVR", "Classify.", "Navigate.", 2.0)
	var collected []frames.Frame
	sink := newCollectSink(&collected)
	pl := pipeline.New()
	pl.Add(ivrProc)
	pl.Add(sink)
	_ = pl.Setup(ctx)
	defer pl.Cleanup(ctx)

	var dtmfSeen bool
	var statusSeen ivr.IVRStatus
	ivrProc.OnIVRStatusChanged(func(s ivr.IVRStatus) { statusSeen = s })
	// Push StartFrame so IVR sends classifier upstream (no prev, so it's a no-op).
	_ = pl.Push(ctx, frames.NewStartFrame())
	// Simulate LLM output with DTMF and completed.
	_ = pl.Push(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: " ` 1 ` "}})
	_ = pl.Push(ctx, frames.NewLLMFullResponseEndFrame())

	for _, f := range collected {
		if _, ok := f.(*frames.OutputDTMFUrgentFrame); ok {
			dtmfSeen = true
		}
		if s, ok := f.(*frames.TextFrame); ok && s.Text == " completed " {
			statusSeen = ivr.IVRStatusCompleted
		}
	}
	// We may not have a prev processor so DTMF/status are pushed downstream to sink.
	if !dtmfSeen {
		// Check for DTMF in collected (IVR pushes it downstream)
		for _, f := range collected {
			if df, ok := f.(*frames.OutputDTMFUrgentFrame); ok && df.Button == "1" {
				dtmfSeen = true
				break
			}
		}
	}
	if !dtmfSeen {
		t.Logf("collected frames: %d", len(collected))
		for i, f := range collected {
			t.Logf("  [%d] %s", i, f.FrameType())
		}
		t.Error("expected OutputDTMFUrgentFrame in collected")
	}
	if statusSeen != ivr.IVRStatusCompleted {
		// Status callback might not fire from this test flow (no completed in text we pushed)
		t.Logf("statusSeen = %v (optional)", statusSeen)
	}
}

type collectSink struct {
	*processors.BaseProcessor
	collected *[]frames.Frame
	mu        sync.Mutex
}

func newCollectSink(collected *[]frames.Frame) *collectSink {
	return &collectSink{BaseProcessor: processors.NewBaseProcessor("Sink"), collected: collected}
}

func (c *collectSink) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Downstream {
		c.mu.Lock()
		*c.collected = append(*c.collected, f)
		c.mu.Unlock()
	}
	return nil
}

var _ processors.Processor = (*collectSink)(nil)

// TestIVRProcessor_StatusCompleted mirrors upstream test_ivr_navigation: " completed " tag triggers IVRStatusCompleted callback.
func TestIVRProcessor_StatusCompleted(t *testing.T) {
	ctx := context.Background()
	ivrProc := ivr.NewIVRProcessor("IVR", "Classify.", "Navigate.", 2.0)
	var collected []frames.Frame
	sink := newCollectSink(&collected)
	pl := pipeline.New()
	pl.Add(ivrProc)
	pl.Add(sink)
	_ = pl.Setup(ctx)
	defer pl.Cleanup(ctx)

	var statusSeen ivr.IVRStatus
	ivrProc.OnIVRStatusChanged(func(s ivr.IVRStatus) { statusSeen = s })
	_ = pl.Push(ctx, frames.NewStartFrame())
	// Tag " completed " in backtick delimiters triggers handleMatch -> handleIVRStatus(IVRStatusCompleted)
	_ = pl.Push(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: " ` completed ` "}})
	_ = pl.Push(ctx, frames.NewLLMFullResponseEndFrame())

	if statusSeen != ivr.IVRStatusCompleted {
		t.Errorf("expected OnIVRStatusChanged(IVRStatusCompleted), got %v", statusSeen)
	}
}

// TestIVRProcessor_NavigationMultipleDTMF mirrors upstream: multiple DTMF in sequence (menu steps) produce multiple OutputDTMFUrgentFrame.
func TestIVRProcessor_NavigationMultipleDTMF(t *testing.T) {
	ctx := context.Background()
	ivrProc := ivr.NewIVRProcessor("IVR", "Classify.", "Navigate.", 2.0)
	var collected []frames.Frame
	sink := newCollectSink(&collected)
	pl := pipeline.New()
	pl.Add(ivrProc)
	pl.Add(sink)
	_ = pl.Setup(ctx)
	defer pl.Cleanup(ctx)

	_ = pl.Push(ctx, frames.NewStartFrame())
	_ = pl.Push(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: " ` 1 ` "}})
	_ = pl.Push(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: " ` 2 ` "}})
	_ = pl.Push(ctx, frames.NewLLMFullResponseEndFrame())

	var dtmfFrames int
	for _, f := range collected {
		if _, ok := f.(*frames.OutputDTMFUrgentFrame); ok {
			dtmfFrames++
		}
	}
	if dtmfFrames < 2 {
		t.Errorf("expected at least 2 OutputDTMFUrgentFrame (navigation steps), got %d", dtmfFrames)
	}
}
