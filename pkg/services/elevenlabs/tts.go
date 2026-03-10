package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/services/httpclient"
)

const (
	defaultTTSModel   = "eleven_multilingual_v2"
	defaultOutputFmt  = "pcm_24000" // raw PCM 24kHz for pipeline compatibility
)

// TTSService implements services.TTSService using ElevenLabs Text-to-Speech.
type TTSService struct {
	client   *http.Client
	apiKey   string
	voiceID  string
	modelID  string
	outputFmt string
}

// NewTTS creates an ElevenLabs TTS service.
// If apiKey is empty, config.GetEnv("ELEVENLABS_API_KEY", "") is used.
// voiceID is required (use ElevenLabs voices list). modelID and outputFormat can be empty for defaults.
func NewTTS(apiKey, voiceID, modelID, outputFormat string) *TTSService {
	if apiKey == "" {
		apiKey = config.GetEnv("ELEVENLABS_API_KEY", "")
	}
	if modelID == "" {
		modelID = defaultTTSModel
	}
	if outputFormat == "" {
		outputFormat = defaultOutputFmt
	}
	return &TTSService{
		client:    httpclient.Client(60 * time.Second),
		apiKey:    apiKey,
		voiceID:   voiceID,
		modelID:   modelID,
		outputFmt: outputFormat,
	}
}

// ttsRequest is the JSON body for the TTS stream endpoint.
type ttsRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
}

// Speak requests TTS and returns one or more TTSAudioRawFrame. Uses pcm_24000 by default for raw PCM at 24kHz.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	payload, err := json.Marshal(ttsRequest{Text: text, ModelID: s.modelID})
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/text-to-speech/%s/stream?output_format=%s", elevenlabsAPIBase, s.voiceID, s.outputFmt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", s.apiKey)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs tts: request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs tts: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elevenlabs tts: %s: %s", resp.Status, string(data))
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

// SpeakStream runs TTS and sends the resulting TTSAudioRawFrame(s) to outCh.
func (s *TTSService) SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
	framesOut, err := s.Speak(ctx, text, sampleRate)
	if err != nil {
		return
	}
	for _, f := range framesOut {
		select {
		case <-ctx.Done():
			return
		case outCh <- f:
		}
	}
}
