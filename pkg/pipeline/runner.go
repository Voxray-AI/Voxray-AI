package pipeline

import (
	"context"
	"sync"

	"voila-go/pkg/frames"
)

// Transport is the minimal interface for runner: input frames from transport, output frames to transport.
type Transport interface {
	// Input returns a channel of frames from the client (or nil if not used).
	Input() <-chan frames.Frame
	// Output sends frames to the client.
	Output() chan<- frames.Frame
	// Start starts the transport; block until Ready or error.
	Start(ctx context.Context) error
	// Close closes the transport.
	Close() error
}

// Runner builds a pipeline from processors and runs it with a transport.
type Runner struct {
	Pipeline  *Pipeline
	Transport Transport
	done      chan struct{}
	mu        sync.Mutex
}

// NewRunner returns a Runner that will run the given pipeline with the transport.
func NewRunner(pl *Pipeline, tr Transport) *Runner {
	return &Runner{Pipeline: pl, Transport: tr, done: make(chan struct{})}
}

// Run starts the pipeline and transport, feeds input frames into the pipeline, and sends pipeline output to transport.
// It blocks until ctx is cancelled or a fatal error occurs. Caller can push frames into the pipeline
// from another goroutine; output is typically sent to Transport.Output() by the last processor (sink).
func (r *Runner) Run(ctx context.Context) error {
	if r.Pipeline == nil || r.Transport == nil {
		return nil
	}
	if err := r.Pipeline.Setup(ctx); err != nil {
		return err
	}
	defer r.Pipeline.Cleanup(ctx)

	if err := r.Transport.Start(ctx); err != nil {
		return err
	}
	defer r.Transport.Close()

	// Push StartFrame
	if err := r.Pipeline.Start(ctx, frames.NewStartFrame()); err != nil {
		return err
	}

	inCh := r.Transport.Input()
	outCh := r.Transport.Output()
	if inCh == nil && outCh == nil {
		<-ctx.Done()
		return ctx.Err()
	}

	// If we have an input channel, forward frames to pipeline in a goroutine.
	if inCh != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case f, ok := <-inCh:
					if !ok {
						return
					}
					_ = r.Pipeline.Push(ctx, f)
					// Stop on fatal error or cancel
					if ef, ok := f.(*frames.ErrorFrame); ok && ef.Fatal {
						return
					}
					if _, ok := f.(*frames.CancelFrame); ok {
						return
					}
				}
			}
		}()
	}

	// Block until context done
	<-ctx.Done()
	r.mu.Lock()
	close(r.done)
	r.done = make(chan struct{})
	r.mu.Unlock()
	return ctx.Err()
}

// Done returns a channel that is closed when Run returns.
func (r *Runner) Done() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}
