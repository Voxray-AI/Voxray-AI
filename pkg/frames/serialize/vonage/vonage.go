// Package vonage provides Vonage Audio Connector WebSocket serializer.
package vonage

import (
	"encoding/json"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// Serializer implements serialize.Serializer, SerializerWithSetup, and SerializerWithMessageType for Vonage.
// Binary WebSocket messages = 16-bit PCM audio; text = JSON events.
type Serializer struct {
	SampleRate     int
	VonageRate     int
}

// Params configures the Vonage serializer.
type Params struct {
	VonageSampleRate int
	SampleRate       int
}

// NewSerializer returns a Vonage Audio Connector serializer.
func NewSerializer(params *Params) *Serializer {
	s := &Serializer{VonageRate: 16000}
	if params != nil {
		if params.VonageSampleRate > 0 {
			s.VonageRate = params.VonageSampleRate
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
	data, _, err := s.SerializeWithType(f)
	return data, err
}

// SerializeWithType implements serialize.SerializerWithMessageType (audio = binary, JSON = text).
func (s *Serializer) SerializeWithType(f frames.Frame) ([]byte, bool, error) {
	switch v := f.(type) {
	case *frames.InterruptionFrame:
		out := map[string]string{"action": "clear"}
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
	if inRate != s.VonageRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.VonageRate)
	} else {
		resampled = pcm
	}
	// Vonage expects raw 16-bit PCM (binary), so return as binary = true
	return resampled, true, nil
}

// Deserialize implements serialize.Serializer. data may be text (JSON) or binary (PCM).
func (s *Serializer) Deserialize(data []byte) (frames.Frame, error) {
	var msg struct {
		Event string `json:"event"`
		Digit string `json:"digit"`
		DTMF  struct {
			Digit string `json:"digit"`
		} `json:"dtmf"`
	}
	if err := json.Unmarshal(data, &msg); err == nil {
		// Valid JSON: handle as Vonage event
		switch msg.Event {
		case "websocket:connected", "websocket:cleared", "websocket:notify":
			return nil, nil
		case "websocket:dtmf":
			digit := msg.Digit
			if digit == "" {
				digit = msg.DTMF.Digit
			}
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
	// Not JSON: treat as binary PCM audio
	if len(data) == 0 {
		return nil, nil
	}
	pcm := data
	sr := s.SampleRate
	if sr == 0 {
		sr = 16000
	}
	if s.VonageRate != sr {
		pcm = audio.Resample16MonoAlloc(pcm, s.VonageRate, sr)
	}
	return frames.NewAudioRawFrame(pcm, sr, 1, 0), nil
}

var (
	_ serialize.Serializer                 = (*Serializer)(nil)
	_ serialize.SerializerWithSetup        = (*Serializer)(nil)
	_ serialize.SerializerWithMessageType = (*Serializer)(nil)
)
