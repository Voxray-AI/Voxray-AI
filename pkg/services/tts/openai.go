// Package tts provides TTS service implementations (OpenAI TTS).
package tts

import (
	"context"
	"io"

	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

// OpenAIService implements services.TTSService using OpenAI TTS (e.g. tts-1).
type OpenAIService struct {
	client *openai.Client
	model  string
}

// NewOpenAI creates an OpenAI TTS service.
func NewOpenAI(apiKey, model string) *OpenAIService {
	if apiKey == "" {
		apiKey = config.GetEnv("OPENAI_API_KEY", "")
	}
	if model == "" {
		model = string(openai.TTSModel1)
	}
	return &OpenAIService{client: openai.NewClient(apiKey), model: model}
}

// Speak requests TTS and returns one or more TTSAudioRawFrame.
func (s *OpenAIService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	req := openai.CreateSpeechRequest{
		Model: openai.SpeechModel(s.model),
		Input: text,
		Voice: openai.VoiceAlloy,
	}
	rc, err := s.client.CreateSpeech(ctx, req)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

// SpeakStream runs TTS and sends the resulting TTSAudioRawFrame(s) to outCh.
func (s *OpenAIService) SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
	frames, err := s.Speak(ctx, text, sampleRate)
	if err != nil {
		return
	}
	for _, f := range frames {
		select {
		case <-ctx.Done():
			return
		case outCh <- f:
		}
	}
}
