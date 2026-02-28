// Package observers provides optional frame processing observers for metrics and logging.
package observers

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// Ensure ObservingProcessor implements processors.Processor.
var _ processors.Processor = (*ObservingProcessor)(nil)

// Observer is notified when frames are processed or pushed.
type Observer interface {
	OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction)
}

// NoopObserver does nothing.
type NoopObserver struct{}

func (NoopObserver) OnFrameProcessed(string, frames.Frame, processors.Direction) {}

// CompositeObserver forwards OnFrameProcessed to multiple observers.
type CompositeObserver struct {
	Observers []Observer
}

// NewCompositeObserver returns an observer that delegates to all observers in order.
func NewCompositeObserver(observers ...Observer) *CompositeObserver {
	return &CompositeObserver{Observers: observers}
}

// OnFrameProcessed implements Observer.
func (c *CompositeObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	for _, o := range c.Observers {
		if o != nil {
			o.OnFrameProcessed(processorName, f, dir)
		}
	}
}

// ObserverWithMetrics wraps an observer and updates metrics (latency, token/char count).
type ObserverWithMetrics struct {
	Observer Observer
	Metrics  *Metrics
	start    map[uint64]int64 // frame ID -> start time nanos (optional latency)
}

// NewObserverWithMetrics returns an observer that delegates and updates Metrics.
func NewObserverWithMetrics(o Observer, m *Metrics) *ObserverWithMetrics {
	if m == nil {
		m = NewMetrics()
	}
	return &ObserverWithMetrics{Observer: o, Metrics: m, start: make(map[uint64]int64)}
}

// OnFrameProcessed records frame and optionally latency; delegates to Observer.
func (ob *ObserverWithMetrics) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	ob.Metrics.IncFrames()
	if ob.Observer != nil {
		ob.Observer.OnFrameProcessed(processorName, f, dir)
	}
}

// ObservingProcessor wraps a processor and notifies an observer for each frame (for metrics/logging).
type ObservingProcessor struct {
	Inner    processors.Processor
	Observer Observer
}

// ProcessFrame forwards to the inner processor and notifies the observer.
func (o *ObservingProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if o.Observer != nil {
		o.Observer.OnFrameProcessed(o.Inner.Name(), f, dir)
	}
	return o.Inner.ProcessFrame(ctx, f, dir)
}

func (o *ObservingProcessor) SetNext(p processors.Processor)   { o.Inner.SetNext(p) }
func (o *ObservingProcessor) SetPrev(p processors.Processor)   { o.Inner.SetPrev(p) }
func (o *ObservingProcessor) Setup(ctx context.Context) error   { return o.Inner.Setup(ctx) }
func (o *ObservingProcessor) Cleanup(ctx context.Context) error   { return o.Inner.Cleanup(ctx) }
func (o *ObservingProcessor) Name() string { return o.Inner.Name() }

// WrapWithObserver returns a processor that notifies observer then forwards to p.
func WrapWithObserver(p processors.Processor, observer Observer) processors.Processor {
	if observer == nil {
		return p
	}
	return &ObservingProcessor{Inner: p, Observer: observer}
}
