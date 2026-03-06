package audio

import (
	"encoding/json"
	"time"

	audiofilters "voxray-go/pkg/audio/filters"
	"voxray-go/pkg/audio/vad"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
)

// AudioFilterProcessorOptions describes JSON options for the
// "audio_filter" processor when used via plugin_options.
type AudioFilterProcessorOptions struct {
	Filters []struct {
		// Type selects the filter implementation, e.g. "gain".
		Type string `json:"type"`
		// Gain configures GainFilter when Type is "gain".
		Gain float64 `json:"gain,omitempty"`
	} `json:"filters,omitempty"`
}

// NewAudioFilterProcessorFromOptions builds an AudioFilterProcessor from
// JSON plugin options.
func NewAudioFilterProcessorFromOptions(name string, opts json.RawMessage) processors.Processor {
	var o AudioFilterProcessorOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	var fs []audiofilters.Filter
	for _, cfg := range o.Filters {
		switch cfg.Type {
		case "gain":
			fs = append(fs, audiofilters.NewGainFilter(cfg.Gain))
		}
	}
	var chain *audiofilters.Chain
	if len(fs) > 0 {
		chain = audiofilters.NewChain(fs...)
	}
	return NewAudioFilterProcessor(name, chain)
}

func init() {
	pipeline.RegisterProcessor("vad", func(name string, _ json.RawMessage) processors.Processor {
		a := vad.NewEnergyAnalyzer(vad.Params{})
		return NewVADProcessor(name, a, 200*time.Millisecond)
	})
	pipeline.RegisterProcessor("audio_buffer", func(name string, _ json.RawMessage) processors.Processor {
		return NewAudioBufferProcessor(name, 0, 1, 0, false)
	})
	pipeline.RegisterProcessor("audio_filter", func(name string, opts json.RawMessage) processors.Processor {
		return NewAudioFilterProcessorFromOptions(name, opts)
	})
}
