// Package plivo provides Plivo Audio Streaming WebSocket protocol serializer.
package plivo

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// Serializer implements serialize.Serializer and serialize.SerializerWithSetup for Plivo.
type Serializer struct {
	StreamID   string
	CallID     string
	AuthID     string
	AuthToken  string
	SampleRate int
	PlivoRate  int
	AutoHangUp bool
	hangUpOnce sync.Once
}

// Params configures the Plivo serializer.
type Params struct {
	PlivoSampleRate int
	SampleRate      int
	AutoHangUp      bool
}

// NewSerializer returns a Plivo WebSocket serializer.
func NewSerializer(streamID, callID, authID, authToken string, params *Params) *Serializer {
	s := &Serializer{
		StreamID:  streamID,
		CallID:    callID,
		AuthID:    authID,
		AuthToken: authToken,
		PlivoRate: 8000,
		AutoHangUp: true,
	}
	if params != nil {
		if params.PlivoSampleRate > 0 {
			s.PlivoRate = params.PlivoSampleRate
		}
		s.SampleRate = params.SampleRate
		s.AutoHangUp = params.AutoHangUp
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
		out := map[string]string{"event": "clearAudio", "streamId": s.StreamID}
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
	if inRate != s.PlivoRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.PlivoRate)
	} else {
		resampled = pcm
	}
	ulawBytes := audio.EncodeULaw(resampled)
	if len(ulawBytes) == 0 {
		return nil, false, nil
	}
	payload := base64.StdEncoding.EncodeToString(ulawBytes)
	out := map[string]interface{}{
		"event":    "playAudio",
		"streamId": s.StreamID,
		"media": map[string]interface{}{
			"contentType": "audio/x-mulaw",
			"sampleRate":  s.PlivoRate,
			"payload":     payload,
		},
	}
	b, err := json.Marshal(out)
	return b, false, err
}

func (s *Serializer) hangUpCall() {
	if s.CallID == "" || s.AuthID == "" || s.AuthToken == "" {
		return
	}
	u := "https://api.plivo.com/v1/Account/" + s.AuthID + "/Call/" + s.CallID + "/"
	req, _ := http.NewRequest(http.MethodDelete, u, nil)
	req.SetBasicAuth(s.AuthID, s.AuthToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// Deserialize implements serialize.Serializer.
func (s *Serializer) Deserialize(data []byte) (frames.Frame, error) {
	var msg struct {
		Event  string `json:"event"`
		Media  struct { Payload string `json:"payload"` } `json:"media"`
		DTMF   struct { Digit string `json:"digit"` } `json:"dtmf"`
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
		pcm := audio.DecodeULaw(decoded)
		if len(pcm) == 0 {
			return nil, nil
		}
		sr := s.SampleRate
		if sr == 0 {
			sr = 16000
		}
		if s.PlivoRate != sr {
			pcm = audio.Resample16MonoAlloc(pcm, s.PlivoRate, sr)
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
