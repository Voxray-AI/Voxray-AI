// Package notifier provides a one-shot notifier for gate synchronization.
// Used by voicemail extension gates to wait until classification (conversation/voicemail) is decided.
package notifier

import (
	"context"
	"sync"
)

// Notifier is a one-shot signal. Wait blocks until Notify is called or context is cancelled.
type Notifier struct {
	mu     sync.Mutex
	ch     chan struct{}
	closed bool
}

// New returns a new Notifier.
func New() *Notifier {
	return &Notifier{ch: make(chan struct{})}
}

// Notify signals all waiters. Idempotent after the first call.
func (n *Notifier) Notify() {
	n.mu.Lock()
	if !n.closed {
		close(n.ch)
		n.closed = true
	}
	n.mu.Unlock()
}

// Wait blocks until Notify has been called or ctx is cancelled.
func (n *Notifier) Wait(ctx context.Context) error {
	select {
	case <-n.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reset makes the notifier usable again (for tests). Not safe to call concurrently with Wait.
func (n *Notifier) Reset() {
	n.mu.Lock()
	n.ch = make(chan struct{})
	n.closed = false
	n.mu.Unlock()
}
