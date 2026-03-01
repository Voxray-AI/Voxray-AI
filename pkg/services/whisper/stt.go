// Package whisper provides Whisper API-backed STT (OpenAI or self-hosted compatible) with configurable base URL.
package whisper

import (
	"context"
	"os"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
)

// Service implements services.STTService using an OpenAI-compatible transcription API.
// Base URL defaults to OpenAI when WHISPER_BASE_URL is empty; set it to use a self-hosted Whisper API.
type Service struct {
	client *openai.Client
}

// NewService creates a Whisper STT service.
// apiKey: use WHISPER_API_KEY or OPENAI_API_KEY when empty (via config/env).
// baseURL: when empty, OpenAI default is used (so behavior matches OpenAI Whisper).
func NewService(apiKey, baseURL string) *Service {
	if apiKey == "" {
		apiKey = config.GetEnv("WHISPER_API_KEY", "")
	}
	if apiKey == "" {
		apiKey = config.GetEnv("OPENAI_API_KEY", "")
	}
	if baseURL == "" {
		baseURL = config.GetEnv("WHISPER_BASE_URL", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &Service{client: openai.NewClientWithConfig(cfg)}
}

// Transcribe sends audio to the Whisper API and returns one TranscriptionFrame (final).
// Audio is written to a temp file because the client expects a file path.
func (s *Service) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	tmp, err := os.CreateTemp("", "Voila-whisper-*.mp3")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(audio); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	path, _ = filepath.Abs(path)
	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: path,
		Format:   openai.AudioResponseFormatVerboseJSON,
	}
	resp, err := s.client.CreateTranscription(ctx, req)
	if err != nil {
		return nil, err
	}
	tf := frames.NewTranscriptionFrame(resp.Text, "user", "", true)
	if resp.Language != "" {
		tf.Language = resp.Language
	}
	return []*frames.TranscriptionFrame{tf}, nil
}

// TranscribeStream buffers audio from audioCh and sends final TranscriptionFrame(s) to outCh.
// Whisper API is not truly streaming; this batches incoming audio and transcribes when context is done or buffer is flushed.
func (s *Service) TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
	var buf []byte
	for {
		select {
		case <-ctx.Done():
			if len(buf) > 0 && outCh != nil {
				result, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
				for _, f := range result {
					outCh <- f
				}
			}
			return
		case chunk, ok := <-audioCh:
			if !ok {
				if len(buf) > 0 && outCh != nil {
					result, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
					for _, f := range result {
						outCh <- f
					}
				}
				return
			}
			buf = append(buf, chunk...)
		}
	}
}
