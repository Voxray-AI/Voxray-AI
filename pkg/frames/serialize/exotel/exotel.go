// Package exotel provides Exotel Media Streams WebSocket protocol serializer.
package exotel

import (
	"encoding/base64"
	"encoding/json"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// Serializer implements serialize.Serializer and serialize.SerializerWithSetup for Exotel.
// Exotel uses PCM (not μ-law) in base64 for media events.
type Serializer struct {
	StreamSid   string
	CallSid     string
	SampleRate  int
	ExotelRate  int
}

// Params configures the Exotel serializer.
type Params struct {
	ExotelSampleRate int
	SampleRate       int
}

// NewSerializer returns an Exotel Media Streams serializer.
func NewSerializer(streamSid, callSid string, params *Params) *Serializer {
	s := &Serializer{
		StreamSid:  streamSid,
		CallSid:    callSid,
		ExotelRate: 8000,
	}
	if params != nil {
		if params.ExotelSampleRate > 0 {
			s.ExotelRate = params.ExotelSampleRate
		}
		s.SampleRate = params.SampleRate
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
	if inRate != s.ExotelRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.ExotelRate)
	} else {
		resampled = pcm
	}
	payload := base64.StdEncoding.EncodeToString(resampled)
	out := map[string]interface{}{
		"event":    "media",
		"streamSid": s.StreamSid,
		"media":   map[string]string{"payload": payload},
	}
	b, err := json.Marshal(out)
	return b, false, err
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
		// Exotel sends PCM (no μ-law)
		sr := s.SampleRate
		if sr == 0 {
			sr = 16000
		}
		if s.ExotelRate != sr {
			decoded = audio.Resample16MonoAlloc(decoded, s.ExotelRate, sr)
		}
		return frames.NewAudioRawFrame(decoded, sr, 1, 0), nil
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
