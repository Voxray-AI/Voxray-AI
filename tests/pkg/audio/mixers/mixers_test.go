package mixers_test

import (
	"io"
	"testing"

	"voila-go/pkg/audio"
	"voila-go/pkg/audio/mixers"
)

func TestNewSimpleMixer(t *testing.T) {
	m := mixers.NewSimpleMixer()
	if m == nil {
		t.Fatal("NewSimpleMixer() returned nil")
	}
	if m.Output() == nil {
		t.Error("Output() should not be nil")
	}
}

func TestSimpleMixer_RemoveInput_NoopWhenMissing(t *testing.T) {
	m := mixers.NewSimpleMixer()
	// Should not panic when removing non-existent input.
	m.RemoveInput("nonexistent")
}

func TestSimpleMixer_AddInputRemoveInput(t *testing.T) {
	m := mixers.NewSimpleMixer()
	defer m.Close()
	// Add a stream that immediately returns EOF so the mixer goroutine exits.
	s := &eofStream{}
	m.AddInput("one", s)
	m.RemoveInput("one")
}

// eofStream is a minimal audio.Stream that returns EOF on Read.
type eofStream struct{}

func (e *eofStream) Read() (audio.Frame, error) {
	return audio.Frame{}, io.EOF
}

func (e *eofStream) Write(audio.Frame) error { return nil }
func (e *eofStream) Close() error            { return nil }
