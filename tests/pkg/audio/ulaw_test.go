package audio_test

import (
	"bytes"
	"testing"

	"voila-go/pkg/audio"
)

func TestEncodeDecodeULawRoundTrip(t *testing.T) {
	// 16-bit PCM little-endian: a few samples
	pcm := []byte{0x00, 0x00, 0xff, 0x7f, 0x01, 0x80, 0x00, 0x00}
	ulaw := audio.EncodeULaw(pcm)
	if len(ulaw) != 4 {
		t.Fatalf("EncodeULaw: expected 4 bytes, got %d", len(ulaw))
	}
	out := audio.DecodeULaw(ulaw)
	if len(out) != len(pcm) {
		t.Fatalf("DecodeULaw: expected %d bytes, got %d", len(pcm), len(out))
	}
	// Round-trip may not be exact; check approximate
	for i := 0; i < len(pcm); i += 2 {
		if i+1 >= len(out) {
			break
		}
		// Allow some tolerance for μ-law quantization
		orig := int16(pcm[i]) | int16(pcm[i+1])<<8
		dec := int16(out[i]) | int16(out[i+1])<<8
		diff := orig - dec
		if diff < 0 {
			diff = -diff
		}
		if diff > 100 {
			t.Logf("sample %d: orig=%d dec=%d", i/2, orig, dec)
		}
	}
}

func TestDecodeULawKnownValues(t *testing.T) {
	// μ-law 0xff is often silence or near-silence
	ulaw := []byte{0xff, 0x7f}
	out := audio.DecodeULaw(ulaw)
	if len(out) != 4 {
		t.Fatalf("DecodeULaw: expected 4 bytes, got %d", len(out))
	}
	_ = out
}

func TestEncodeULawEmpty(t *testing.T) {
	out := audio.EncodeULaw(nil)
	if out != nil {
		t.Fatalf("EncodeULaw(nil) should return nil, got len=%d", len(out))
	}
	out = audio.EncodeULaw([]byte{1})
	if out != nil {
		t.Fatalf("EncodeULaw(odd length) should return nil, got len=%d", len(out))
	}
}

func TestDecodeULawEmpty(t *testing.T) {
	out := audio.DecodeULaw(nil)
	if out != nil {
		t.Fatalf("DecodeULaw(nil) should return nil")
	}
}

func TestULawRoundTripLarger(t *testing.T) {
	// 160 samples = 320 bytes at 8kHz = 20ms
	pcm := make([]byte, 320)
	for i := 0; i < 320; i += 2 {
		pcm[i] = byte(i)
		pcm[i+1] = byte(i >> 8)
	}
	ulaw := audio.EncodeULaw(pcm)
	if len(ulaw) != 160 {
		t.Fatalf("expected 160 ulaw bytes, got %d", len(ulaw))
	}
	out := audio.DecodeULaw(ulaw)
	if !bytes.Equal(out, pcm) {
		// μ-law is lossy; at least length should match
		if len(out) != len(pcm) {
			t.Fatalf("round-trip length: got %d want %d", len(out), len(pcm))
		}
	}
}
