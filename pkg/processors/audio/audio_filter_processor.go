package audio

import (
	"context"

	audiofilters "voxray-go/pkg/audio/filters"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// AudioFilterProcessor applies a configured chain of audio filters to raw
// PCM audio frames before they reach downstream processors (e.g. VAD, STT).
type AudioFilterProcessor struct {
	*processors.BaseProcessor

	Chain *audiofilters.Chain
}

// NewAudioFilterProcessor constructs an AudioFilterProcessor with the given
// filter chain. When chain is nil, the processor is a no-op pass-through.
func NewAudioFilterProcessor(name string, chain *audiofilters.Chain) *AudioFilterProcessor {
	if name == "" {
		name = "AudioFilter"
	}
	return &AudioFilterProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		Chain:         chain,
	}
}

// ProcessFrame applies filters to audio-carrying frames and forwards all
// frames unchanged otherwise.
func (p *AudioFilterProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch frame := f.(type) {
	case *frames.AudioRawFrame:
		p.filterAudio(frame)
	case *frames.OutputAudioRawFrame:
		p.filterAudio(&frame.AudioRawFrame)
	case *frames.TTSAudioRawFrame:
		p.filterAudio(&frame.OutputAudioRawFrame.AudioRawFrame)
	}

	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

// filterAudio applies the configured chain to a single AudioRawFrame in place.
func (p *AudioFilterProcessor) filterAudio(ar *frames.AudioRawFrame) {
	if p.Chain == nil || ar == nil || len(ar.Audio) == 0 {
		return
	}
	channels := ar.NumChannels
	if channels <= 0 {
		channels = 1
	}
	sampleRate := ar.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	filtered := p.Chain.Apply(ar.Audio, sampleRate, channels)
	if filtered == nil {
		return
	}
	ar.Audio = filtered
	// Recompute NumFrames based on new buffer length.
	ar.NumFrames = len(filtered) / (channels * 2)
}

var _ processors.Processor = (*AudioFilterProcessor)(nil)

