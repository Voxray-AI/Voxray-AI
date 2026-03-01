package filters

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// IdentityFilter forwards all frames unchanged (pass-through).
type IdentityFilter struct {
	*processors.BaseProcessor
}

// NewIdentityFilter returns a pass-through filter.
func NewIdentityFilter(name string) *IdentityFilter {
	if name == "" {
		name = "IdentityFilter"
	}
	return &IdentityFilter{BaseProcessor: processors.NewBaseProcessor(name)}
}

// ProcessFrame forwards the frame unchanged.
func (p *IdentityFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

var _ processors.Processor = (*IdentityFilter)(nil)
