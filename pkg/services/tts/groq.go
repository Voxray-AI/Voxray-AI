// Package tts provides TTS service implementations (OpenAI TTS, Groq TTS).
package tts

import (
	"context"
	"io"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/services/groq"

	openai "github.com/sashabaranov/go-openai"
)

// DefaultGroqTTSModel is the default Groq TTS model (Orpheus English).
const DefaultGroqTTSModel = "canopylabs/orpheus-v1-english"

// DefaultGroqVoice is the default Groq TTS voice.
const DefaultGroqVoice = "alloy"

// GroqService implements services.TTSService using Groq's TTS API (OpenAI-compatible).
// Groq returns WAV at 48 kHz; we decode to raw PCM for TTSAudioRawFrame.
type GroqService struct {
	client *openai.Client
	model  string
	voice  string
}

// NewGroq creates a Groq TTS service.
// If apiKey is empty, config.GetEnv("GROQ_API_KEY", "") is used.
// If model or voice is empty, DefaultGroqTTSModel and DefaultGroqVoice are used.
func NewGroq(apiKey, model, voice string) *GroqService {
	if apiKey == "" {
		apiKey = config.GetEnv("GROQ_API_KEY", "")
	}
	if model == "" {
		model = DefaultGroqTTSModel
	}
	if voice == "" {
		voice = DefaultGroqVoice
	}
	return &GroqService{
		client: groq.NewClient(apiKey),
		model:  model,
		voice:  voice,
	}
}

// Speak requests TTS from Groq (WAV response), decodes to PCM, and returns TTSAudioRawFrame(s).
func (s *GroqService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	req := openai.CreateSpeechRequest{
		Model:          openai.SpeechModel(s.model),
		Input:          text,
		Voice:          openai.SpeechVoice(s.voice),
		ResponseFormat: openai.SpeechResponseFormatWav,
	}
	rc, err := s.client.CreateSpeech(ctx, req)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	wavData, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	pcm, outRate, err := audio.DecodeWAVToPCM(wavData)
	if err != nil {
		return nil, err
	}
	// sampleRate is unused; Groq decode returns the actual rate in outRate.
	_ = sampleRate
	f := frames.NewTTSAudioRawFrame(pcm, outRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

// SpeakStream runs TTS and sends the resulting TTSAudioRawFrame(s) to outCh.
func (s *GroqService) SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
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
