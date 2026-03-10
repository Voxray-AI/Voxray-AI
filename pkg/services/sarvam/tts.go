// Package sarvam provides Sarvam AI TTS and STT service implementations.
package sarvam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/services/httpclient"
)

// DefaultSarvamTTSModel is the default Sarvam TTS model (bulbul v2).
const DefaultSarvamTTSModel = "bulbul:v2"

// DefaultSarvamTTSSpeaker is the default Sarvam TTS speaker.
const DefaultSarvamTTSSpeaker = "anushka"

// SarvamTTSService implements services.TTSService (and TTSStreamingService via SpeakStream)
// using Sarvam AI's text-to-speech HTTP API.
//
// It mirrors the behavior of the Python SarvamHttpTTSService at a high level:
// - POST https://api.sarvam.ai/text-to-speech with JSON payload
// - Audio is returned as base64-encoded WAV/PCM in "audios"[0]
// - We decode the base64 and strip WAV headers when present, returning raw PCM.
type SarvamTTSService struct {
	apiKey     string
	baseURL    string
	model      string
	voice      string
	httpClient *http.Client
}

// NewTTS creates a Sarvam TTS service.
// If apiKey is empty, config.GetEnv("SARVAM_API_KEY", "") is used.
// If model or voice is empty, sensible Sarvam defaults are used.
func NewTTS(apiKey, model, voice string) *SarvamTTSService {
	if apiKey == "" {
		apiKey = config.GetEnv("SARVAM_API_KEY", "")
	}
	if model == "" {
		model = DefaultSarvamTTSModel
	}
	if voice == "" {
		voice = DefaultSarvamTTSSpeaker
	}
	return &SarvamTTSService{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		model:      model,
		voice:      voice,
		httpClient: httpclient.Client(30 * time.Second),
	}
}

// defaultSampleRateForModel returns a reasonable sample rate for a Sarvam TTS model.
func defaultSampleRateForModel(model string) int {
	switch model {
	case "bulbul:v3", "bulbul:v3-beta":
		return 24000
	default:
		return 22050
	}
}

// Speak requests TTS from Sarvam, decodes base64 audio (WAV or PCM) and returns TTSAudioRawFrame(s).
func (s *SarvamTTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if text == "" {
		return nil, nil
	}
	if sampleRate <= 0 {
		sampleRate = defaultSampleRateForModel(s.model)
	}
	logger.Info("Sarvam TTS: request: text=%d chars, model=%s, voice=%s", len(text), s.model, s.voice)

	payload := map[string]any{
		"text":                 text,
		"target_language_code": "en-IN",
		"speaker":              s.voice,
		"speech_sample_rate":   sampleRate,
		"enable_preprocessing": true,
		"model":                s.model,
		"pace":                 1.0,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/text-to-speech", io.NopCloser(bytesReader(bodyBytes)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("api-subscription-key", s.apiKey)
	for k, v := range sdkHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if len(respBody) > 512 {
			respBody = respBody[:512]
		}
		return nil, fmt.Errorf("sarvam TTS error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Audios []string `json:"audios"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if len(out.Audios) == 0 {
		return nil, fmt.Errorf("sarvam TTS: no audio returned")
	}

	audioData, err := base64.StdEncoding.DecodeString(out.Audios[0])
	if err != nil {
		return nil, err
	}

	var pcm []byte
	outRate := sampleRate
	if len(audioData) >= 4 && string(audioData[0:4]) == "RIFF" {
		pcm, outRate, err = audio.DecodeWAVToPCM(audioData)
		if err != nil {
			return nil, err
		}
	} else {
		pcm = audioData
	}
	logger.Info("Sarvam TTS: response: received audio %d bytes", len(pcm))

	f := frames.NewTTSAudioRawFrame(pcm, outRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

// SpeakStream runs TTS using Sarvam's WebSocket streaming API and sends
// TTSAudioRawFrame(s) to outCh as audio chunks arrive.
func (s *SarvamTTSService) SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
	s.runTTSStreaming(ctx, text, sampleRate, outCh)
}

// bytesReader returns an io.Reader for the provided byte slice without extra allocation.
func bytesReader(b []byte) io.Reader {
	return &byteSliceReader{b: b}
}

type byteSliceReader struct {
	b []byte
	i int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

