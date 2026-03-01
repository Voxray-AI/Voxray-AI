// Package notifier provides a simple signal that one goroutine can wait on and another can trigger.
package notifier

import (
	"context"
	"sync"
)

// Notifier allows waiting until Notify is called (or context is cancelled).
// Each Notify unblocks one Wait. Safe for concurrent use.
type Notifier struct {
	mu sync.Mutex
	ch chan struct{}
}

// New returns a new Notifier.
func New() *Notifier {
	return &Notifier{ch: make(chan struct{}, 1)}
}

// Notify unblocks one waiter. Non-blocking; if no one is waiting, the next Wait will return immediately.
func (n *Notifier) Notify() {
	n.mu.Lock()
	ch := n.ch
	n.mu.Unlock()
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Wait blocks until Notify is called or ctx is cancelled. Returns nil if Notify was called, otherwise ctx.Err().
func (n *Notifier) Wait(ctx context.Context) error {
	n.mu.Lock()
	ch := n.ch
	n.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
