// Package genesys provides Genesys AudioHook WebSocket protocol serializer.
// Protocol: text = JSON (open/opened/close/closed/ping/pong/update/event), binary = PCMU at 8kHz.
package genesys

import (
	"encoding/json"
	"sync/atomic"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

const protocolVersion = "2"

// Serializer implements serialize.Serializer, SerializerWithSetup, and SerializerWithMessageType for Genesys AudioHook.
type Serializer struct {
	SampleRate  int
	GenesysRate int
	MediaFormat string // "PCMU" or "L16"
	clientSeq   atomic.Int64
	serverSeq   atomic.Int64
	sessionID   string
}

// Params configures the Genesys serializer.
type Params struct {
	GenesysSampleRate int
	SampleRate        int
	MediaFormat       string
}

// NewSerializer returns a Genesys AudioHook serializer.
func NewSerializer(params *Params) *Serializer {
	s := &Serializer{GenesysRate: 8000, MediaFormat: "PCMU"}
	if params != nil {
		if params.GenesysSampleRate > 0 {
			s.GenesysRate = params.GenesysSampleRate
		}
		s.SampleRate = params.SampleRate
		if params.MediaFormat != "" {
			s.MediaFormat = params.MediaFormat
		}
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

// SerializeWithType implements serialize.SerializerWithMessageType (audio = binary, control = text).
func (s *Serializer) SerializeWithType(f frames.Frame) ([]byte, bool, error) {
	switch v := f.(type) {
	case *frames.InterruptionFrame:
		// Genesys: send update/event to clear or similar; minimal is to send a no-op or skip
		return nil, false, nil
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
	if inRate != s.GenesysRate {
		resampled = audio.Resample16MonoAlloc(pcm, inRate, s.GenesysRate)
	} else {
		resampled = pcm
	}
	var out []byte
	if s.MediaFormat == "PCMU" {
		out = audio.EncodeULaw(resampled)
	} else {
		out = resampled
	}
	if len(out) == 0 {
		return nil, false, nil
	}
	// Genesys uses binary WebSocket frames for audio
	return out, true, nil
}

// Deserialize implements serialize.Serializer. Binary = PCMU audio; text = JSON (open/opened/close/closed/ping/pong/event).
func (s *Serializer) Deserialize(data []byte) (frames.Frame, error) {
	// Try JSON first
	var msg struct {
		Type       string `json:"type"`
		Seq        int64  `json:"seq"`
		ClientSeq  int64  `json:"clientseq"`
		ID         string `json:"id"`
		Parameters struct {
			Media  string `json:"media"`
			Format string `json:"format"`
			Event  struct {
				Type string `json:"type"`
				DTMF struct {
					Digit string `json:"digit"`
				} `json:"dtmf"`
			} `json:"event"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(data, &msg); err == nil {
		s.clientSeq.Store(msg.Seq)
		if msg.ID != "" {
			s.sessionID = msg.ID
		}
		switch msg.Type {
		case "open", "opened", "close", "closed", "ping", "pong", "update":
			return nil, nil
		case "event":
			if msg.Parameters.Event.Type == "dtmf" {
				digit := msg.Parameters.Event.DTMF.Digit
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
		return nil, nil
	}
	// Binary: PCMU audio
	if len(data) == 0 {
		return nil, nil
	}
	var pcm []byte
	if s.MediaFormat == "PCMU" {
		pcm = audio.DecodeULaw(data)
	} else {
		pcm = data
	}
	if len(pcm) == 0 {
		return nil, nil
	}
	sr := s.SampleRate
	if sr == 0 {
		sr = 16000
	}
	if s.GenesysRate != sr {
		pcm = audio.Resample16MonoAlloc(pcm, s.GenesysRate, sr)
	}
	return frames.NewAudioRawFrame(pcm, sr, 1, 0), nil
}

// CreateOpenedResponse builds the "opened" JSON response for the client (caller sends via transport).
func (s *Serializer) CreateOpenedResponse(startPaused bool, supportedLanguages []string, selectedLanguage string) ([]byte, error) {
	seq := s.serverSeq.Add(1)
	params := map[string]interface{}{
		"startPaused": startPaused,
		"media": []map[string]interface{}{
			{"type": "audio", "format": s.MediaFormat, "channels": []string{"external"}, "rate": s.GenesysRate},
		},
	}
	if len(supportedLanguages) > 0 {
		params["supportedLanguages"] = supportedLanguages
	}
	if selectedLanguage != "" {
		params["selectedLanguage"] = selectedLanguage
	}
	msg := map[string]interface{}{
		"version":    protocolVersion,
		"type":       "opened",
		"seq":        seq,
		"clientseq":  s.clientSeq.Load(),
		"id":         s.sessionID,
		"parameters": params,
	}
	return json.Marshal(msg)
}

// CreatePongResponse builds the "pong" response for ping.
func (s *Serializer) CreatePongResponse() ([]byte, error) {
	seq := s.serverSeq.Add(1)
	msg := map[string]interface{}{
		"version":   protocolVersion,
		"type":      "pong",
		"seq":       seq,
		"clientseq": s.clientSeq.Load(),
		"id":        s.sessionID,
	}
	return json.Marshal(msg)
}

var (
	_ serialize.Serializer                 = (*Serializer)(nil)
	_ serialize.SerializerWithSetup        = (*Serializer)(nil)
	_ serialize.SerializerWithMessageType = (*Serializer)(nil)
)
