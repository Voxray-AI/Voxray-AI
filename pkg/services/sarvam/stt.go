package sarvam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/services/httpclient"
)

// DefaultSarvamSTTModel is the default Sarvam STT model when none is specified.
// It matches the REST API default (saarika:v2.5).
const DefaultSarvamSTTModel = "saarika:v2.5"

// SarvamSTTService implements services.STTService (and STTStreamingService via TranscribeStream)
// using Sarvam AI's speech-to-text REST API.
//
// It uses:
//
//	POST https://api.sarvam.ai/speech-to-text (multipart/form-data)
//
// with fields:
//   - file: binary audio (WAV or raw PCM; format must match input_audio_codec)
//   - model: e.g. "saarika:v2.5" or "saaras:v3"
//   - input_audio_codec: "wav" when sending WAV bytes, "pcm_s16le" for raw PCM
//   - language_code: optional, e.g. "en-IN", "hi-IN"; empty means auto-detect
type SarvamSTTService struct {
	apiKey       string
	baseURL      string
	model        string
	languageCode string // optional; empty = auto-detect
	httpClient   *http.Client
}

// NewSTT creates a Sarvam STT service.
// If apiKey is empty, config.GetEnv("SARVAM_API_KEY", "") is used.
// If model is empty, DefaultSarvamSTTModel is used.
func NewSTT(apiKey, model string) *SarvamSTTService {
	return NewSTTWithLanguage(apiKey, model, "")
}

// NewSTTWithLanguage creates a Sarvam STT service with an optional language code.
func NewSTTWithLanguage(apiKey, model, languageCode string) *SarvamSTTService {
	if apiKey == "" {
		apiKey = config.GetEnv("SARVAM_API_KEY", "")
	}
	if model == "" {
		model = DefaultSarvamSTTModel
	}
	return &SarvamSTTService{
		apiKey:       apiKey,
		baseURL:      DefaultBaseURL,
		model:        model,
		languageCode: languageCode,
		httpClient:   httpclient.Client(60 * time.Second),
	}
}

// Transcribe sends audio to Sarvam's REST STT API and returns one TranscriptionFrame (final).
func (s *SarvamSTTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if len(audio) == 0 {
		return nil, nil
	}
	logger.Info("Sarvam STT: received audio from pipeline, %d bytes, sending to API", len(audio))
	// Detect WAV (RIFF header) vs raw PCM. Sarvam API requires correct format/codec.
	isWAV := len(audio) >= 12 && bytes.Equal(audio[0:4], []byte("RIFF")) && bytes.Equal(audio[8:12], []byte("WAVE"))
	fileName := "audio.pcm"
	codec := "pcm_s16le"
	if isWAV {
		fileName = "audio.wav"
		codec = "wav"
	}
	logger.Info("Sarvam STT request: %d bytes, %d Hz, %d ch, format=%s", len(audio), sampleRate, numChannels, codec)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// File part: filename and codec must match actual audio format
	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := fileWriter.Write(audio); err != nil {
		return nil, err
	}

	// Model part
	if err := writer.WriteField("model", s.model); err != nil {
		return nil, err
	}

	// input_audio_codec: wav for WAV files, pcm_s16le for raw PCM (per Sarvam API)
	if err := writer.WriteField("input_audio_codec", codec); err != nil {
		return nil, err
	}

	// language_code: optional; REST API supports e.g. "en-IN", "unknown" for auto-detect
	if s.languageCode != "" {
		_ = writer.WriteField("language_code", s.languageCode)
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/speech-to-text", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("api-subscription-key", s.apiKey)
	for k, v := range sdkHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

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
		return nil, fmt.Errorf("sarvam STT error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Transcript   string   `json:"transcript"`
		LanguageCode *string  `json:"language_code"`
		RequestID    string   `json:"request_id"`
		LanguageProb *float64 `json:"language_probability"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}

	lang := ""
	if out.LanguageCode != nil {
		lang = *out.LanguageCode
	}
	logger.Info("Sarvam STT response: transcript=%q language=%s request_id=%s", out.Transcript, lang, out.RequestID)

	// Even if transcript is empty, return a frame to keep behavior predictable.
	tf := frames.NewTranscriptionFrame(out.Transcript, "user", "", true)
	if out.LanguageCode != nil && *out.LanguageCode != "" {
		tf.Language = *out.LanguageCode
	}
	return []*frames.TranscriptionFrame{tf}, nil
}

// TranscribeStream uses Sarvam's WebSocket streaming STT API: it connects to the streaming
// endpoint, sends audio from audioCh (as base64), and pushes TranscriptionFrame(s) to outCh
// as transcript messages arrive. When audioCh closes, the buffered audio is sent and the
// connection is closed. For one-off transcription use Transcribe (REST) instead.
func (s *SarvamSTTService) TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
	s.runSTTStreaming(ctx, audioCh, sampleRate, numChannels, outCh)
}
