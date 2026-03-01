package vonage_test

import (
	"encoding/json"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
	"voila-go/pkg/frames/serialize/vonage"
)

func TestVonageSerializer_SerializeWithType_AudioBinary(t *testing.T) {
	s := vonage.NewSerializer(nil)
	s.SampleRate = 16000
	s.VonageRate = 16000
	pcm := []byte{0, 0, 1, 0, 2, 0}
	ar := frames.NewAudioRawFrame(pcm, 16000, 1, 0)
	data, binary, err := s.SerializeWithType(ar)
	if err != nil {
		t.Fatalf("SerializeWithType: %v", err)
	}
	if !binary {
		t.Fatal("expected binary=true for audio")
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}
}

func TestVonageSerializer_SerializeWithType_InterruptionText(t *testing.T) {
	s := vonage.NewSerializer(nil)
	f := frames.NewInterruptionFrame()
	data, binary, err := s.SerializeWithType(f)
	if err != nil {
		t.Fatalf("SerializeWithType: %v", err)
	}
	if binary {
		t.Fatal("expected binary=false for JSON")
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if m["action"] != "clear" {
		t.Fatalf("action = %q, want clear", m["action"])
	}
}

func TestVonageSerializer_Deserialize_JSONEventIgnored(t *testing.T) {
	s := vonage.NewSerializer(nil)
	msg := []byte(`{"event":"websocket:connected"}`)
	f, err := s.Deserialize(msg)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if f != nil {
		t.Fatalf("expected nil for websocket:connected, got %T", f)
	}
}

func TestVonageSerializer_Deserialize_DTMF(t *testing.T) {
	s := vonage.NewSerializer(nil)
	msg := []byte(`{"event":"websocket:dtmf","digit":"5"}`)
	f, err := s.Deserialize(msg)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	in, ok := f.(*frames.InputDTMFFrame)
	if !ok {
		t.Fatalf("expected *frames.InputDTMFFrame, got %T", f)
	}
	if in.Digit != "5" {
		t.Fatalf("digit = %q, want 5", in.Digit)
	}
}

func TestVonageSerializer_ImplementsSerializerWithMessageType(t *testing.T) {
	var _ serialize.SerializerWithMessageType = (*vonage.Serializer)(nil)
}
