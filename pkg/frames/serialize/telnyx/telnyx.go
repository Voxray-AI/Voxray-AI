// Package telnyx provides Telnyx WebSocket protocol serializer.
package telnyx

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// Serializer implements serialize.Serializer and serialize.SerializerWithSetup for Telnyx.
type Serializer struct {
	StreamID        string
	CallControlID   string
	APIKey          string
	SampleRate      int
	TelnyxRate      int
	InboundEncoding  string // "PCMU" or "PCMA"
	OutboundEncoding string // "PCMU" or "PCMA"
	AutoHangUp      bool
	hangUpOnce      sync.Once
}

// Params configures the Telnyx serializer.
type Params struct {
	TelnyxSampleRate int
	SampleRate       int
	InboundEncoding  string
	OutboundEncoding string
	AutoHangUp       bool
}

// NewSerializer returns a Telnyx WebSocket serializer.
func NewSerializer(streamID, outboundEncoding, inboundEncoding, callControlID, apiKey string, params *Params) *Serializer {
	s := &Serializer{
		StreamID:         streamID,
		CallControlID:    callControlID,
		APIKey:           apiKey,
		TelnyxRate:       8000,
		InboundEncoding:  "PCMU",
		OutboundEncoding: "PCMU",
		AutoHangUp:       true,
	}
	if params != nil {
		if params.TelnyxSampleRate > 0 {
			s.TelnyxRate = params.TelnyxSampleRate
		}
		s.SampleRate = params.SampleRate
		if params.InboundEncoding != "" {
			s.InboundEncoding = params.InboundEncoding
		}
		if params.OutboundEncoding != "" {
			s.OutboundEncoding = params.OutboundEncoding
		}
		s.AutoHangUp = params.AutoHangUp
	}
	if inboundEncoding != "" {
		s.InboundEncoding = inboundEncoding
	}
	if outboundEncoding != "" {
		s.OutboundEncoding = outboundEncoding
	}
	return s
}

// Setup implements serialize.SerializerWithSetup.
func (s *Serializer) Setup(start *frames.StartFrame) {
	if start != nil && s.SampleRate == 0 {
		s.SampleRate = start.AudioInSampleRate
	}
	if s.SampleRate == 0 {
		s.SampleRate = 16000
	}
}

// Serialize implements serialize.Serializer.
func (s *Serializer) Serialize(f frames.Frame) ([]byte, error) {
	data, _, err := s.serialize(f)
	return data, err
}

func (s *Serializer) serialize(f frames.Frame) ([]byte, bool, error) {
	if s.AutoHangUp {
		switch f.(type) {
		case *frames.EndFrame, *frames.CancelFrame:
			s.hangUpOnce.Do(s.hangUpCall)
			return nil, false, nil
		}
	}

	switch v := f.(type) {
	case *frames.InterruptionFrame:
		out := map[string]string{"event": "clear"}
		b, err := json.Marshal(out)
		return b, false, err
	case *frames.AudioRawFrame:
		return s.serializeAudio(v.Audio, v.SampleRate)
	case *frames.TTSAudioRawFrame:
		return s.serializeAudio(v.Audio, v.SampleRate)
	case *frames.OutputAudioRawFrame:
		return s.serializeAudio(v.Audio, v.SampleRate)
	case *frames.TransportMessageFrame:
		b, err := json.Marshal(v.Message)
		return b, false, err
	}
	return nil, false, nil
}

func (s *Serializer) serializeAudio(pcm []byte, sampleRate int) ([]byte, bool, error) {
	if len(pcm) == 0 {
		return nil, false, nil
	}
	inRate := sampleRate
	if inRate <= 0 {
		inRate = s.SampleRate
	}
	if s.SampleRate == 0 {
		s.SampleRate = 16000
	}
	var resampled []byte
	if inRate != s.TelnyxRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.TelnyxRate)
	} else {
		resampled = pcm
	}
	var encoded []byte
	switch s.InboundEncoding {
	case "PCMU":
		encoded = audio.EncodeULaw(resampled)
	case "PCMA":
		encoded = audio.EncodeALaw(resampled)
	default:
		encoded = audio.EncodeULaw(resampled)
	}
	if len(encoded) == 0 {
		return nil, false, nil
	}
	payload := base64.StdEncoding.EncodeToString(encoded)
	out := map[string]interface{}{
		"event": "media",
		"media": map[string]string{"payload": payload},
	}
	b, err := json.Marshal(out)
	return b, false, err
}

func (s *Serializer) hangUpCall() {
	if s.CallControlID == "" || s.APIKey == "" {
		return
	}
	u := "https://api.telnyx.com/v2/calls/" + s.CallControlID + "/actions/hangup"
	req, _ := http.NewRequest(http.MethodPost, u, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// Deserialize implements serialize.Serializer.
func (s *Serializer) Deserialize(data []byte) (frames.Frame, error) {
	var msg struct {
		Event string `json:"event"`
		Media struct {
			Payload string `json:"payload"`
		} `json:"media"`
		DTMF struct {
			Digit string `json:"digit"`
		} `json:"dtmf"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	switch msg.Event {
	case "media":
		if msg.Media.Payload == "" {
			return nil, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(msg.Media.Payload)
		if err != nil || len(decoded) == 0 {
			return nil, nil
		}
		var pcm []byte
		switch s.OutboundEncoding {
		case "PCMU":
			pcm = audio.DecodeULaw(decoded)
		case "PCMA":
			pcm = audio.DecodeALaw(decoded)
		default:
			pcm = audio.DecodeULaw(decoded)
		}
		if len(pcm) == 0 {
			return nil, nil
		}
		sr := s.SampleRate
		if sr == 0 {
			sr = 16000
		}
		if s.TelnyxRate != sr {
			pcm = audio.Resample16MonoAlloc(pcm, s.TelnyxRate, sr)
		}
		return frames.NewAudioRawFrame(pcm, sr, 1, 0), nil
	case "dtmf":
		digit := msg.DTMF.Digit
		if digit == "" {
			return nil, nil
		}
		kp, err := frames.ParseKeypadEntry(digit)
		if err != nil {
			return nil, nil
		}
		return frames.NewInputDTMFFrame(kp)
	}
	return nil, nil
}

var (
	_ serialize.Serializer           = (*Serializer)(nil)
	_ serialize.SerializerWithSetup = (*Serializer)(nil)
)
