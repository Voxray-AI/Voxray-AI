package google

import (
	"context"
	"fmt"

	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"

	speech "cloud.google.com/go/speech/apiv2"
	"cloud.google.com/go/speech/apiv2/speechpb"
)

// DefaultGoogleSTTModel is the default Speech-to-Text v2 model (latest_long for long-form).
const DefaultGoogleSTTModel = "latest_long"

// STTService implements services.STTService using Google Cloud Speech-to-Text V2 (non-streaming Recognize).
// Auth: GOOGLE_APPLICATION_CREDENTIALS or Application Default Credentials.
type STTService struct {
	client    *speech.Client
	project   string
	location  string
	model     string
	language  string // BCP-47 e.g. "en-US"; empty = auto-detect
}

// NewSTT creates a Google Cloud Speech-to-Text V2 STT service.
// project and location can be empty to use GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION.
// model can be empty for DefaultGoogleSTTModel; languageCode is e.g. "en-US" or "" for auto-detect.
func NewSTT(ctx context.Context, project, location, model, languageCode string) (*STTService, error) {
	if project == "" {
		project = config.GetEnv("GOOGLE_CLOUD_PROJECT", "")
	}
	if location == "" {
		location = config.GetEnv("GOOGLE_CLOUD_LOCATION", "us-central1")
	}
	if model == "" {
		model = DefaultGoogleSTTModel
	}
	client, err := speech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("google STT client: %w", err)
	}
	return &STTService{
		client:   client,
		project:  project,
		location: location,
		model:    model,
		language: languageCode,
	}, nil
}

// Close releases the Speech client. Call when the service is no longer needed.
func (s *STTService) Close() error {
	return s.client.Close()
}

// recognizerName returns the resource name for the default recognizer.
func (s *STTService) recognizerName() string {
	return fmt.Sprintf("projects/%s/locations/%s/recognizers/_", s.project, s.location)
}

// Transcribe sends audio to Speech-to-Text V2 Recognize and returns transcription frames.
// Audio must be raw 16-bit PCM (no WAV header); sampleRate and numChannels are used in the request.
func (s *STTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if len(audio) == 0 {
		return nil, nil
	}
	if s.project == "" {
		return nil, fmt.Errorf("google STT: project is required (set GOOGLE_CLOUD_PROJECT)")
	}
	logger.Info("Google STT: received audio from pipeline, %d bytes, sending to API", len(audio))

	config := &speechpb.RecognitionConfig{
		DecodingConfig: &speechpb.RecognitionConfig_ExplicitDecodingConfig{
			ExplicitDecodingConfig: &speechpb.ExplicitDecodingConfig{
				Encoding:          speechpb.ExplicitDecodingConfig_LINEAR16,
				SampleRateHertz:   int32(sampleRate),
				AudioChannelCount: int32(numChannels),
			},
		},
		Model: s.model,
	}
	if s.language != "" {
		config.LanguageCodes = []string{s.language}
	}

	req := &speechpb.RecognizeRequest{
		Recognizer: s.recognizerName(),
		Config:     config,
		AudioSource: &speechpb.RecognizeRequest_Content{
			Content: audio,
		},
	}

	resp, err := s.client.Recognize(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("google STT Recognize: %w", err)
	}

	var out []*frames.TranscriptionFrame
	for _, result := range resp.GetResults() {
		for _, alt := range result.GetAlternatives() {
			text := alt.GetTranscript()
			if text == "" {
				continue
			}
			lang := result.GetLanguageCode()
			tf := frames.NewTranscriptionFrame(text, "", "", true)
			tf.Language = lang
			out = append(out, tf)
			// First alternative per result is the best; we emit one frame per result
			break
		}
	}
	if len(out) == 0 && (resp.GetResults() == nil || len(resp.GetResults()) == 0) {
		// No speech detected - return single empty or placeholder if needed; pipeline often expects at least one frame
		return nil, nil
	}
	logger.Info("Google STT response: %d result(s)", len(out))
	return out, nil
}
