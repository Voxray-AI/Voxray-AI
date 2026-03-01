package rtvi

import (
	"encoding/json"

	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("rtvi", func(name string, opts json.RawMessage) processors.Processor {
		return NewRTVIProcessorFromOptions(name, opts)
	})
}
