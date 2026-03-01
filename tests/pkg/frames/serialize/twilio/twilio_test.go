package twilio_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize/twilio"
)

func TestTwilioSerializer_SerializeDeserialize_Media(t *testing.T) {
	s := twilio.NewSerializer("stream-123", "", "", "", "", "", nil)
	s.SampleRate = 16000
	s.TwilioRate = 8000

	// Small PCM chunk (8 samples = 16 bytes at 16kHz)
	pcm := make([]byte, 16)
	for i := range pcm {
		pcm[i] = byte(i)
	}
	ar := frames.NewAudioRawFrame(pcm, 16000, 1, 0)

	data, err := s.Serialize(ar)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Serialize returned empty")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if m["event"] != "media" {
		t.Fatalf("event = %v, want media", m["event"])
	}

	f, err := s.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	out, ok := f.(*frames.AudioRawFrame)
	if !ok {
		t.Fatalf("expected *frames.AudioRawFrame, got %T", f)
	}
	if out.SampleRate != 16000 {
		t.Fatalf("SampleRate = %d, want 16000", out.SampleRate)
	}
	// After 16k->8k resample, ulaw, 8k->16k we may get different length; just check we got something
	if len(out.Audio) == 0 {
		t.Fatal("decoded audio is empty")
	}
}

func TestTwilioSerializer_InterruptionFrame(t *testing.T) {
	s := twilio.NewSerializer("stream-123", "", "", "", "", "", nil)
	f := frames.NewInterruptionFrame()
	data, err := s.Serialize(f)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if m["event"] != "clear" || m["streamSid"] != "stream-123" {
		t.Fatalf("unexpected: %v", m)
	}
}

func TestTwilioSerializer_DeserializeDTMF(t *testing.T) {
	s := twilio.NewSerializer("stream-123", "", "", "", "", "", nil)
	s.SampleRate = 16000
	msg := []byte(`{"event":"dtmf","dtmf":{"digit":"*"}}`)
	f, err := s.Deserialize(msg)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	in, ok := f.(*frames.InputDTMFFrame)
	if !ok {
		t.Fatalf("expected *frames.InputDTMFFrame, got %T", f)
	}
	if in.Digit != "*" {
		t.Fatalf("digit = %q, want *", in.Digit)
	}
}

func TestTwilioSerializer_DeserializeMedia(t *testing.T) {
	s := twilio.NewSerializer("stream-123", "", "", "", "", "", nil)
	s.SampleRate = 16000
	// Minimal μ-law payload (a few bytes) -> PCM -> should get AudioRawFrame
	ulaw := []byte{0xff, 0xfe, 0xfd, 0xfc}
	payload := base64.StdEncoding.EncodeToString(ulaw)
	msg, _ := json.Marshal(map[string]interface{}{
		"event": "media", "streamSid": "s",
		"media": map[string]string{"payload": payload},
	})
	f, err := s.Deserialize(msg)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if f == nil {
		return
	}
	_, ok := f.(*frames.AudioRawFrame)
	if !ok {
		t.Fatalf("expected *frames.AudioRawFrame, got %T", f)
	}
	_ = audio.DecodeULaw(ulaw)
}
