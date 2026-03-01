package frameworks

import (
	"encoding/json"

	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("external_chain", func(name string, opts json.RawMessage) processors.Processor {
		return NewExternalChainProcessorFromOptions(name, opts)
	})
}
