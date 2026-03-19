package smallwebrtc

import (
	"testing"
	"time"
)

func TestMaxDurationFiresAfterFirstInboundAudio(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr := NewTransport(&Config{
		MaxDuration: 25 * time.Millisecond,
		OnMaxDurationTimeout: func() {
			select {
			case fired <- struct{}{}:
			default:
			}
		},
	})

	// Mark the first inbound audio twice; the timer should schedule only once.
	tr.noteFirstInboundAudio()
	tr.noteFirstInboundAudio()

	select {
	case <-fired:
		// ok
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected max-duration timeout to fire")
	}
}

func TestMaxDurationDoesNotFireAfterClose(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr := NewTransport(&Config{
		MaxDuration: 25 * time.Millisecond,
		OnMaxDurationTimeout: func() {
			select {
			case fired <- struct{}{}:
			default:
			}
		},
	})

	tr.noteFirstInboundAudio()
	_ = tr.Close()

	select {
	case <-fired:
		t.Fatal("did not expect max-duration timeout to fire after Close")
	case <-time.After(150 * time.Millisecond):
		// ok
	}
}

