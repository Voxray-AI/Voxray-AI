package frames

import (
	"testing"
)

func TestNewStartFrame(t *testing.T) {
	f := NewStartFrame()
	if f.FrameType() != "StartFrame" {
		t.Errorf("FrameType = %q", f.FrameType())
	}
	if f.ID() == 0 {
		t.Error("ID should be non-zero")
	}
	if f.AudioInSampleRate != 16000 {
		t.Errorf("AudioInSampleRate = %d", f.AudioInSampleRate)
	}
}

func TestNewTextFrame(t *testing.T) {
	f := NewTextFrame("hello")
	if f.FrameType() != "TextFrame" {
		t.Errorf("FrameType = %q", f.FrameType())
	}
	if f.Text != "hello" || !f.AppendToContext {
		t.Errorf("Text=%q AppendToContext=%v", f.Text, f.AppendToContext)
	}
}

func TestNewCancelFrame(t *testing.T) {
	f := NewCancelFrame("done")
	if f.FrameType() != "CancelFrame" || f.Reason != "done" {
		t.Errorf("unexpected: %+v", f)
	}
}
