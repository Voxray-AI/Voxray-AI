package voice

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/services"
)

// TTSProcessor turns LLMTextFrame or TTSSpeakFrame into TTSAudioRawFrame using a TTSService.
type TTSProcessor struct {
	*processors.BaseProcessor
	TTS        services.TTSService
	SampleRate int
}

// NewTTSProcessor returns a processor that speaks text and pushes TTSAudioRawFrame(s) downstream.
func NewTTSProcessor(name string, tts services.TTSService, sampleRate int) *TTSProcessor {
	if name == "" {
		name = "TTS"
	}
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	return &TTSProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		TTS:           tts,
		SampleRate:    sampleRate,
	}
}

// ProcessFrame speaks LLMTextFrame or TTSSpeakFrame and forwards other frames.
func (p *TTSProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	var text string
	switch t := f.(type) {
	case *frames.LLMTextFrame:
		text = t.Text
	case *frames.TTSSpeakFrame:
		text = t.Text
	default:
		return p.PushDownstream(ctx, f)
	}
	if text == "" {
		return nil
	}

	audioFrames, err := p.TTS.Speak(ctx, text, p.SampleRate)
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	for _, af := range audioFrames {
		_ = p.PushDownstream(ctx, af)
	}
	return nil
}
