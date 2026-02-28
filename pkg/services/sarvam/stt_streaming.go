// Package sarvam: WebSocket streaming STT per
// https://docs.sarvam.ai/api-reference-docs/api-guides-tutorials/speech-to-text/streaming-api

package sarvam

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
)

const (
	sttStreamingPath  = "/speech-to-text/ws"
	sttModeTranscribe  = "transcribe"
)

// transcribeRequest is the JSON sent to Sarvam WebSocket to send audio.
type transcribeRequest struct {
	Audio      string `json:"audio"`
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
}

// streamResponse is a generic message from Sarvam WebSocket (speech_start, speech_end, transcript).
type streamResponse struct {
	Type       string  `json:"type"`
	Text       string  `json:"text,omitempty"`
	Transcript string  `json:"transcript,omitempty"`
	Language   *string `json:"language_code,omitempty"`
}

// runSTTStreaming opens a WebSocket to Sarvam streaming STT. It buffers all audio from audioCh
// until the channel closes, then sends the buffered audio in one message and pushes
// TranscriptionFrame(s) to outCh as transcript messages arrive. Uses codec from first chunk (WAV vs PCM).
func (s *SarvamSTTService) runSTTStreaming(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
	codec := "pcm_s16le"
	encoding := "audio/pcm"
	var firstChunk []byte
	var allBuf []byte
	for c := range audioCh {
		allBuf = append(allBuf, c...)
		if firstChunk == nil && len(c) > 0 {
			firstChunk = c
		}
	}
	if len(allBuf) == 0 {
		return
	}
	if len(firstChunk) >= 12 && bytes.Equal(firstChunk[0:4], []byte("RIFF")) && bytes.Equal(firstChunk[8:12], []byte("WAVE")) {
		codec = "wav"
		encoding = "audio/wav"
	}

	wsURL, err := s.streamingURL(sampleRate, codec)
	if err != nil {
		logger.Error("Sarvam STT streaming: build URL: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam STT streaming: %v", err), false, "stt"):
			case <-ctx.Done():
			}
		}
		return
	}
	header := http.Header{}
	header.Set("Api-Subscription-Key", s.apiKey)
	for k, v := range sdkHeaders() {
		header.Set(k, v)
	}

	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		logger.Error("Sarvam STT streaming: dial %s: %v", wsURL, err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam STT streaming: %v", err), false, "stt"):
			case <-ctx.Done():
			}
		}
		return
	}
	defer conn.Close()

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Error("Sarvam STT streaming read: %v", err)
				}
				return
			}
			var msg streamResponse
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			text := msg.Text
			if text == "" {
				text = msg.Transcript
			}
			if text == "" {
				continue
			}
			tf := frames.NewTranscriptionFrame(text, "user", "", true)
			if msg.Language != nil && *msg.Language != "" {
				tf.Language = *msg.Language
			}
			if outCh != nil {
				select {
				case outCh <- tf:
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}
	}()

	payload := transcribeRequest{
		Audio:      base64.StdEncoding.EncodeToString(allBuf),
		Encoding:   encoding,
		SampleRate: sampleRate,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Sarvam STT streaming: marshal payload: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam STT streaming: %v", err), false, "stt"):
			case <-ctx.Done():
			}
		}
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, body); err != nil {
		logger.Error("Sarvam STT streaming write: %v", err)
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	close(done)
	wg.Wait()
}

// streamingURL builds the WebSocket URL with query parameters.
func (s *SarvamSTTService) streamingURL(sampleRate int, codec string) (string, error) {
	base := s.baseURL
	if strings.HasPrefix(base, "https://") {
		base = "wss://" + strings.TrimPrefix(base, "https://")
	} else if strings.HasPrefix(base, "http://") {
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	u, err := url.Parse(base + sttStreamingPath)
	if err != nil {
		return "", fmt.Errorf("parse STT streaming URL: %w", err)
	}
	q := u.Query()
	q.Set("model", s.model)
	q.Set("mode", sttModeTranscribe)
	if s.languageCode != "" {
		q.Set("language-code", s.languageCode)
	} else {
		q.Set("language-code", "en-IN")
	}
	q.Set("sample_rate", fmt.Sprintf("%d", sampleRate))
	q.Set("input_audio_codec", codec)
	q.Set("high_vad_sensitivity", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
