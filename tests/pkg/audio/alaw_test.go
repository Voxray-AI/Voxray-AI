package audio_test

import (
	"testing"

	"voila-go/pkg/audio"
)

func TestEncodeDecodeALawRoundTrip(t *testing.T) {
	pcm := []byte{0x00, 0x00, 0xff, 0x7f, 0x01, 0x80}
	alaw := audio.EncodeALaw(pcm)
	if len(alaw) != 3 {
		t.Fatalf("EncodeALaw: expected 3 bytes, got %d", len(alaw))
	}
	out := audio.DecodeALaw(alaw)
	if len(out) != len(pcm) {
		t.Fatalf("DecodeALaw: expected %d bytes, got %d", len(pcm), len(out))
	}
}

func TestEncodeALawEmpty(t *testing.T) {
	out := audio.EncodeALaw(nil)
	if out != nil {
		t.Fatalf("EncodeALaw(nil) should return nil")
	}
}

func TestDecodeALawEmpty(t *testing.T) {
	out := audio.DecodeALaw(nil)
	if out != nil {
		t.Fatalf("DecodeALaw(nil) should return nil")
	}
}

func TestALawRoundTripLarger(t *testing.T) {
	pcm := make([]byte, 320)
	for i := 0; i < 320; i += 2 {
		pcm[i] = byte(i)
		pcm[i+1] = byte(i >> 8)
	}
	alaw := audio.EncodeALaw(pcm)
	if len(alaw) != 160 {
		t.Fatalf("expected 160 alaw bytes, got %d", len(alaw))
	}
	out := audio.DecodeALaw(alaw)
	if len(out) != len(pcm) {
		t.Fatalf("round-trip length: got %d want %d", len(out), len(pcm))
	}
}
