package filters

import (
	"context"
	"encoding/json"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/sync/notifier"
)

// WakeNotifierFilterOptions is the JSON shape for plugin_options["wake_notifier_filter"].
// Notifier must be set programmatically via NewWakeNotifierFilter; when created from config with no notifier, Notify is never called.
type WakeNotifierFilterOptions struct {
	Types []string `json:"types"` // frame types that can trigger notification (e.g. ["TranscriptionFrame"])
}

// WakeNotifierFilter forwards all frames; when a frame's type is in the allowed set and the predicate returns true, it calls notifier.Notify().
type WakeNotifierFilter struct {
	*processors.BaseProcessor
	notifier *notifier.Notifier
	types   map[string]struct{}
	predicate func(frames.Frame) bool
}

// NewWakeNotifierFilter returns a filter that forwards every frame and optionally triggers n when frame type is in types and predicate(f) is true.
// If predicate is nil, any matching type triggers Notify. If notifier is nil, Notify is never called.
func NewWakeNotifierFilter(name string, n *notifier.Notifier, types []string, predicate func(frames.Frame) bool) *WakeNotifierFilter {
	if name == "" {
		name = "WakeNotifierFilter"
	}
	tmap := make(map[string]struct{})
	for _, t := range types {
		tmap[t] = struct{}{}
	}
	return &WakeNotifierFilter{
		BaseProcessor: processors.NewBaseProcessor(name),
		notifier:      n,
		types:         tmap,
		predicate:     predicate,
	}
}

// NewWakeNotifierFilterFromOptions builds from plugin_options. Notifier is nil (Notify never called); set via programmatic constructor for real use.
func NewWakeNotifierFilterFromOptions(name string, opts json.RawMessage) *WakeNotifierFilter {
	var o WakeNotifierFilterOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	return NewWakeNotifierFilter(name, nil, o.Types, nil)
}

// ProcessFrame forwards the frame; if type matches and (predicate is nil or predicate(f)), calls notifier.Notify().
func (p *WakeNotifierFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if p.notifier != nil {
		if _, ok := p.types[f.FrameType()]; ok {
			if p.predicate == nil || p.predicate(f) {
				p.notifier.Notify()
			}
		}
	}
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

var _ processors.Processor = (*WakeNotifierFilter)(nil)
