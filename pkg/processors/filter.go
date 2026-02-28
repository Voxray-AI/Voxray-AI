// Package processors provides FilterProcessor for conditional frame forwarding.
package processors

import (
	"context"

	"voila-go/pkg/frames"
)

// FilterFunc returns true if the frame should be forwarded in the given direction.
type FilterFunc func(ctx context.Context, f frames.Frame, dir Direction) bool

// FilterProcessor forwards frames only when the filter function returns true.
// Used by ServiceSwitcher to gate frames so only the active service receives them.
type FilterProcessor struct {
	*BaseProcessor
	Filter  FilterFunc
	dir     Direction // filter applies to this direction; opposite direction is always forwarded
	name    string
}

// NewFilterProcessor returns a processor that forwards downstream frames when filter(ctx, f, Downstream) is true,
// and forwards upstream frames when filter(ctx, f, Upstream) is true. If filter is nil, all frames pass.
func NewFilterProcessor(name string, filter FilterFunc) *FilterProcessor {
	if name == "" {
		name = "Filter"
	}
	return &FilterProcessor{BaseProcessor: NewBaseProcessor(name), Filter: filter, name: name}
}

// ProcessFrame implements Processor. Forwards the frame only if the filter returns true for this direction.
func (fp *FilterProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir Direction) error {
	if fp.Filter != nil && !fp.Filter(ctx, f, dir) {
		return nil
	}
	if dir == Downstream && fp.Next() != nil {
		return fp.Next().ProcessFrame(ctx, f, dir)
	}
	if dir == Upstream && fp.Prev() != nil {
		return fp.Prev().ProcessFrame(ctx, f, dir)
	}
	return nil
}

// Name returns the processor name.
func (fp *FilterProcessor) Name() string {
	if fp.name != "" {
		return fp.name
	}
	return "Filter"
}

var _ Processor = (*FilterProcessor)(nil)
