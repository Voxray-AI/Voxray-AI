package filters

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// NullFilter blocks all frames except lifecycle frames (Start, End, Cancel, Stop, Error).
type NullFilter struct {
	*processors.BaseProcessor
}

// NewNullFilter returns a filter that only passes lifecycle frames.
func NewNullFilter(name string) *NullFilter {
	if name == "" {
		name = "NullFilter"
	}
	return &NullFilter{BaseProcessor: processors.NewBaseProcessor(name)}
}

// ProcessFrame forwards only lifecycle frames; drops all others.
func (p *NullFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if !isLifecycleFrame(f) {
		return nil
	}
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

var _ processors.Processor = (*NullFilter)(nil)
