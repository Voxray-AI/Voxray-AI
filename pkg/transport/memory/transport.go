// Package memory provides an in-memory transport for testing and stress testing.
// It implements transport.Transport using channels only (no WebRTC). Callers push
// frames via SendInput and read pipeline output via Out().
package memory

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/transport"
)

const defaultBuf = 128

// Transport is an in-memory transport that satisfies transport.Transport.
// Use SendInput to push frames into the pipeline; use Out() to read frames from the pipeline.
type Transport struct {
	inCh   chan frames.Frame
	outCh  chan frames.Frame
	closed chan struct{}
	once   sync.Once
}

// Ensure Transport implements transport.Transport.
var _ transport.Transport = (*Transport)(nil)

// NewTransport creates a new in-memory transport with buffered channels (default 128).
func NewTransport() *Transport {
	return NewTransportWithBuffer(defaultBuf)
}

// NewTransportWithBuffer creates a new in-memory transport with the given buffer size per channel.
func NewTransportWithBuffer(buf int) *Transport {
	if buf <= 0 {
		buf = defaultBuf
	}
	return &Transport{
		inCh:   make(chan frames.Frame, buf),
		outCh:  make(chan frames.Frame, buf),
		closed: make(chan struct{}),
	}
}

// Done returns a channel that is closed when the transport is closed.
func (t *Transport) Done() <-chan struct{} { return t.closed }

// Input returns the channel of frames that the pipeline consumes (runner reads from this).
func (t *Transport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel the pipeline writes to (sink sends TTS and other frames here).
func (t *Transport) Output() chan<- frames.Frame { return t.outCh }

// SendInput sends a frame into the pipeline input. It blocks until the frame is received
// or the transport is closed. Use from stress test / test code to simulate client input.
func (t *Transport) SendInput(ctx context.Context, f frames.Frame) bool {
	select {
	case <-ctx.Done():
		return false
	case <-t.closed:
		return false
	case t.inCh <- f:
		return true
	}
}

// Out returns the output channel for reading pipeline results (e.g. TTSAudioRawFrame).
// The test receives from this channel to consume frames the pipeline sends to the client.
func (t *Transport) Out() <-chan frames.Frame { return t.outCh }

// Start implements transport.Transport. It returns immediately (no network setup).
func (t *Transport) Start(ctx context.Context) error {
	return nil
}

// Close closes the transport. It closes the input channel so the runner's input
// goroutine exits, but does not close the output channel to avoid panics from
// in-flight pipeline sends (sink may still be sending when the runner exits).
func (t *Transport) Close() error {
	var err error
	t.once.Do(func() {
		close(t.closed)
		close(t.inCh)
		// Do not close t.outCh: the runner calls Close() when ctx is done while
		// the pipeline may still be sending to outCh (e.g. TTS/sink). Closing
		// outCh here would cause "send on closed channel". Callers reading from
		// Out() will see no more sends after the pipeline drains.
	})
	return err
}
