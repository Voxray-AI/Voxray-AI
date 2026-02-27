// Package services defines interfaces and implementations for LLM, STT, and TTS.
package services

import (
	"context"

	"voila-go/pkg/frames"
)

// LLMService provides chat completion; may stream text frames.
type LLMService interface {
	// Chat runs a completion; for each streamed token/chunk, onToken is called with a text frame.
	Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error
}

// STTService transcribes audio to text (transcription frames).
type STTService interface {
	Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error)
}

// STTStreamingService optionally supports streaming transcription (interim + final frames).
type STTStreamingService interface {
	STTService
	// TranscribeStream sends transcription frames (interim and final) to outCh as audio is received on audioCh.
	TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame)
}

// TTSService converts text to speech (audio frames).
type TTSService interface {
	Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error)
}

// TTSStreamingService optionally supports streaming TTS (incremental audio to outCh).
type TTSStreamingService interface {
	TTSService
	// SpeakStream streams TTS audio frames to outCh as they are produced.
	SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame)
}

// RealtimeEvent represents a high-level event emitted by a realtime session.
// It can carry LLM text, TTS audio, or generic frames for extensibility.
type RealtimeEvent struct {
	Text  *frames.LLMTextFrame
	Audio *frames.TTSAudioRawFrame
	Frame frames.Frame
}

// RealtimeConfig configures a realtime session for a given provider/model.
type RealtimeConfig struct {
	Provider string           // e.g. "openai"
	Model    string           // e.g. "gpt-4o-realtime" or regular chat model
	Voice    string           // TTS voice, if applicable
	Tools    []map[string]any // optional function calling tools
}

// RealtimeSession is a bidirectional, long-lived conversation with an AI service.
type RealtimeSession interface {
	// SendText sends text input into the session (e.g. user message).
	SendText(ctx context.Context, text string) error
	// SendAudio sends raw audio input into the session (e.g. microphone audio).
	SendAudio(ctx context.Context, audio []byte, sampleRate, numChannels int) error
	// Events returns a channel of high-level events from the session.
	Events() <-chan RealtimeEvent
	// Close terminates the session and closes the events channel.
	Close(ctx context.Context) error
}

// RealtimeService creates realtime sessions.
type RealtimeService interface {
	NewSession(ctx context.Context, cfg RealtimeConfig) (RealtimeSession, error)
}

