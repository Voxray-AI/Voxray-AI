package google

import (
	"context"
	"fmt"

	"voila-go/pkg/audio"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

// DefaultGoogleTTSVoice is the default voice (language-specific default when name is empty).
const DefaultGoogleTTSLanguage = "en-US"

// TTSService implements services.TTSService using Google Cloud Text-to-Speech (v1).
// Auth: GOOGLE_APPLICATION_CREDENTIALS or Application Default Credentials.
type TTSService struct {
	client   *texttospeech.Client
	language string // BCP-47 e.g. "en-US"
	voice    string // optional voice name e.g. "en-US-Standard-A"
}

// NewTTS creates a Google Cloud Text-to-Speech service.
// languageCode is BCP-47 (e.g. "en-US"); voiceName is optional (e.g. "en-US-Standard-A" or "" for default).
func NewTTS(ctx context.Context, languageCode, voiceName string) (*TTSService, error) {
	if languageCode == "" {
		languageCode = config.GetEnv("GOOGLE_TTS_LANGUAGE", DefaultGoogleTTSLanguage)
	}
	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("google TTS client: %w", err)
	}
	return &TTSService{
		client:   client,
		language: languageCode,
		voice:    voiceName,
	}, nil
}

// Close releases the TTS client.
func (s *TTSService) Close() error {
	return s.client.Close()
}

// Speak synthesizes text to speech and returns TTSAudioRawFrame(s).
// Uses LINEAR16 (WAV) from the API and decodes to raw PCM to match pipeline expectations.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if text == "" {
		return nil, nil
	}
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	logger.Info("Google TTS: request: text=%d chars, language=%s", len(text), s.language)

	voice := &texttospeechpb.VoiceSelectionParams{
		LanguageCode: s.language,
	}
	if s.voice != "" {
		voice.Name = s.voice
	}

	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: voice,
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16,
			SampleRateHertz: int32(sampleRate),
		},
	}

	resp, err := s.client.SynthesizeSpeech(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("google TTS SynthesizeSpeech: %w", err)
	}
	if len(resp.AudioContent) == 0 {
		return nil, fmt.Errorf("google TTS: no audio returned")
	}

	// LINEAR16 from Cloud TTS includes WAV header; decode to raw PCM.
	pcm, outRate, err := audio.DecodeWAVToPCM(resp.AudioContent)
	if err != nil {
		// If it's not WAV (e.g. raw), use as-is
		pcm = resp.AudioContent
		outRate = sampleRate
	}
	logger.Info("Google TTS: response: %d bytes PCM @ %d Hz", len(pcm), outRate)
	f := frames.NewTTSAudioRawFrame(pcm, outRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}
