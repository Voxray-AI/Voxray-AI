// Package audio provides audio processors (VAD, buffer/merge/turn callbacks).
package audio

import (
	"context"
	"time"

	"voxray-go/pkg/audio/vad"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// VADProcessor processes audio frames through voice activity detection and
// pushes VAD-related frames downstream: VADUserStartedSpeakingFrame,
// VADUserStoppedSpeakingFrame, and optionally UserSpeakingFrame periodically
// while speech is detected.
type VADProcessor struct {
	*processors.BaseProcessor
	Analyzer             vad.Analyzer
	SpeechActivityPeriod time.Duration

	prevState   vad.State
	lastActivity time.Time
}

// NewVADProcessor returns a VADProcessor that uses the given analyzer.
// speechActivityPeriod is the minimum interval between UserSpeakingFrame pushes
// while speech is detected; zero defaults to 200ms.
func NewVADProcessor(name string, analyzer vad.Analyzer, speechActivityPeriod time.Duration) *VADProcessor {
	if name == "" {
		name = "VAD"
	}
	if speechActivityPeriod <= 0 {
		speechActivityPeriod = 200 * time.Millisecond
	}
	return &VADProcessor{
		BaseProcessor:        processors.NewBaseProcessor(name),
		Analyzer:             analyzer,
		SpeechActivityPeriod: speechActivityPeriod,
		prevState:            vad.StateQuiet,
	}
}

// ProcessFrame forwards the frame downstream first, then runs VAD on audio frames
// and pushes VAD start/stop/activity frames on state transitions.
func (p *VADProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	// Forward first so StartFrame and audio flow immediately
	if dir == processors.Downstream && p.Next() != nil {
		if err := p.Next().ProcessFrame(ctx, f, dir); err != nil {
			return err
		}
	}

	if dir != processors.Downstream {
		return nil
	}

	switch frame := f.(type) {
	case *frames.StartFrame:
		if frame.AudioInSampleRate > 0 {
			p.Analyzer.SetSampleRate(frame.AudioInSampleRate)
		}
		return nil
	case *frames.VADParamsUpdateFrame:
		params := p.Analyzer.Params()
		if frame.StopSecs > 0 {
			params.StopSecs = frame.StopSecs
		}
		if frame.StartSecs > 0 {
			params.StartSecs = frame.StartSecs
		}
		p.Analyzer.SetParams(params)
		return nil
	case *frames.AudioRawFrame:
		if len(frame.Audio) == 0 {
			return nil
		}
		state, _, _, _ := p.Analyzer.Analyze(frame.Audio)
		now := time.Now()

		if state == vad.StateSpeaking && p.prevState != vad.StateSpeaking {
			params := p.Analyzer.Params()
			_ = p.PushDownstream(ctx, frames.NewVADUserStartedSpeakingFrame(params.StartSecs))
			p.lastActivity = now
		} else if state == vad.StateQuiet && p.prevState == vad.StateSpeaking {
			params := p.Analyzer.Params()
			_ = p.PushDownstream(ctx, frames.NewVADUserStoppedSpeakingFrame(params.StopSecs))
		} else if state == vad.StateSpeaking && p.SpeechActivityPeriod > 0 {
			if now.Sub(p.lastActivity) >= p.SpeechActivityPeriod {
				_ = p.PushDownstream(ctx, frames.NewUserSpeakingFrame())
				p.lastActivity = now
			}
		}

		p.prevState = state
		return nil
	}

	return nil
}
