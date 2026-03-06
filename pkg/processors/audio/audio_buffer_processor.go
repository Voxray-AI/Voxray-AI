package audio

import (
	"context"

	audiopkg "voxray-go/pkg/audio"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// AudioBufferProcessor buffers user and bot audio, resamples to a target rate,
// syncs buffers (pad with silence), and merges for mono (mix) or stereo (user=left, bot=right).
// Optional callbacks: OnAudioData (merged), OnTrackAudioData (separate tracks), and when
// EnableTurnAudio is true, OnUserTurnAudioData and OnBotTurnAudioData per turn.
type AudioBufferProcessor struct {
	*processors.BaseProcessor
	SampleRate      int  // 0 = use from StartFrame
	NumChannels     int  // 1 = mono mix, 2 = stereo interleave
	BufferSize      int  // bytes; 0 = no buffered callbacks, only turn callbacks
	EnableTurnAudio bool

	OnAudioData        func(merged []byte, sampleRate, numChannels int)
	OnTrackAudioData   func(userBuf, botBuf []byte, sampleRate, numChannels int)
	OnUserTurnAudioData func(buf []byte, sampleRate, numChannels int)
	OnBotTurnAudioData  func(buf []byte, sampleRate, numChannels int)

	sampleRate       int
	bufferSize1s     int
	recording        bool
	userBuffer       []byte
	botBuffer        []byte
	userTurnBuffer   []byte
	botTurnBuffer    []byte
	userSpeaking     bool
	botSpeaking      bool
}

// NewAudioBufferProcessor returns an AudioBufferProcessor with the given config.
func NewAudioBufferProcessor(name string, sampleRate, numChannels, bufferSize int, enableTurnAudio bool) *AudioBufferProcessor {
	if name == "" {
		name = "AudioBuffer"
	}
	if numChannels <= 0 {
		numChannels = 1
	}
	return &AudioBufferProcessor{
		BaseProcessor:     processors.NewBaseProcessor(name),
		SampleRate:        sampleRate,
		NumChannels:       numChannels,
		BufferSize:        bufferSize,
		EnableTurnAudio:   enableTurnAudio,
	}
}

// Cleanup ensures any buffered audio is flushed before the processor is torn down.
func (p *AudioBufferProcessor) Cleanup(ctx context.Context) error {
	p.StopRecording(ctx)
	if p.BaseProcessor != nil {
		return p.BaseProcessor.Cleanup(ctx)
	}
	return nil
}

// StartRecording enables buffering and resets buffers.
func (p *AudioBufferProcessor) StartRecording() {
	p.recording = true
	p.userBuffer = nil
	p.botBuffer = nil
	p.userTurnBuffer = nil
	p.botTurnBuffer = nil
}

// StopRecording flushes remaining audio via callbacks and stops recording.
func (p *AudioBufferProcessor) StopRecording(ctx context.Context) {
	if !p.recording {
		return
	}
	p.callAudioDataHandlers()
	p.recording = false
	p.userBuffer = nil
	p.botBuffer = nil
	p.userTurnBuffer = nil
	p.botTurnBuffer = nil
}

func (p *AudioBufferProcessor) syncBufferToPosition(buf *[]byte, target int) {
	if len(*buf) >= target {
		return
	}
	need := target - len(*buf)
	*buf = append(*buf, make([]byte, need)...)
}

func (p *AudioBufferProcessor) mergeBuffers() []byte {
	if p.NumChannels == 1 {
		return audiopkg.MixMono(p.userBuffer, p.botBuffer)
	}
	if p.NumChannels == 2 {
		return audiopkg.InterleaveStereo(p.userBuffer, p.botBuffer)
	}
	return nil
}

func (p *AudioBufferProcessor) alignTrackBuffers() {
	userLen := len(p.userBuffer)
	botLen := len(p.botBuffer)
	if userLen == botLen {
		return
	}
	target := userLen
	if botLen > target {
		target = botLen
	}
	p.syncBufferToPosition(&p.userBuffer, target)
	p.syncBufferToPosition(&p.botBuffer, target)
}

func (p *AudioBufferProcessor) callAudioDataHandlers() {
	if !p.recording || (len(p.userBuffer) == 0 && len(p.botBuffer) == 0) {
		return
	}
	p.alignTrackBuffers()
	merged := p.mergeBuffers()
	if len(merged) > 0 && p.OnAudioData != nil {
		p.OnAudioData(merged, p.sampleRate, p.NumChannels)
	}
	if p.OnTrackAudioData != nil {
		userCopy := make([]byte, len(p.userBuffer))
		botCopy := make([]byte, len(p.botBuffer))
		copy(userCopy, p.userBuffer)
		copy(botCopy, p.botBuffer)
		p.OnTrackAudioData(userCopy, botCopy, p.sampleRate, p.NumChannels)
	}
	p.userBuffer = nil
	p.botBuffer = nil
}

// ProcessFrame buffers and resamples user/bot audio, syncs buffers, and invokes callbacks.
func (p *AudioBufferProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Next() != nil {
			return p.Next().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch frame := f.(type) {
	case *frames.StartFrame:
		p.sampleRate = p.SampleRate
		if p.sampleRate <= 0 && frame.AudioOutSampleRate > 0 {
			p.sampleRate = frame.AudioOutSampleRate
		}
		if p.sampleRate <= 0 {
			p.sampleRate = 16000
		}
		p.bufferSize1s = p.sampleRate * 2
	case *frames.CancelFrame, *frames.EndFrame:
		p.StopRecording(ctx)
	}

	if p.Next() != nil {
		if err := p.Next().ProcessFrame(ctx, f, dir); err != nil {
			return err
		}
	}

	if !p.recording {
		return nil
	}

	switch frame := f.(type) {
	case *frames.AudioRawFrame:
		if len(frame.Audio) == 0 {
			return nil
		}
		sr := frame.SampleRate
		if sr <= 0 {
			sr = 16000
		}
		resampled := audiopkg.Resample16MonoAlloc(frame.Audio, sr, p.sampleRate)
		if len(resampled) == 0 {
			return nil
		}
		p.syncBufferToPosition(&p.botBuffer, len(p.userBuffer))
		p.userBuffer = append(p.userBuffer, resampled...)
		if p.EnableTurnAudio {
			p.userTurnBuffer = append(p.userTurnBuffer, resampled...)
			if !p.userSpeaking && len(p.userTurnBuffer) > p.bufferSize1s {
				discard := len(p.userTurnBuffer) - p.bufferSize1s
				p.userTurnBuffer = p.userTurnBuffer[discard:]
			}
		}
	case *frames.TTSAudioRawFrame:
		p.processOutputAudio(frame.Audio, frame.SampleRate)
	case *frames.OutputAudioRawFrame:
		p.processOutputAudio(frame.Audio, frame.SampleRate)
	case *frames.UserStartedSpeakingFrame:
		p.userSpeaking = true
	case *frames.UserStoppedSpeakingFrame:
		if p.EnableTurnAudio && p.OnUserTurnAudioData != nil && len(p.userTurnBuffer) > 0 {
			buf := make([]byte, len(p.userTurnBuffer))
			copy(buf, p.userTurnBuffer)
			p.OnUserTurnAudioData(buf, p.sampleRate, 1)
		}
		p.userSpeaking = false
		p.userTurnBuffer = nil
	case *frames.BotStartedSpeakingFrame:
		p.botSpeaking = true
	case *frames.BotStoppedSpeakingFrame:
		if p.EnableTurnAudio && p.OnBotTurnAudioData != nil && len(p.botTurnBuffer) > 0 {
			buf := make([]byte, len(p.botTurnBuffer))
			copy(buf, p.botTurnBuffer)
			p.OnBotTurnAudioData(buf, p.sampleRate, 1)
		}
		p.botSpeaking = false
		p.botTurnBuffer = nil
	}

	if p.BufferSize > 0 && (len(p.userBuffer) >= p.BufferSize || len(p.botBuffer) >= p.BufferSize) {
		p.callAudioDataHandlers()
	}

	return nil
}

func (p *AudioBufferProcessor) processOutputAudio(audio []byte, sampleRate int) {
	if len(audio) == 0 {
		return
	}
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	resampled := audiopkg.Resample16MonoAlloc(audio, sampleRate, p.sampleRate)
	if len(resampled) == 0 {
		return
	}
	p.syncBufferToPosition(&p.userBuffer, len(p.botBuffer))
	p.botBuffer = append(p.botBuffer, resampled...)
	if p.EnableTurnAudio && p.botSpeaking {
		p.botTurnBuffer = append(p.botTurnBuffer, resampled...)
	}
}
