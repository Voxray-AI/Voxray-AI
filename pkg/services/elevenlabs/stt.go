package elevenlabs

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/services/httpclient"
)

// pcmToWAV wraps raw 16-bit little-endian PCM in a minimal WAV header for API upload.
func pcmToWAV(pcm []byte, sampleRate, numChannels int) []byte {
	if numChannels <= 0 {
		numChannels = 1
	}
	dataLen := len(pcm)
	headerLen := 44
	total := headerLen + dataLen
	buf := make([]byte, total)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(total-8))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*numChannels*2))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(numChannels*2))
	binary.LittleEndian.PutUint16(buf[34:36], 16)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], pcm)
	return buf
}

const (
	elevenlabsAPIBase = "https://api.elevenlabs.io/v1"
	defaultSTTModel  = "scribe_v1"
)

// sttResponse is the JSON response from ElevenLabs speech-to-text convert endpoint.
type sttResponse struct {
	Text               string `json:"text"`
	LanguageCode       string `json:"language_code"`
	LanguageProbability float64 `json:"language_probability"`
	TranscriptionID    string `json:"transcription_id"`
}

// STTService implements services.STTService using ElevenLabs Speech-to-Text (Scribe).
type STTService struct {
	client  *http.Client
	apiKey  string
	modelID string
}

// NewSTT creates an ElevenLabs STT service.
// If apiKey is empty, config.GetEnv("ELEVENLABS_API_KEY", "") is used.
func NewSTT(apiKey, modelID string) *STTService {
	if apiKey == "" {
		apiKey = config.GetEnv("ELEVENLABS_API_KEY", "")
	}
	if modelID == "" {
		modelID = defaultSTTModel
	}
	return &STTService{
		client:  httpclient.Client(60 * time.Second),
		apiKey:  apiKey,
		modelID: modelID,
	}
}

// Transcribe sends audio to ElevenLabs Scribe and returns TranscriptionFrame(s).
// Raw 16-bit PCM is wrapped in a WAV header before upload.
func (s *STTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	wav := pcmToWAV(audio, sampleRate, numChannels)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	part, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("elevenlabs stt: create form file: %w", err)
	}
	if _, err := part.Write(wav); err != nil {
		return nil, fmt.Errorf("elevenlabs stt: write audio: %w", err)
	}
	if err := w.WriteField("model_id", s.modelID); err != nil {
		return nil, fmt.Errorf("elevenlabs stt: write model_id: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("elevenlabs stt: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, elevenlabsAPIBase+"/speech-to-text", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("xi-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs stt: request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs stt: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elevenlabs stt: %s: %s", resp.Status, string(data))
	}

	var out sttResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("elevenlabs stt: decode: %w", err)
	}
	tf := frames.NewTranscriptionFrame(out.Text, "user", "", true)
	if out.LanguageCode != "" {
		tf.Language = out.LanguageCode
	}
	return []*frames.TranscriptionFrame{tf}, nil
}
