package audio

import (
	"encoding/json"
	"time"

	"voila-go/pkg/audio/vad"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("vad", func(name string, _ json.RawMessage) processors.Processor {
		a := vad.NewEnergyAnalyzer(vad.Params{})
		return NewVADProcessor(name, a, 200*time.Millisecond)
	})
	pipeline.RegisterProcessor("audio_buffer", func(name string, _ json.RawMessage) processors.Processor {
		return NewAudioBufferProcessor(name, 0, 1, 0, false)
	})
}
