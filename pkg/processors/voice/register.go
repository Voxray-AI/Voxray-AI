package voice

import (
	"encoding/json"

	"voxray-go/pkg/audio/interruptions"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
)

// InterruptionControllerOptions describes JSON options for the
// "interruption_controller" processor when used via plugin_options.
type InterruptionControllerOptions struct {
	// Strategy selects the interruption strategy, e.g. "min_words".
	Strategy string `json:"strategy,omitempty"`
	// MinWords configures MinWordsStrategy when Strategy is "min_words".
	MinWords int `json:"min_words,omitempty"`
}

// NewInterruptionControllerFromOptions builds an InterruptionController from
// JSON plugin options.
func NewInterruptionControllerFromOptions(name string, opts json.RawMessage) processors.Processor {
	var o InterruptionControllerOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	strategy := interruptions.NewStrategy(o.Strategy, o.MinWords)
	return NewInterruptionController(name, strategy)
}

func init() {
	pipeline.RegisterProcessor("interruption_controller", func(name string, opts json.RawMessage) processors.Processor {
		return NewInterruptionControllerFromOptions(name, opts)
	})
}

