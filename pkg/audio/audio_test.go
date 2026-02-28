package audio

import "testing"

func TestPCM16MonoNumFrames(t *testing.T) {
	if n := PCM16MonoNumFrames(nil); n != 0 {
		t.Errorf("PCM16MonoNumFrames(nil) = %d, want 0", n)
	}
	if n := PCM16MonoNumFrames([]byte{0, 0}); n != 1 {
		t.Errorf("PCM16MonoNumFrames(2 bytes) = %d, want 1", n)
	}
	if n := PCM16MonoNumFrames(make([]byte, 100)); n != 50 {
		t.Errorf("PCM16MonoNumFrames(100 bytes) = %d, want 50", n)
	}
}

func TestDecodeWAVToPCM_TooShort(t *testing.T) {
	_, _, err := DecodeWAVToPCM(make([]byte, 10))
	if err == nil {
		t.Error("DecodeWAVToPCM(short) should return error")
	}
}

func TestDecodeWAVToPCM_InvalidHeader(t *testing.T) {
	buf := make([]byte, 44)
	// Not RIFF
	_, _, err := DecodeWAVToPCM(buf)
	if err == nil {
		t.Error("DecodeWAVToPCM(invalid header) should return error")
	}
}

func TestResample16Mono_SameRate(t *testing.T) {
	in := []byte{0, 0, 1, 0, 2, 0}
	out := make([]byte, 0, 10)
	got := Resample16Mono(in, 16000, 16000, out)
	if len(got) != len(in) {
		t.Errorf("Resample16Mono(same rate) len = %d, want %d", len(got), len(in))
	}
}

func TestResample16MonoAlloc(t *testing.T) {
	in := []byte{0, 0, 1, 0}
	got := Resample16MonoAlloc(in, 8000, 16000)
	if got == nil {
		t.Fatal("Resample16MonoAlloc returned nil")
	}
	// 2 input samples at 8k -> 4 output samples at 16k = 8 bytes
	if len(got) != 8 {
		t.Errorf("Resample16MonoAlloc len = %d, want 8", len(got))
	}
}
