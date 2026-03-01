package mixers

import (
	"io"
	"sync"

	"voxray-go/pkg/audio"
)

// Mixer mixes one or more input audio streams into a single output stream.
type Mixer interface {
	// AddInput registers a new input stream with the given ID.
	AddInput(id string, s audio.Stream)
	// RemoveInput unregisters an input stream by ID.
	RemoveInput(id string)
	// Output returns the mixed output stream.
	Output() audio.Stream
	// Close stops the mixer and closes the output stream.
	Close() error
}

// simpleStream is an in-memory implementation of audio.Stream backed by a channel.
type simpleStream struct {
	ch   chan audio.Frame
	once sync.Once
}

func newSimpleStream(buffer int) *simpleStream {
	if buffer <= 0 {
		buffer = 64
	}
	return &simpleStream{ch: make(chan audio.Frame, buffer)}
}

func (s *simpleStream) Read() (audio.Frame, error) {
	f, ok := <-s.ch
	if !ok {
		return audio.Frame{}, io.EOF
	}
	return f, nil
}

func (s *simpleStream) Write(f audio.Frame) error {
	select {
	case s.ch <- f:
		return nil
	default:
		// Drop if buffer is full; callers can tune buffer size.
		return nil
	}
}

func (s *simpleStream) Close() error {
	s.once.Do(func() {
		close(s.ch)
	})
	return nil
}

// SimpleMixer is a naive mixer that currently forwards audio frames from all
// inputs to the output without rescaling. It is intentionally conservative:
// callers can rely on it as a routing mixer and later swap in a more advanced
// implementation that actually sums samples.
type SimpleMixer struct {
	mu      sync.RWMutex
	inputs  map[string]audio.Stream
	output  *simpleStream
	closed  bool
	closeCh chan struct{}
}

// NewSimpleMixer creates a SimpleMixer.
func NewSimpleMixer() *SimpleMixer {
	m := &SimpleMixer{
		inputs:  make(map[string]audio.Stream),
		output:  newSimpleStream(128),
		closeCh: make(chan struct{}),
	}
	return m
}

// AddInput registers a new input stream and starts a forwarding goroutine.
func (m *SimpleMixer) AddInput(id string, s audio.Stream) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	if _, exists := m.inputs[id]; exists {
		m.mu.Unlock()
		return
	}
	m.inputs[id] = s
	m.mu.Unlock()

	go func() {
		for {
			select {
			case <-m.closeCh:
				return
			default:
			}
			f, err := s.Read()
			if err != nil {
				return
			}
			_ = m.output.Write(f)
		}
	}()
}

// RemoveInput stops using the input with the given ID.
func (m *SimpleMixer) RemoveInput(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.inputs[id]
	if !ok {
		return
	}
	delete(m.inputs, id)
	_ = s.Close()
}

// Output returns the mixed output as an audio.Stream.
func (m *SimpleMixer) Output() audio.Stream {
	return m.output
}

// Close stops the mixer and closes all inputs and the output.
func (m *SimpleMixer) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	close(m.closeCh)
	for _, s := range m.inputs {
		_ = s.Close()
	}
	m.inputs = nil
	m.mu.Unlock()
	return m.output.Close()
}

