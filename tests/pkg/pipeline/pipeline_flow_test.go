package pipeline_test

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
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
	p := pipeline.New()

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

// TestPipeline_StartFrameMetadata mirrors upstream test_pipeline_start_metadata: StartFrame with metadata passes through.
func TestPipeline_StartFrameMetadata(t *testing.T) {
	p := pipeline.New()
	m1 := newMockProcessor("m1")
	p.Link(m1)
	ctx := context.Background()
	if err := p.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer p.Cleanup(ctx)

	start := frames.NewStartFrame()
	start.Metadata()["foo"] = "bar"
	if err := p.Start(ctx, start); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(m1.received) < 1 {
		t.Fatalf("expected at least StartFrame, got %d", len(m1.received))
	}
	gotStart, ok := m1.received[0].(*frames.StartFrame)
	if !ok {
		t.Fatalf("first frame expected *StartFrame, got %T", m1.received[0])
	}
	if gotStart.Metadata()["foo"] != "bar" {
		t.Errorf("expected metadata foo=bar, got %v", gotStart.Metadata())
	}
}

// TestPipeline_CancelFramePropagation mirrors upstream: CancelFrame propagates downstream to all processors in order.
func TestPipeline_CancelFramePropagation(t *testing.T) {
	p := pipeline.New()
	m1 := newMockProcessor("m1")
	m2 := newMockProcessor("m2")
	p.Link(m1, m2)
	ctx := context.Background()
	if err := p.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer p.Cleanup(ctx)
	if err := p.Start(ctx, nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = p.Push(ctx, frames.NewTextFrame("x"))
	_ = p.Push(ctx, frames.NewCancelFrame("test-reason"))
	// Both processors should receive StartFrame, TextFrame, then CancelFrame
	if len(m1.received) != 3 {
		t.Errorf("m1 expected 3 frames (Start, Text, Cancel), got %d", len(m1.received))
	}
	if len(m2.received) != 3 {
		t.Errorf("m2 expected 3 frames (Start, Text, Cancel), got %d", len(m2.received))
	}
	if len(m1.received) >= 3 && m1.received[2].FrameType() != "CancelFrame" {
		t.Errorf("m1 third frame expected CancelFrame, got %s", m1.received[2].FrameType())
	}
	if len(m2.received) >= 3 && m2.received[2].FrameType() != "CancelFrame" {
		t.Errorf("m2 third frame expected CancelFrame, got %s", m2.received[2].FrameType())
	}
}

