// Package stt provides STT service implementations (OpenAI Whisper, Groq Whisper).
package stt

import (
	"context"
	"os"
	"path/filepath"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/services/groq"

	openai "github.com/sashabaranov/go-openai"
)

// DefaultGroqWhisperModel is the default Groq Whisper model when none is specified.
const DefaultGroqWhisperModel = "whisper-large-v3"

// GroqService implements services.STTService using Groq's Whisper API.
// It provides high-performance speech-to-text conversion by leveraging Groq's infrastructure,
// while maintaining compatibility with the OpenAI transcription requested format.
type GroqService struct {
	client *openai.Client
	model  string
}

// NewGroq creates a Groq Whisper STT service with default model (whisper-large-v3).
// If apiKey is empty, config.GetEnv("GROQ_API_KEY", "") is used.
func NewGroq(apiKey string) *GroqService {
	return NewGroqWithModel(apiKey, DefaultGroqWhisperModel)
}

// NewGroqWithModel creates a Groq Whisper STT service with the given model.
// If apiKey is empty, config.GetEnv("GROQ_API_KEY", "") is used.
// If model is empty, DefaultGroqWhisperModel is used.
func NewGroqWithModel(apiKey, model string) *GroqService {
	if apiKey == "" {
		apiKey = config.GetEnv("GROQ_API_KEY", "")
	}
	if model == "" {
		model = DefaultGroqWhisperModel
	}
	return &GroqService{client: groq.NewClient(apiKey), model: model}
}

// Transcribe sends audio to Groq Whisper and returns one TranscriptionFrame (final).
func (s *GroqService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	tmp, err := os.CreateTemp("", "Voxray-audio-*.wav")
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
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}
	// If Abs fails, path is used as-is (CreateTemp often returns an absolute path).
	req := openai.AudioRequest{
		Model:    s.model,
		FilePath: path,
		Format:   openai.AudioResponseFormatVerboseJSON,
	}
	resp, err := s.client.CreateTranscription(ctx, req)
	if err != nil {
		return nil, err
	}
	// Log STT response to verify correct/expected format (Groq Whisper: text + optional language).
	textLen := len(resp.Text)
	language := resp.Language
	if language == "" {
		language = "(not set)"
	}
	logger.Info("STT response: model=%s audioBytes=%d textLen=%d language=%s\n", s.model, len(audio), textLen, language)
	if textLen == 0 {
		logger.Info("STT response has empty text (check audio length >= 0.01s, format wav/flac/mp3, or prompt)\n")
	} else {
		preview := resp.Text
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		logger.Info("STT text preview: %q\n", preview)
	}
	tf := frames.NewTranscriptionFrame(resp.Text, "user", "", true)
	if resp.Language != "" {
		tf.Language = resp.Language
	}
	return []*frames.TranscriptionFrame{tf}, nil
}

// TranscribeStream buffers audio from audioCh and sends final TranscriptionFrame(s) to outCh.
func (s *GroqService) TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
	var buf []byte
	for {
		select {
		case <-ctx.Done():
			if len(buf) > 0 && outCh != nil {
				frames, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
				for _, f := range frames {
					outCh <- f
				}
			}
			return
		case chunk, ok := <-audioCh:
			if !ok {
				if len(buf) > 0 && outCh != nil {
					frames, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
					for _, f := range frames {
						outCh <- f
					}
				}
				return
			}
			buf = append(buf, chunk...)
		}
	}
}
