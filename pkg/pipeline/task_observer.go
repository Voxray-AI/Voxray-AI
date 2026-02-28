// Package pipeline provides TaskObserver, a proxy observer that processes observer events asynchronously.
package pipeline

import (
	"context"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/observers"
	"voila-go/pkg/processors"
)

const defaultTaskObserverQueueSize = 256

// frameEvent is a single observer event to be processed asynchronously.
type frameEvent struct {
	processorName string
	frame         frames.Frame
	dir           processors.Direction
}

// TaskObserver is a proxy observer that queues OnFrameProcessed events and processes them
// in a background goroutine so slow observers do not block the pipeline.
type TaskObserver struct {
	observers []observers.Observer
	queue     chan frameEvent
	done      chan struct{}
	mu        sync.Mutex
}

// NewTaskObserver creates a TaskObserver that forwards events to the given observers asynchronously.
// Queue size defaults to defaultTaskObserverQueueSize. Start must be called before use.
func NewTaskObserver(observersList []observers.Observer, queueSize int) *TaskObserver {
	if queueSize <= 0 {
		queueSize = defaultTaskObserverQueueSize
	}
	return &TaskObserver{
		observers: observersList,
		queue:     make(chan frameEvent, queueSize),
		done:      make(chan struct{}),
	}
}

// Start starts the background goroutine that drains the queue and notifies all observers.
func (t *TaskObserver) Start() {
	go t.run()
}

// run drains the queue and calls each observer. Stops when the queue is closed.
func (t *TaskObserver) run() {
	for ev := range t.queue {
		for _, o := range t.observers {
			if o != nil {
				o.OnFrameProcessed(ev.processorName, ev.frame, ev.dir)
			}
		}
	}
	close(t.done)
}

// Stop closes the queue and waits for the drain goroutine to finish.
func (t *TaskObserver) Stop(ctx context.Context) {
	t.mu.Lock()
	ch := t.queue
	t.queue = nil
	t.mu.Unlock()
	if ch != nil {
		close(ch)
		select {
		case <-t.done:
		case <-ctx.Done():
		}
	}
}

// OnFrameProcessed implements observers.Observer. It queues the event; if the queue is full, it drops.
func (t *TaskObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	t.mu.Lock()
	q := t.queue
	t.mu.Unlock()
	if q == nil {
		return
	}
	ev := frameEvent{processorName: processorName, frame: f, dir: dir}
	select {
	case q <- ev:
	default:
		// Queue full, drop to avoid blocking
	}
}

// AddObserver adds an observer to the list. Safe to call before or after Start.
func (t *TaskObserver) AddObserver(o observers.Observer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observers = append(t.observers, o)
}

var _ observers.Observer = (*TaskObserver)(nil)
