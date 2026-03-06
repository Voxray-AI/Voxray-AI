package pipeline

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
)

// inputQueueCap is the buffer size between transport read and pipeline push.
// Decouples reading mic input from pipeline processing so that when the pipeline
// is blocked (e.g. Sink writing TTS to transport), we still drain transport input.
const inputQueueCap = 256

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
	Pipeline   *Pipeline
	Transport  Transport
	StartFrame *frames.StartFrame // optional; if nil, NewStartFrame() is used in Run
	done       chan struct{}
	mu         sync.Mutex
}

// NewRunner returns a Runner that will run the given pipeline with the transport.
// If start is non-nil, it is pushed as the StartFrame when Run starts; otherwise frames.NewStartFrame() is used.
func NewRunner(pl *Pipeline, tr Transport, start *frames.StartFrame) *Runner {
	return &Runner{Pipeline: pl, Transport: tr, StartFrame: start, done: make(chan struct{})}
}

// Run starts the pipeline and transport, feeds input frames into the pipeline, and sends pipeline output to transport.
// It blocks until ctx is cancelled or a fatal error occurs. Caller can push frames into the pipeline
// from another goroutine; output is typically sent to Transport.Output() by the last processor (sink).
func (r *Runner) Run(ctx context.Context) error {
	if r.Pipeline == nil || r.Transport == nil {
		return nil
	}
	logger.Info("pipeline runner: starting")
	if err := r.Pipeline.Setup(ctx); err != nil {
		return err
	}
	defer func() {
		logger.Info("pipeline runner: cleanup")
		r.Pipeline.Cleanup(ctx)
	}()

	if err := r.Transport.Start(ctx); err != nil {
		return err
	}
	defer r.Transport.Close()

	// Push StartFrame
	startFrame := r.StartFrame
	if startFrame == nil {
		startFrame = frames.NewStartFrame()
	}
	if err := r.Pipeline.Start(ctx, startFrame); err != nil {
		return err
	}

	inCh := r.Transport.Input()
	outCh := r.Transport.Output()
	if inCh == nil && outCh == nil {
		logger.Info("pipeline runner: no input/output channels, waiting for context done")
		<-ctx.Done()
		return ctx.Err()
	}

	// If we have an input channel, forward frames via a queue so that reading from
	// transport is never blocked by pipeline processing (e.g. Sink blocked on TTS output).
	// Reader goroutine drains transport into queueCh; worker goroutine drains queueCh into pipeline.
	if inCh != nil {
		logger.Info("pipeline runner: input channel active, forwarding frames to pipeline (queue cap=%d)", inputQueueCap)
		queueCh := make(chan frames.Frame, inputQueueCap)
		// Reader: transport -> queue (never blocks on pipeline)
		go func() {
			var inCount uint64
			for {
				select {
				case <-ctx.Done():
					close(queueCh)
					return
				case f, ok := <-inCh:
					if !ok {
						logger.Info("pipeline runner: input channel closed (received %d frames total)", inCount)
						close(queueCh)
						return
					}
					inCount++
					if inCount == 1 {
						logger.Info("pipeline runner: first frame from transport type=%s id=%d", f.FrameType(), f.ID())
					} else if inCount%25 == 0 {
						logger.Info("pipeline runner: frames from transport so far: %d (latest type=%s)", inCount, f.FrameType())
					}
					select {
					case <-ctx.Done():
						close(queueCh)
						return
					case queueCh <- f:
						// Log a sample of enqueued frames to the standard logger instead of a debug file.
						if inCount%25 == 0 || inCount <= 2 {
							logger.Info("pipeline runner: transport frame enqueued (count=%d, frameType=%s)", inCount, f.FrameType())
						}
					}
				}
			}
		}()
		// Worker: queue -> pipeline (may block; does not block reader)
		go func() {
			for f := range queueCh {
				if ctx.Err() != nil {
					return
				}
				_ = r.Pipeline.Push(ctx, f)
				if ef, ok := f.(*frames.ErrorFrame); ok && ef.Fatal {
					return
				}
				if _, ok := f.(*frames.CancelFrame); ok {
					return
				}
			}
		}()
	}

	// Block until context done
	<-ctx.Done()
	logger.Info("pipeline runner: context done, stopping")
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
