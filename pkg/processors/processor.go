// Package processors provides the frame processor abstraction and built-in processors.
package processors

import (
	"context"

	"voxray-go/pkg/frames"
)

// Direction is the frame flow direction.
type Direction int

const (
	Downstream Direction = 1
	Upstream   Direction = 2
)

// Processor processes frames and can be linked into a pipeline.
type Processor interface {
	ProcessFrame(ctx context.Context, f frames.Frame, dir Direction) error
	SetNext(p Processor)
	SetPrev(p Processor)
	Setup(ctx context.Context) error
	Cleanup(ctx context.Context) error
	Name() string
}

// BaseProcessor provides next/prev linking and default forward behavior.
type BaseProcessor struct {
	name string
	next Processor
	prev Processor
}

// NewBaseProcessor returns a BaseProcessor with the given name.
func NewBaseProcessor(name string) *BaseProcessor {
	return &BaseProcessor{name: name}
}

// Name returns the processor name.
func (b *BaseProcessor) Name() string {
	if b.name != "" {
		return b.name
	}
	return "BaseProcessor"
}

// SetNext sets the next processor in the pipeline.
func (b *BaseProcessor) SetNext(p Processor) { b.next = p }

// SetPrev sets the previous processor in the pipeline.
func (b *BaseProcessor) SetPrev(p Processor) { b.prev = p }

// Next returns the next processor.
func (b *BaseProcessor) Next() Processor { return b.next }

// Prev returns the previous processor.
func (b *BaseProcessor) Prev() Processor { return b.prev }

// Setup is a no-op for BaseProcessor; override in embeddings.
func (b *BaseProcessor) Setup(ctx context.Context) error { return nil }

// Cleanup is a no-op for BaseProcessor; override in embeddings.
func (b *BaseProcessor) Cleanup(ctx context.Context) error { return nil }

// ProcessFrame forwards the frame to next (downstream) or prev (upstream). Override in embeddings.
func (b *BaseProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir Direction) error {
	if dir == Downstream && b.next != nil {
		return b.next.ProcessFrame(ctx, f, dir)
	}
	if dir == Upstream && b.prev != nil {
		return b.prev.ProcessFrame(ctx, f, dir)
	}
	return nil
}

// PushDownstream forwards f to the next processor.
func (b *BaseProcessor) PushDownstream(ctx context.Context, f frames.Frame) error {
	return b.ProcessFrame(ctx, f, Downstream)
}

// PushUpstream forwards f to the previous processor.
func (b *BaseProcessor) PushUpstream(ctx context.Context, f frames.Frame) error {
	return b.ProcessFrame(ctx, f, Upstream)
}
