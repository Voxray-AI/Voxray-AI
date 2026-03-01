package filters

import (
	"context"
	"encoding/json"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// FrameFilterOptions is the JSON shape for plugin_options["frame_filter"].
type FrameFilterOptions struct {
	AllowedTypes []string `json:"allowed_types"`
}

// FrameFilter allows only frames whose FrameType() is in the allowed set.
// Lifecycle frames (Start, End, Cancel, Stop, Error) always pass through.
type FrameFilter struct {
	*processors.BaseProcessor
	allowed map[string]struct{}
}

// NewFrameFilter returns a filter that forwards only frames with type in allowedTypes, plus lifecycle frames.
func NewFrameFilter(name string, allowedTypes []string) *FrameFilter {
	if name == "" {
		name = "FrameFilter"
	}
	allowed := make(map[string]struct{})
	for _, t := range allowedTypes {
		allowed[t] = struct{}{}
	}
	return &FrameFilter{
		BaseProcessor: processors.NewBaseProcessor(name),
		allowed:       allowed,
	}
}

// NewFrameFilterFromOptions builds a FrameFilter from plugin_options JSON. If opts is nil or empty, allows no data frames (only lifecycle).
func NewFrameFilterFromOptions(name string, opts json.RawMessage) *FrameFilter {
	var o FrameFilterOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	return NewFrameFilter(name, o.AllowedTypes)
}

// ProcessFrame forwards the frame only if it is lifecycle or its type is in the allowed set.
func (p *FrameFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if isLifecycleFrame(f) {
		return p.BaseProcessor.ProcessFrame(ctx, f, dir)
	}
	if _, ok := p.allowed[f.FrameType()]; ok {
		return p.BaseProcessor.ProcessFrame(ctx, f, dir)
	}
	return nil
}

var _ processors.Processor = (*FrameFilter)(nil)
