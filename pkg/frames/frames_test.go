package frames

import "testing"

func TestNewBase(t *testing.T) {
	b := NewBase()
	if b.ID() == 0 {
		t.Error("NewBase() ID should be non-zero")
	}
	if b.Metadata() == nil {
		t.Error("NewBase() Metadata should be non-nil")
	}
}

func TestNewBaseWithID(t *testing.T) {
	b := NewBaseWithID(42)
	if b.ID() != 42 {
		t.Errorf("NewBaseWithID(42) ID = %d, want 42", b.ID())
	}
}

func TestNewStartFrame(t *testing.T) {
	f := NewStartFrame()
	if f.FrameType() != "StartFrame" {
		t.Errorf("FrameType = %q", f.FrameType())
	}
	if f.AudioInSampleRate != 16000 {
		t.Errorf("AudioInSampleRate = %d", f.AudioInSampleRate)
	}
}
