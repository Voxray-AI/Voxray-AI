package processors

import (
	"context"
	"sync"

	"voila-go/pkg/frames"
)

// QueuedItem holds a frame, direction, and optional callback.
type QueuedItem struct {
	Frame frames.Frame
	Dir   Direction
}

// FrameQueue is a queue that prioritizes system frames over data frames.
type FrameQueue struct {
	mu      sync.Mutex
	system  []QueuedItem
	data    []QueuedItem
	wait    chan struct{}
	closed  bool
}

// NewFrameQueue returns a new FrameQueue.
func NewFrameQueue() *FrameQueue {
	return &FrameQueue{wait: make(chan struct{}, 1)}
}

// Put adds an item; system frames go to system queue, others to data queue.
func (q *FrameQueue) Put(item QueuedItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	if isSystemFrame(item.Frame) {
		q.system = append(q.system, item)
	} else {
		q.data = append(q.data, item)
	}
	select {
	case q.wait <- struct{}{}:
	default:
	}
}

// Get blocks until an item is available; returns system frames first.
func (q *FrameQueue) Get(ctx context.Context) (QueuedItem, bool) {
	for {
		q.mu.Lock()
		if len(q.system) > 0 {
			item := q.system[0]
			q.system = q.system[1:]
			q.mu.Unlock()
			return item, true
		}
		if len(q.data) > 0 {
			item := q.data[0]
			q.data = q.data[1:]
			q.mu.Unlock()
			return item, true
		}
		if q.closed {
			q.mu.Unlock()
			return QueuedItem{}, false
		}
		q.mu.Unlock()

		select {
		case <-q.wait:
			continue
		case <-ctx.Done():
			return QueuedItem{}, false
		}
	}
}

// Close closes the queue; subsequent Get will return false.
func (q *FrameQueue) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	select {
	case q.wait <- struct{}{}:
	default:
	}
}

func isSystemFrame(f frames.Frame) bool {
	if f == nil {
		return false
	}
	t := f.FrameType()
	return t == "StartFrame" || t == "CancelFrame" || t == "ErrorFrame"
}

// QueuedProcessor runs a processor with an input queue and a single goroutine.
type QueuedProcessor struct {
	*BaseProcessor
	Queue *FrameQueue
}

// NewQueuedProcessor wraps a processor and runs ProcessFrame in a goroutine from the queue.
func NewQueuedProcessor(base *BaseProcessor, queue *FrameQueue) *QueuedProcessor {
	return &QueuedProcessor{BaseProcessor: base, Queue: queue}
}

// Run processes items from the queue until context is cancelled or queue is closed.
func (q *QueuedProcessor) Run(ctx context.Context) {
	for {
		item, ok := q.Queue.Get(ctx)
		if !ok {
			return
		}
		if err := q.ProcessFrame(ctx, item.Frame, item.Dir); err != nil {
			// Push error upstream if we have prev
			if q.Prev() != nil {
				_ = q.PushUpstream(ctx, frames.NewErrorFrame(err.Error(), false, q.Name()))
			}
		}
	}
}
