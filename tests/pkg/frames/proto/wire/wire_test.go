package wire_test

import (
	"testing"

	"voila-go/pkg/frames/proto/wire"
)

func TestFrameEnvelope_MarshalUnmarshalRoundTrip(t *testing.T) {
	env := &wire.FrameEnvelope{Type: "TextFrame", Payload: []byte(`{"text":"hi"}`)}
	data, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out wire.FrameEnvelope
	if err := out.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Type != env.Type {
		t.Errorf("Type = %q, want %q", out.Type, env.Type)
	}
	if string(out.Payload) != string(env.Payload) {
		t.Errorf("Payload = %q, want %q", out.Payload, env.Payload)
	}
}

func TestFrameEnvelope_Empty(t *testing.T) {
	env := &wire.FrameEnvelope{}
	data, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out wire.FrameEnvelope
	if err := out.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Type != "" || len(out.Payload) != 0 {
		t.Errorf("empty envelope round-trip: Type=%q Payload=%v", out.Type, out.Payload)
	}
}
