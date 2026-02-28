package serialize

import (
	"testing"

	"voila-go/pkg/frames"
)

func TestEncoderDecoderRoundTrip(t *testing.T) {
	text := frames.NewTextFrame("hello")
	data, err := Encoder(text)
	if err != nil {
		t.Fatalf("Encoder: %v", err)
	}
	decoded, err := Decoder(data)
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	tf, ok := decoded.(*frames.TextFrame)
	if !ok {
		t.Fatalf("expected *frames.TextFrame, got %T", decoded)
	}
	if tf.Text != "hello" {
		t.Errorf("Text = %q, want hello", tf.Text)
	}
}

func TestDecodeByType_UnknownType(t *testing.T) {
	_, err := DecodeByType("UnknownFrameType", []byte("{}"))
	if err == nil {
		t.Error("DecodeByType(unknown type) should return error")
	}
}
