package realtime

import (
	"context"
	"fmt"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/services"
	openaillm "voila-go/pkg/services/openai"
	sttservice "voila-go/pkg/services/stt"
)

// OpenAIRealtime provides a minimal OpenAI-backed implementation of
// services.RealtimeService. It builds on the existing LLM and STT services
// rather than the dedicated OpenAI Realtime API, but exposes a compatible
// interface that can be upgraded later.
type OpenAIRealtime struct {
	llm *openaillm.Service
	stt *sttservice.OpenAIService
}

// NewOpenAIRealtime constructs an OpenAIRealtime provider using the given API
// key and model. If apiKey or model are empty, the defaults from NewService
// and NewOpenAI are used.
func NewOpenAIRealtime(apiKey, model string) *OpenAIRealtime {
	return &OpenAIRealtime{
		llm: openaillm.NewService(apiKey, model),
		stt: sttservice.NewOpenAI(apiKey),
	}
}

type openAISession struct {
	llm    *openaillm.Service
	stt    *sttservice.OpenAIService
	events chan services.RealtimeEvent
	done   chan struct{}
	once   sync.Once
}

// NewSession creates a new realtime session. The RealtimeConfig is currently
// used only for model overrides; provider/voice/tools are accepted for future
// compatibility.
func (r *OpenAIRealtime) NewSession(ctx context.Context, cfg services.RealtimeConfig) (services.RealtimeSession, error) {
	llm := r.llm
	if cfg.Model != "" {
		llm = openaillm.NewService("", cfg.Model)
	}
	s := &openAISession{
		llm:    llm,
		stt:    r.stt,
		events: make(chan services.RealtimeEvent, 128),
		done:   make(chan struct{}),
	}
	return s, nil
}

// SendText runs a chat completion against the underlying LLM and streams tokens
// as RealtimeEvent entries on the events channel.
func (s *openAISession) SendText(ctx context.Context, text string) error {
	msgs := []map[string]any{
		{"role": "user", "content": text},
	}
	if err := s.llm.Chat(ctx, msgs, func(tf *frames.LLMTextFrame) {
		ev := services.RealtimeEvent{
			Text:  tf,
			Frame: tf,
		}
		s.sendEvent(ctx, ev)
	}); err != nil {
		return fmt.Errorf("realtime send text: %w", err)
	}
	return nil
}

// SendAudio sends a single buffer of audio to the underlying STT service and
// emits the resulting TranscriptionFrame(s) as RealtimeEvent entries.
func (s *openAISession) SendAudio(ctx context.Context, audio []byte, sampleRate, numChannels int) error {
	framesOut, err := s.stt.Transcribe(ctx, audio, sampleRate, numChannels)
	if err != nil {
		return fmt.Errorf("realtime send audio: %w", err)
	}
	for _, f := range framesOut {
		ev := services.RealtimeEvent{
			Frame: f,
		}
		s.sendEvent(ctx, ev)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// Events returns the channel of RealtimeEvent values for this session.
func (s *openAISession) Events() <-chan services.RealtimeEvent {
	return s.events
}

// Close marks the realtime session as closed. Underlying HTTP clients are shared and kept alive.
func (s *openAISession) Close(ctx context.Context) error {
	s.once.Do(func() {
		close(s.done)
	})
	return nil
}

func (s *openAISession) sendEvent(ctx context.Context, ev services.RealtimeEvent) {
	select {
	case <-ctx.Done():
	case <-s.done:
	case s.events <- ev:
	}
}