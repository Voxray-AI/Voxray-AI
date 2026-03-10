// Package realtime provides realtime session implementations (OpenAI Realtime API and shim).
// RealtimeSession (SendText, SendAudio, Events, Close) and RealtimeService (NewSession) align with
// OpenAI Realtime and WebSocket session abstractions: a long-lived bidirectional session over
// WebSocket with event-driven input/output. OpenAIRealtimeAPI uses the official OpenAI Realtime
// WebSocket API; use realtime.NewFromConfig(cfg, "openai") to construct it.
package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/services"
)

const (
	realtimeAPIHost = "api.openai.com"
	realtimePath    = "/v1/realtime"
	realtimeModel   = "gpt-4o-realtime-preview-2024-12-17"
)

// openAIRealtimeEvent is a generic JSON event from the Realtime API.
type openAIRealtimeEvent struct {
	Type string          `json:"type"`
	EventID string       `json:"event_id,omitempty"`
	Session *struct {
		Model string `json:"model,omitempty"`
	} `json:"session,omitempty"`
	// response.audio_transcript.done
	Transcript *string `json:"transcript,omitempty"`
	// response.audio.delta
	Delta *string `json:"delta,omitempty"`
	// item for conversation.item.created
	Item *struct {
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			Audio string `json:"audio,omitempty"`
		} `json:"content,omitempty"`
	} `json:"item,omitempty"`
}

// OpenAIRealtimeAPI implements services.RealtimeService using the official OpenAI Realtime WebSocket API.
// It corresponds to the Python openai_realtime service: single WebSocket, response.audio_transcript.done
// and response.audio.delta events mapped to RealtimeEvent (Text/Audio).
type OpenAIRealtimeAPI struct {
	apiKey string
	model  string
}

// NewFromConfig returns a RealtimeService for the given provider, or an error if unsupported.
// Provider should be one of services.SupportedRealtimeProviders (e.g. "openai").
// This lives in the realtime package to avoid an import cycle (services -> realtime -> services).
func NewFromConfig(cfg *config.Config, provider string) (services.RealtimeService, error) {
	apiKey := cfg.GetAPIKey("openai", "OPENAI_API_KEY")
	model := cfg.Model
	if model == "" {
		model = realtimeModel
	}
	switch provider {
	case "openai":
		return NewOpenAIRealtimeAPI(apiKey, model), nil
	default:
		return nil, fmt.Errorf("realtime not supported for provider %q", provider)
	}
}

// NewOpenAIRealtimeAPI creates a Realtime service that uses the OpenAI Realtime API (WebSocket).
func NewOpenAIRealtimeAPI(apiKey, model string) *OpenAIRealtimeAPI {
	if apiKey == "" {
		apiKey = config.GetEnv("OPENAI_API_KEY", "")
	}
	if model == "" {
		model = realtimeModel
	}
	return &OpenAIRealtimeAPI{apiKey: apiKey, model: model}
}

// NewSession opens a WebSocket to the Realtime API and returns a session.
func (r *OpenAIRealtimeAPI) NewSession(ctx context.Context, cfg services.RealtimeConfig) (services.RealtimeSession, error) {
	model := r.model
	if cfg.Model != "" {
		model = cfg.Model
	}
	url := fmt.Sprintf("wss://%s%s?model=%s", realtimeAPIHost, realtimePath, model)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+r.apiKey)
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		return nil, fmt.Errorf("realtime api: dial: %w", err)
	}
	s := &openAIRealtimeAPISession{
		conn:   conn,
		events: make(chan services.RealtimeEvent, 128),
		done:   make(chan struct{}),
	}
	go s.readLoop(ctx)
	return s, nil
}

type openAIRealtimeAPISession struct {
	conn   *websocket.Conn
	events chan services.RealtimeEvent
	done   chan struct{}
	once   sync.Once
}

func (s *openAIRealtimeAPISession) SendText(ctx context.Context, text string) error {
	payload := map[string]any{
		"type": "conversation.item.create",
		"item": map[string]any{
			"type":  "message",
			"role":  "user",
			"content": []map[string]any{
				{"type": "input_text", "text": text},
			},
		},
	}
	return s.send(payload)
}

func (s *openAIRealtimeAPISession) SendAudio(ctx context.Context, audio []byte, sampleRate, numChannels int) error {
	encoded := base64.StdEncoding.EncodeToString(audio)
	payload := map[string]any{
		"type":  "input_audio_buffer.append",
		"audio": encoded,
	}
	return s.send(payload)
}

func (s *openAIRealtimeAPISession) send(payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s.conn.WriteMessage(websocket.TextMessage, data)
	return nil
}

func (s *openAIRealtimeAPISession) Events() <-chan services.RealtimeEvent {
	return s.events
}

func (s *openAIRealtimeAPISession) Close(ctx context.Context) error {
	s.once.Do(func() {
		close(s.done)
		s.conn.Close()
		close(s.events)
	})
	return nil
}

func (s *openAIRealtimeAPISession) readLoop(ctx context.Context) {
	defer func() { _ = s.Close(ctx) }() // best-effort close on exit
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var ev openAIRealtimeEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "response.audio_transcript.done":
			if ev.Transcript != nil && *ev.Transcript != "" {
				tf := &frames.LLMTextFrame{}
				tf.TextFrame = frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: *ev.Transcript, AppendToContext: true}
				s.emit(services.RealtimeEvent{Text: tf, Frame: tf})
			}
		case "response.audio.delta":
			if ev.Delta != nil {
				decoded, err := base64.StdEncoding.DecodeString(*ev.Delta)
				if err == nil && len(decoded) > 0 {
					af := frames.NewTTSAudioRawFrame(decoded, 24000)
					s.emit(services.RealtimeEvent{Audio: af, Frame: af})
				}
			}
		case "response.done", "error":
			// session ended or error
		}
	}
}

func (s *openAIRealtimeAPISession) emit(ev services.RealtimeEvent) {
	select {
	case <-s.done:
	case s.events <- ev:
	}
}
