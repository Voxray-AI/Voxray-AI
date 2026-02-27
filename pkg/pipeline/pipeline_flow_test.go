package pipeline

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// mockProcessor is a simple processor that records the frames it receives.
type mockProcessor struct {
	*processors.BaseProcessor
	received []frames.Frame
}

func newMockProcessor(name string) *mockProcessor {
	return &mockProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		received:      make([]frames.Frame, 0),
	}
}

func (p *mockProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	p.received = append(p.received, f)
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

func TestPipeline_Flow(t *testing.T) {
	p := New()

	m1 := newMockProcessor("m1")
	m2 := newMockProcessor("m2")
	m3 := newMockProcessor("m3")

	p.Link(m1, m2, m3)

	ctx := context.Background()

	// Test Setup
	if err := p.Setup(ctx); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Start (pushes StartFrame)
	if err := p.Start(ctx, nil); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Push a text frame
	tf := frames.NewTextFrame("hello")
	if err := p.Push(ctx, tf); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify m1 received StartFrame and TextFrame
	if len(m1.received) != 2 {
		t.Errorf("m1 expected 2 frames, got %d", len(m1.received))
	}
	if m1.received[0].FrameType() != "StartFrame" {
		t.Errorf("m1 first frame expected StartFrame, got %s", m1.received[0].FrameType())
	}
	if m1.received[1].FrameType() != "TextFrame" {
		t.Errorf("m1 second frame expected TextFrame, got %s", m1.received[1].FrameType())
	}

	// Verify m2 received the same
	if len(m2.received) != 2 {
		t.Errorf("m2 expected 2 frames, got %d", len(m2.received))
	}

	// Verify m3 received the same
	if len(m3.received) != 2 {
		t.Errorf("m3 expected 2 frames, got %d", len(m3.received))
	}

	// Test Cleanup
	p.Cleanup(ctx)
}
