// Package voice provides processors that wire STT, LLM, and TTS into a pipeline.
package voice

import (
	"context"
	"sync"
	"time"

	"voila-go/pkg/audio"
	"voila-go/pkg/audio/turn"
	"voila-go/pkg/audio/vad"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
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

	// useAsync toggles whether end-of-turn detection is driven via AnalyzeEndOfTurnAsync.
	useAsync bool
	// pendingResult holds the in-flight async analysis result channel, if any.
	pendingResult <-chan turn.EndOfTurnResult

	// userTurnController manages high-level user turn/idle events and emits
	// UserStartedSpeakingFrame, UserStoppedSpeakingFrame, and UserIdleFrame.
	userTurnController *turn.UserTurnController

	firstAudioLog   sync.Once
	audioChunkCount uint64
	lastVADSpeech   bool // for optional VAD transition logging
}

// NewTurnProcessor returns a processor that buffers audio and forwards one segment per turn.
// When useAsync is true, end-of-turn detection is driven via Analyzer.AnalyzeEndOfTurnAsync;
// otherwise the synchronous AppendAudio return value is used.
func NewTurnProcessor(name string, v vad.Detector, a turn.Analyzer, sampleRate, channels int, useAsync bool) *TurnProcessor {
	return NewTurnProcessorWithUserTurn(name, v, a, sampleRate, channels, useAsync, 5.0, 0.0)
}

// NewTurnProcessorWithUserTurn is like NewTurnProcessor but allows callers to
// configure user turn stop and idle timeouts.
func NewTurnProcessorWithUserTurn(
	name string,
	v vad.Detector,
	a turn.Analyzer,
	sampleRate,
	channels int,
	useAsync bool,
	userTurnStopTimeout float64,
	userIdleTimeout float64,
) *TurnProcessor {
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
	p := &TurnProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		VAD:           v,
		Analyzer:      a,
		SampleRate:    sampleRate,
		Channels:      channels,
		useAsync:      useAsync,
	}

	// Wire a simple user turn controller using VAD-only strategies and
	// conservative timeouts (5s stop timeout, 0 idle timeout by default).
	startStrategy := &turn.VADUserTurnStartStrategy{}
	stopStrategy := &turn.SilenceUserTurnStopStrategy{}
	p.userTurnController = turn.NewUserTurnController(
		startStrategy,
		stopStrategy,
		userTurnStopTimeout,
		userIdleTimeout,
		func(ctx context.Context, f frames.Frame) error {
			return p.PushDownstream(ctx, f)
		},
	)
	return p
}

// ProcessFrame buffers AudioRawFrame, runs VAD and turn detection; on turn complete pushes audio downstream.
func (p *TurnProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Upstream {
		if vf, ok := f.(*frames.VADParamsUpdateFrame); ok {
			p.Analyzer.UpdateParams(turn.Params{StopSecs: vf.StopSecs})
			if vf.StartSecs != 0 {
				p.Analyzer.UpdateVADStartSecs(vf.StartSecs)
			}
			return nil
		}
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	// Apply VAD params update when received downstream (e.g. from pipeline config) or upstream (from IVR).
	if vf, ok := f.(*frames.VADParamsUpdateFrame); ok {
		p.Analyzer.UpdateParams(turn.Params{StopSecs: vf.StopSecs})
		if vf.StartSecs != 0 {
			p.Analyzer.UpdateVADStartSecs(vf.StartSecs)
		}
		return p.PushDownstream(ctx, f)
	}

	// Pass through non-audio frames; on cancel clear turn state
	switch f.(type) {
	case *frames.CancelFrame:
		p.buffer = nil
		p.Analyzer.Clear()
		// Drop any pending async analysis; any in-flight goroutine will observe ctx cancellation
		// at the pipeline level and exit independently.
		p.pendingResult = nil
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

	p.firstAudioLog.Do(func() {
		logger.Info("pipeline (turn): first audio chunk received from transport, %d bytes", len(chunk))
	})
	p.audioChunkCount++
	if p.audioChunkCount%25 == 0 {
		logger.Debug("pipeline (turn): audio chunks received so far: %d (buffering until turn complete)", p.audioChunkCount)
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
	if isSpeech != p.lastVADSpeech {
		logger.Debug("pipeline (turn): VAD speech=%v", isSpeech)
		p.lastVADSpeech = isSpeech
	}

	// Feed VAD event into user turn controller to drive high-level start/stop/idle.
	if p.userTurnController != nil {
		_ = p.userTurnController.ProcessVADUpdate(ctx, isSpeech)
	}

	p.buffer = append(p.buffer, chunk...)
	state := p.Analyzer.AppendAudio(chunk, isSpeech)

	// Synchronous mode: preserve existing behavior.
	if !p.useAsync {
		if state == turn.Complete {
			// Push accumulated audio as one AudioRawFrame for this turn
			audioCopy := make([]byte, len(p.buffer))
			copy(audioCopy, p.buffer)
			out := frames.NewAudioRawFrame(audioCopy, p.SampleRate, p.Channels, 0)
			p.buffer = nil
			p.Analyzer.Clear()
			logger.Info("pipeline (turn): turn complete, pushing %d bytes to STT", len(audioCopy))
			return p.PushDownstream(ctx, out)
		}
		return nil
	}

	// Async mode: allow synchronous Complete as a fast path.
	if state == turn.Complete {
		audioCopy := make([]byte, len(p.buffer))
		copy(audioCopy, p.buffer)
		out := frames.NewAudioRawFrame(audioCopy, p.SampleRate, p.Channels, 0)
		p.buffer = nil
		p.Analyzer.Clear()
		p.pendingResult = nil
		logger.Info("pipeline (turn): turn complete, pushing %d bytes to STT", len(audioCopy))
		return p.PushDownstream(ctx, out)
	}

	// Non-blocking check for a completed async result.
	if p.pendingResult != nil {
		select {
		case res, ok := <-p.pendingResult:
			if ok && res.Err == nil && res.State == turn.Complete {
				audioCopy := make([]byte, len(p.buffer))
				copy(audioCopy, p.buffer)
				out := frames.NewAudioRawFrame(audioCopy, p.SampleRate, p.Channels, 0)
				p.buffer = nil
				p.Analyzer.Clear()
				p.pendingResult = nil
				logger.Info("pipeline (turn): turn complete (async), pushing %d bytes to STT", len(audioCopy))
				return p.PushDownstream(ctx, out)
			}
			// On non-complete state, error, or closed channel, drop pending and continue.
			p.pendingResult = nil
		default:
			// No result available yet.
		}
	}

	// If analysis is active and there is no pending async call, start one.
	if p.pendingResult == nil && p.Analyzer.SpeechTriggered() {
		p.pendingResult = p.Analyzer.AnalyzeEndOfTurnAsync(ctx)
	}

	return nil
}
