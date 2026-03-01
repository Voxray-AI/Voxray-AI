package filters

import (
	"context"
	"encoding/json"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// FunctionFilterPredicate returns true if the frame should pass. Used for programmatic FunctionFilter.
type FunctionFilterPredicate func(f frames.Frame) bool

// FunctionFilterOptions is the JSON shape for plugin_options["function_filter"].
// The actual filter predicate cannot be set via config; use NewFunctionFilter for that.
type FunctionFilterOptions struct {
	Direction          string `json:"direction"`           // "downstream", "upstream", or "" for both
	FilterSystemFrames bool   `json:"filter_system_frames"`
}

// FunctionFilter forwards frames based on a predicate. Lifecycle frames (Start, End, Cancel, Stop) always pass.
// Optionally restrict to one direction and optionally filter system frames (e.g. ErrorFrame).
type FunctionFilter struct {
	*processors.BaseProcessor
	Filter             FunctionFilterPredicate
	Direction           processors.Direction // 0 = both
	FilterSystemFrames bool
}

// NewFunctionFilter returns a filter that uses the given predicate. Lifecycle frames always pass.
// If direction is non-zero, only that direction is filtered; the opposite direction always forwards.
func NewFunctionFilter(name string, filter FunctionFilterPredicate, direction processors.Direction, filterSystemFrames bool) *FunctionFilter {
	if name == "" {
		name = "FunctionFilter"
	}
	if filter == nil {
		filter = func(frames.Frame) bool { return true }
	}
	return &FunctionFilter{
		BaseProcessor:       processors.NewBaseProcessor(name),
		Filter:             filter,
		Direction:          direction,
		FilterSystemFrames: filterSystemFrames,
	}
}

// NewFunctionFilterFromOptions builds a FunctionFilter from plugin_options. Predicate is always all-pass when created from config.
func NewFunctionFilterFromOptions(name string, opts json.RawMessage) *FunctionFilter {
	var o FunctionFilterOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	var dir processors.Direction
	switch o.Direction {
	case "upstream":
		dir = processors.Upstream
	case "downstream":
		dir = processors.Downstream
	}
	return NewFunctionFilter(name, func(frames.Frame) bool { return true }, dir, o.FilterSystemFrames)
}

// ProcessFrame forwards the frame if it is lifecycle, or (when direction matches) if the predicate returns true.
func (p *FunctionFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if isLifecycleFrame(f) {
		return p.BaseProcessor.ProcessFrame(ctx, f, dir)
	}
	if p.Direction != 0 && dir != p.Direction {
		return p.BaseProcessor.ProcessFrame(ctx, f, dir)
	}
	if !p.Filter(f) {
		return nil
	}
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

var _ processors.Processor = (*FunctionFilter)(nil)
