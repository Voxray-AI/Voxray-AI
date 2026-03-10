// Package stt provides STT service implementations (OpenAI Whisper).
package stt

import (
	"context"
	"os"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

// OpenAIService implements services.STTService using OpenAI Whisper.
type OpenAIService struct {
	client *openai.Client
}

// NewOpenAI creates an OpenAI Whisper STT service.
func NewOpenAI(apiKey string) *OpenAIService {
	if apiKey == "" {
		apiKey = config.GetEnv("OPENAI_API_KEY", "")
	}
	return &OpenAIService{client: openai.NewClient(apiKey)}
}

// Transcribe sends audio to Whisper and returns one TranscriptionFrame (final).
// Audio is written to a temp file because the client expects a file path.
func (s *OpenAIService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	tmp, err := os.CreateTemp("", "Voxray-audio-*.mp3")
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
// OpenAI Whisper is not truly streaming; this batches incoming audio and transcribes when context is done or buffer is flushed.
func (s *OpenAIService) TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
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
			// Optional: flush on silence or size threshold; here we only flush on close/done
		}
	}
}
