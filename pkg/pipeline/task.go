// Package pipeline provides Task and PipelineTask for queue-based pipeline execution.
package pipeline

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
)

// TaskParams holds optional parameters for running a task (e.g. for future use).
type TaskParams struct{}

// Task is the interface for pipeline task execution with queue, lifecycle, and cancellation.
type Task interface {
	// Name returns the task name.
	Name() string
	// Run runs the task until context is cancelled or the task finishes. It blocks.
	Run(ctx context.Context, params TaskParams) error
	// QueueFrame queues a single frame for processing.
	QueueFrame(ctx context.Context, f frames.Frame) error
	// QueueFrames queues multiple frames for processing.
	QueueFrames(ctx context.Context, frames []frames.Frame) error
	// StopWhenDone schedules the task to stop after processing all queued frames (e.g. pushes EndFrame).
	StopWhenDone(ctx context.Context) error
	// Cancel stops the task immediately.
	Cancel(ctx context.Context) error
	// HasFinished returns true when the task has reached a terminal state.
	HasFinished() bool
}

// DefaultPipelineTaskQueueSize is the buffer size for the frame queue. When the queue is full,
// QueueFrame blocks (back-pressure). This avoids unbounded growth and keeps overflow behavior explicit.
const DefaultPipelineTaskQueueSize = 64

// PipelineTask runs a pipeline with a frame queue. Frames are queued and drained into the pipeline.
// StopWhenDone pushes an EndFrame and stops when the pipeline has drained; Cancel pushes a CancelFrame.
// Overflow policy: when the queue is full, QueueFrame blocks until the drain goroutine receives (back-pressure).
type PipelineTask struct {
	name     string
	pipeline *Pipeline
	queue    chan frames.Frame
	mu       sync.Mutex
	finished bool
	cancel   context.CancelFunc
}

// NewPipelineTask creates a task that runs the given pipeline. The queue is buffered (DefaultPipelineTaskQueueSize)
// so producers can enqueue without blocking until the buffer is full; when full, QueueFrame blocks (back-pressure).
// Call Run in a goroutine and then QueueFrame from other goroutines.
func NewPipelineTask(name string, pl *Pipeline) *PipelineTask {
	return NewPipelineTaskWithQueueSize(name, pl, DefaultPipelineTaskQueueSize)
}

// NewPipelineTaskWithQueueSize is like NewPipelineTask but sets the queue buffer size.
// When the queue is full, QueueFrame blocks (back-pressure). Use queueSize 0 for unbuffered (original behavior).
func NewPipelineTaskWithQueueSize(name string, pl *Pipeline, queueSize int) *PipelineTask {
	if name == "" {
		name = "PipelineTask"
	}
	if queueSize < 0 {
		queueSize = DefaultPipelineTaskQueueSize
	}
	return &PipelineTask{name: name, pipeline: pl, queue: make(chan frames.Frame, queueSize)}
}

// Name implements Task.
func (t *PipelineTask) Name() string { return t.name }

// Run implements Task. It starts a goroutine that drains the queue into the pipeline, then blocks until ctx is done.
// The pipeline is not started (no StartFrame) by Run; the caller should queue a StartFrame first if needed.
func (t *PipelineTask) Run(ctx context.Context, params TaskParams) error {
	if t.pipeline == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, t.cancel = context.WithCancel(ctx)
	defer t.cancel()

	if err := t.pipeline.Setup(ctx); err != nil {
		return err
	}
	defer func() {
		t.pipeline.Cleanup(ctx)
		t.mu.Lock()
		t.finished = true
		t.mu.Unlock()
	}()

	// Goroutine that drains queue into pipeline
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case f, ok := <-t.queue:
				if !ok {
					return
				}
				if err := t.pipeline.Push(ctx, f); err != nil && err != context.Canceled {
					logger.Info("pipeline task: push error %v", err)
				}
				if _, ok := f.(*frames.CancelFrame); ok {
					return
				}
				if ef, ok := f.(*frames.ErrorFrame); ok && ef.Fatal {
					return
				}
			}
		}
	}()

	<-ctx.Done()
	// Allow drain to exit: close queue and nil it so QueueFrame no-ops after Run returns
	t.mu.Lock()
	close(t.queue)
	t.queue = nil
	t.mu.Unlock()
	<-done
	return ctx.Err()
}

// QueueFrame implements Task. It sends the frame to the queue. When the queue buffer is full, it blocks (back-pressure).
// After Run has returned, QueueFrame is a no-op.
func (t *PipelineTask) QueueFrame(ctx context.Context, f frames.Frame) error {
	t.mu.Lock()
	q := t.queue
	t.mu.Unlock()
	if q == nil {
		return nil // task stopped or not started
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q <- f:
		return nil
	}
}

// QueueFrames implements Task.
func (t *PipelineTask) QueueFrames(ctx context.Context, frames []frames.Frame) error {
	for _, f := range frames {
		if err := t.QueueFrame(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

// StopWhenDone implements Task. It queues an EndFrame so the pipeline drains and then stops.
func (t *PipelineTask) StopWhenDone(ctx context.Context) error {
	return t.QueueFrame(ctx, frames.NewEndFrame())
}

// Cancel implements Task. It queues a CancelFrame and cancels the context.
func (t *PipelineTask) Cancel(ctx context.Context) error {
	_ = t.QueueFrame(ctx, frames.NewCancelFrame("task cancelled"))
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// HasFinished implements Task.
func (t *PipelineTask) HasFinished() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.finished
}

var _ Task = (*PipelineTask)(nil)
