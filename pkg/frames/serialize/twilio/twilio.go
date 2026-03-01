// Package twilio provides Twilio Media Streams WebSocket protocol serializer.
package twilio

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// Serializer implements serialize.Serializer and serialize.SerializerWithSetup for Twilio Media Streams.
type Serializer struct {
	StreamSid   string
	CallSid     string
	AccountSid  string
	AuthToken   string
	Region      string
	Edge        string
	SampleRate  int
	TwilioRate  int
	AutoHangUp  bool
	hangUpOnce  sync.Once
	hangUpDone  bool
}

// Params configures the Twilio serializer.
type Params struct {
	TwilioSampleRate int
	SampleRate       int
	AutoHangUp       bool
}

// NewSerializer returns a Twilio Media Streams serializer.
func NewSerializer(streamSid, callSid, accountSid, authToken, region, edge string, params *Params) *Serializer {
	s := &Serializer{
		StreamSid:  streamSid,
		CallSid:    callSid,
		AccountSid: accountSid,
		AuthToken:  authToken,
		Region:     region,
		Edge:       edge,
		TwilioRate: 8000,
		AutoHangUp: true,
	}
	if params != nil {
		if params.TwilioSampleRate > 0 {
			s.TwilioRate = params.TwilioSampleRate
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
			s.hangUpOnce.Do(func() {
				s.hangUpCall()
				s.hangUpDone = true
			})
			return nil, false, nil
		}
	}

	switch v := f.(type) {
	case *frames.InterruptionFrame:
		out := map[string]string{"event": "clear", "streamSid": s.StreamSid}
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
	if inRate != s.TwilioRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.TwilioRate)
	} else {
		resampled = pcm
	}
	ulawBytes := audio.EncodeULaw(resampled)
	if len(ulawBytes) == 0 {
		return nil, false, nil
	}
	payload := base64.StdEncoding.EncodeToString(ulawBytes)
	out := map[string]interface{}{
		"event":     "media",
		"streamSid": s.StreamSid,
		"media":     map[string]string{"payload": payload},
	}
	b, err := json.Marshal(out)
	return b, false, err
}

func (s *Serializer) hangUpCall() {
	if s.CallSid == "" || s.AccountSid == "" || s.AuthToken == "" {
		return
	}
	regionPrefix := s.Region
	if regionPrefix != "" {
		regionPrefix += "."
	}
	edgePrefix := s.Edge
	if edgePrefix != "" {
		edgePrefix += "."
	}
	baseURL := "https://api." + edgePrefix + regionPrefix + "twilio.com/2010-04-01/Accounts/" + s.AccountSid + "/Calls/" + s.CallSid + ".json"
	form := url.Values{}
	form.Set("Status", "completed")
	body := strings.NewReader(form.Encode())
	req, _ := http.NewRequest(http.MethodPost, baseURL, body)
	req.SetBasicAuth(s.AccountSid, s.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// Deserialize implements serialize.Serializer.
func (s *Serializer) Deserialize(data []byte) (frames.Frame, error) {
	var msg struct {
		Event string          `json:"event"`
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
		ulawBytes, err := base64.StdEncoding.DecodeString(msg.Media.Payload)
		if err != nil || len(ulawBytes) == 0 {
			return nil, nil
		}
		pcm := audio.DecodeULaw(ulawBytes)
		if len(pcm) == 0 {
			return nil, nil
		}
		sr := s.SampleRate
		if sr == 0 {
			sr = 16000
		}
		if s.TwilioRate != sr {
			pcm = audio.Resample16MonoAlloc(pcm, s.TwilioRate, sr)
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

// Ensure Serializer implements the interfaces.
var (
	_ serialize.Serializer           = (*Serializer)(nil)
	_ serialize.SerializerWithSetup = (*Serializer)(nil)
)
