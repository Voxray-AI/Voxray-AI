package filters

import (
	"encoding/json"

	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("frame_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewFrameFilterFromOptions(name, opts)
	})
	pipeline.RegisterProcessor("function_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewFunctionFilterFromOptions(name, opts)
	})
	pipeline.RegisterProcessor("identity_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewIdentityFilter(name)
	})
	pipeline.RegisterProcessor("null_filter", func(name string, _ json.RawMessage) processors.Processor {
		return NewNullFilter(name)
	})
	pipeline.RegisterProcessor("stt_mute_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewSTTMuteFilterFromOptions(name, opts)
	})
	pipeline.RegisterProcessor("wake_check_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewWakeCheckFilterFromOptions(name, opts)
	})
	pipeline.RegisterProcessor("wake_notifier_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewWakeNotifierFilterFromOptions(name, opts)
	})
}
