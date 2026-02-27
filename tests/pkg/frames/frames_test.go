package frames_test

import (
	"testing"

	"voila-go/pkg/frames"
)

func TestNewStartFrame(t *testing.T) {
	f := frames.NewStartFrame()
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
	f := frames.NewTextFrame("hello")
	if f.FrameType() != "TextFrame" {
		t.Errorf("FrameType = %q", f.FrameType())
	}
	if f.Text != "hello" || !f.AppendToContext {
		t.Errorf("Text=%q AppendToContext=%v", f.Text, f.AppendToContext)
	}
}

func TestNewCancelFrame(t *testing.T) {
	f := frames.NewCancelFrame("done")
	if f.FrameType() != "CancelFrame" || f.Reason != "done" {
		t.Errorf("unexpected: %+v", f)
	}
}

func TestUserTurnFramesImplementFrame(t *testing.T) {
	var _ frames.Frame = (&frames.UserStartedSpeakingFrame{})
	var _ frames.Frame = (&frames.UserStoppedSpeakingFrame{})
	var _ frames.Frame = (&frames.UserIdleFrame{})
}

