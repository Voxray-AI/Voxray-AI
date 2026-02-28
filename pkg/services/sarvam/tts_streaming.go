// Package sarvam: WebSocket streaming TTS per
// https://docs.sarvam.ai/api-reference-docs/api-guides-tutorials/text-to-speech/streaming-api

package sarvam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
)

const ttsStreamingPath = "/text-to-speech/ws"

// ttsConfigMessage is sent first to configure the streaming TTS session.
type ttsConfigMessage struct {
	Type string `json:"type"`
	Data struct {
		Speaker            string  `json:"speaker"`
		TargetLanguageCode string  `json:"target_language_code"`
		Pace               float64 `json:"pace,omitempty"`
		MinBufferSize      int     `json:"min_buffer_size,omitempty"`
		OutputAudioCodec   string  `json:"output_audio_codec,omitempty"`
		SpeechSampleRate   string  `json:"speech_sample_rate,omitempty"`
	} `json:"data"`
}

// ttsTextMessage sends text to be synthesized.
type ttsTextMessage struct {
	Type string `json:"type"`
	Data struct {
		Text string `json:"text"`
	} `json:"data"`
}

// ttsFlushMessage forces the buffer to be processed.
type ttsFlushMessage struct {
	Type string `json:"type"`
}

// ttsStreamResponse is a server message (audio, event, or error).
type ttsStreamResponse struct {
	Type string `json:"type"`
	Data *struct {
		Audio       string `json:"audio"`
		ContentType string `json:"content_type"`
		EventType   string `json:"event_type"`
		Message     string `json:"message"`
	} `json:"data,omitempty"`
}

// runTTSStreaming opens a WebSocket to Sarvam TTS streaming, sends config then text and flush,
// and pushes decoded TTSAudioRawFrame(s) to outCh as audio messages arrive.
func (s *SarvamTTSService) runTTSStreaming(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
	if sampleRate <= 0 {
		sampleRate = defaultSampleRateForModel(s.model)
	}

	wsURL, err := s.ttsStreamingURL()
	if err != nil {
		logger.Error("Sarvam TTS streaming: build URL: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam TTS streaming: %v", err), false, "tts"):
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
		logger.Error("Sarvam TTS streaming: dial %s: %v", wsURL, err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam TTS streaming: %v", err), false, "tts"):
			case <-ctx.Done():
			}
		}
		return
	}
	defer conn.Close()

	// 1. Send config (required first)
	configMsg := ttsConfigMessage{Type: "config"}
	configMsg.Data.Speaker = s.voice
	configMsg.Data.TargetLanguageCode = "en-IN"
	configMsg.Data.Pace = 1.0
	configMsg.Data.MinBufferSize = 50
	configMsg.Data.OutputAudioCodec = "wav"
	configMsg.Data.SpeechSampleRate = fmt.Sprintf("%d", sampleRate)
	configBody, err := json.Marshal(configMsg)
	if err != nil {
		logger.Error("Sarvam TTS streaming: marshal config: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam TTS streaming: %v", err), false, "tts"):
			case <-ctx.Done():
			}
		}
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, configBody); err != nil {
		logger.Error("Sarvam TTS streaming config write: %v", err)
		return
	}

	// 2. Send text
	textMsg := ttsTextMessage{Type: "text"}
	textMsg.Data.Text = text
	textBody, err := json.Marshal(textMsg)
	if err != nil {
		logger.Error("Sarvam TTS streaming: marshal text: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam TTS streaming: %v", err), false, "tts"):
			case <-ctx.Done():
			}
		}
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, textBody); err != nil {
		logger.Error("Sarvam TTS streaming text write: %v", err)
		return
	}

	// 3. Flush to process
	flushBody, err := json.Marshal(ttsFlushMessage{Type: "flush"})
	if err != nil {
		logger.Error("Sarvam TTS streaming: marshal flush: %v", err)
		if outCh != nil {
			select {
			case outCh <- frames.NewErrorFrame(fmt.Sprintf("sarvam TTS streaming: %v", err), false, "tts"):
			case <-ctx.Done():
			}
		}
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, flushBody); err != nil {
		logger.Error("Sarvam TTS streaming flush write: %v", err)
		return
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Error("Sarvam TTS streaming read: %v", err)
				}
				return
			}
			var msg ttsStreamResponse
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "error":
				if msg.Data != nil && outCh != nil {
					select {
					case outCh <- frames.NewErrorFrame(msg.Data.Message, false, "tts"):
					case <-ctx.Done():
					case <-done:
					}
				}
				return
			case "audio":
				if msg.Data == nil || msg.Data.Audio == "" || outCh == nil {
					continue
				}
				audioData, err := base64.StdEncoding.DecodeString(msg.Data.Audio)
				if err != nil {
					continue
				}
				var pcm []byte
				outRate := sampleRate
				if len(audioData) >= 4 && string(audioData[0:4]) == "RIFF" {
					pcm, outRate, err = audio.DecodeWAVToPCM(audioData)
					if err != nil {
						continue
					}
				} else {
					pcm = audioData
				}
				if len(pcm) == 0 {
					continue
				}
				f := frames.NewTTSAudioRawFrame(pcm, outRate)
				select {
				case outCh <- f:
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			case "event":
				if msg.Data != nil && msg.Data.EventType == "final" {
					return
				}
			}
		}
	}()

	wg.Wait()
	close(done)
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

func (s *SarvamTTSService) ttsStreamingURL() (string, error) {
	base := s.baseURL
	if strings.HasPrefix(base, "https://") {
		base = "wss://" + strings.TrimPrefix(base, "https://")
	} else if strings.HasPrefix(base, "http://") {
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	u, err := url.Parse(base + ttsStreamingPath)
	if err != nil {
		return "", fmt.Errorf("parse TTS streaming URL: %w", err)
	}
	q := u.Query()
	q.Set("model", s.model)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
