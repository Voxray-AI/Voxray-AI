// Package mock provides mock STT, LLM, and TTS services for testing and stress testing
// without calling real APIs. All mocks are configurable (response text, audio length, optional latency).
package mock

import (
	"context"
	"sync"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/services"
)

// Ensure mocks implement interfaces.
var (
	_ services.STTService = (*STT)(nil)
	_ services.LLMService = (*LLM)(nil)
	_ services.TTSService = (*TTS)(nil)
)

// STT is a mock STT service that returns configurable transcription(s).
type STT struct {
	// Transcript is the text returned for every Transcribe call.
	Transcript string
	// Transcripts is used for round-robin when set (non-empty); overrides Transcript.
	Transcripts []string
	// Latency is the optional delay before returning (for latency simulation).
	Latency time.Duration
	mu      sync.Mutex
	idx     int
}

// NewSTT returns a mock STT with default transcript "hello".
func NewSTT() *STT {
	return &STT{Transcript: "hello"}
}

// NewSTTWithTranscript returns a mock STT that always returns the given transcript.
func NewSTTWithTranscript(transcript string) *STT {
	return &STT{Transcript: transcript}
}

// Transcribe returns one or more TranscriptionFrame with the configured text.
func (m *STT) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if m.Latency > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.Latency):
		}
	}
	text := m.Transcript
	if len(m.Transcripts) > 0 {
		m.mu.Lock()
		text = m.Transcripts[m.idx%len(m.Transcripts)]
		m.idx++
		m.mu.Unlock()
	}
	if text == "" {
		text = "hello"
	}
	return []*frames.TranscriptionFrame{
		frames.NewTranscriptionFrame(text, "", "", true),
	}, nil
}

// LLM is a mock LLM service that streams a configurable response.
type LLM struct {
	// Response is the text streamed via onToken (one rune per call).
	Response string
	// Latency is the optional delay before starting (for latency simulation).
	Latency time.Duration
}

// NewLLM returns a mock LLM with default response "hi there".
func NewLLM() *LLM {
	return &LLM{Response: "hi there"}
}

// NewLLMWithResponse returns a mock LLM that streams the given response.
func NewLLMWithResponse(response string) *LLM {
	return &LLM{Response: response}
}

// Chat streams the configured response via onToken, one rune at a time.
func (m *LLM) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	if m.Latency > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.Latency):
		}
	}
	resp := m.Response
	if resp == "" {
		resp = "hi there"
	}
	for _, c := range resp {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		onToken(&frames.LLMTextFrame{
			TextFrame: frames.TextFrame{
				DataFrame:       frames.DataFrame{Base: frames.NewBase()},
				Text:           string(c),
				AppendToContext: true,
			},
		})
	}
	return nil
}

// TTS is a mock TTS service that returns configurable audio (e.g. silence).
type TTS struct {
	// AudioBytes is the number of bytes of audio to return per Speak call (default 2400 = 50ms at 24kHz mono 16-bit).
	AudioBytes int
	// SampleRate is the sample rate of the returned audio (default 24000).
	SampleRate int
	// Latency is the optional delay before returning (for latency simulation).
	Latency time.Duration
}

// NewTTS returns a mock TTS that returns a short silence buffer.
func NewTTS() *TTS {
	return &TTS{
		AudioBytes: 2400, // 50ms at 24kHz mono 16-bit
		SampleRate: 24000,
	}
}

// Speak returns one or more TTSAudioRawFrame with the configured payload.
func (m *TTS) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if m.Latency > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.Latency):
		}
	}
	n := m.AudioBytes
	if n <= 0 {
		n = 2400
	}
	sr := m.SampleRate
	if sr <= 0 {
		sr = 24000
	}
	if sampleRate > 0 {
		sr = sampleRate
	}
	audio := make([]byte, n)
	return []*frames.TTSAudioRawFrame{
		frames.NewTTSAudioRawFrame(audio, sr),
	}, nil
}
