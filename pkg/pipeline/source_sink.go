package pipeline

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// Source is a processor that reads frames from a channel and pushes them downstream.
type Source struct {
	*processors.BaseProcessor
	In <-chan frames.Frame
}

// NewSource returns a Source that reads from ch.
func NewSource(name string, ch <-chan frames.Frame) *Source {
	if name == "" {
		name = "Source"
	}
	return &Source{BaseProcessor: processors.NewBaseProcessor(name), In: ch}
}

// Run reads from In and pushes to next until context is done or channel closed.
func (s *Source) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-s.In:
			if !ok {
				return
			}
			_ = s.PushDownstream(ctx, f)
		}
	}
}

// Sink is a processor that forwards all frames to a channel (for transport output).
type Sink struct {
	*processors.BaseProcessor
	Out chan<- frames.Frame
}

// NewSink returns a Sink that writes to ch.
func NewSink(name string, ch chan<- frames.Frame) *Sink {
	if name == "" {
		name = "Sink"
	}
	return &Sink{BaseProcessor: processors.NewBaseProcessor(name), Out: ch}
}

// ProcessFrame forwards the frame to Out and does not call next (end of chain).
func (s *Sink) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if s.Prev() != nil {
			return s.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if s.Out != nil {
		select {
		case s.Out <- f:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
