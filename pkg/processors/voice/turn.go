// Package voice provides processors that wire STT, LLM, and TTS into a pipeline.
package voice

import (
	"context"
	"time"

	"voila-go/pkg/audio"
	"voila-go/pkg/audio/turn"
	"voila-go/pkg/audio/vad"
	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// TurnProcessor buffers AudioRawFrame chunks, runs VAD and turn detection, and forwards
// concatenated audio downstream only when the turn is complete (end of speech).
type TurnProcessor struct {
	*processors.BaseProcessor
	VAD        vad.Detector
	Analyzer   turn.Analyzer
	SampleRate int
	Channels   int

	// buffer accumulates audio for the current turn until Complete
	buffer []byte
}

// NewTurnProcessor returns a processor that buffers audio and forwards one segment per turn.
func NewTurnProcessor(name string, v vad.Detector, a turn.Analyzer, sampleRate, channels int) *TurnProcessor {
	if name == "" {
		name = "Turn"
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	a.SetSampleRate(sampleRate)
	return &TurnProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		VAD:           v,
		Analyzer:      a,
		SampleRate:    sampleRate,
		Channels:      channels,
	}
}

// ProcessFrame buffers AudioRawFrame, runs VAD and turn detection; on turn complete pushes audio downstream.
func (p *TurnProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	// Pass through non-audio frames; on cancel clear turn state
	switch f.(type) {
	case *frames.CancelFrame:
		p.buffer = nil
		p.Analyzer.Clear()
		return p.PushDownstream(ctx, f)
	case *frames.AudioRawFrame:
		// continue below
	default:
		return p.PushDownstream(ctx, f)
	}

	ar := f.(*frames.AudioRawFrame)
	chunk := ar.Audio
	if len(chunk) == 0 {
		return nil
	}

	af := audio.Frame{
		Data:        chunk,
		SampleRate:  p.SampleRate,
		NumChannels: p.Channels,
		Timestamp:   time.Now(),
	}
	isSpeech, err := p.VAD.IsSpeech(af)
	if err != nil {
		isSpeech = false
	}

	p.buffer = append(p.buffer, chunk...)
	state := p.Analyzer.AppendAudio(chunk, isSpeech)

	if state == turn.Complete {
		// Push accumulated audio as one AudioRawFrame for this turn
		audioCopy := make([]byte, len(p.buffer))
		copy(audioCopy, p.buffer)
		out := frames.NewAudioRawFrame(audioCopy, p.SampleRate, p.Channels, 0)
		p.buffer = nil
		p.Analyzer.Clear()
		return p.PushDownstream(ctx, out)
	}

	return nil
}
