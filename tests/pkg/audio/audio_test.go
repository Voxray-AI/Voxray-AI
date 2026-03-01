package audio_test

import (
	"testing"

	"voila-go/pkg/audio"
)

func TestPCM16MonoNumFrames(t *testing.T) {
	if n := audio.PCM16MonoNumFrames(nil); n != 0 {
		t.Errorf("PCM16MonoNumFrames(nil) = %d, want 0", n)
	}
	if n := audio.PCM16MonoNumFrames([]byte{0, 0}); n != 1 {
		t.Errorf("PCM16MonoNumFrames(2 bytes) = %d, want 1", n)
	}
	if n := audio.PCM16MonoNumFrames(make([]byte, 100)); n != 50 {
		t.Errorf("PCM16MonoNumFrames(100 bytes) = %d, want 50", n)
	}
}

func TestDecodeWAVToPCM_TooShort(t *testing.T) {
	_, _, err := audio.DecodeWAVToPCM(make([]byte, 10))
	if err == nil {
		t.Error("DecodeWAVToPCM(short) should return error")
	}
}

func TestDecodeWAVToPCM_InvalidHeader(t *testing.T) {
	buf := make([]byte, 44)
	// Not RIFF
	_, _, err := audio.DecodeWAVToPCM(buf)
	if err == nil {
		t.Error("DecodeWAVToPCM(invalid header) should return error")
	}
}

func TestResample16Mono_SameRate(t *testing.T) {
	in := []byte{0, 0, 1, 0, 2, 0}
	out := make([]byte, 0, 10)
	got := audio.Resample16Mono(in, 16000, 16000, out)
	if len(got) != len(in) {
		t.Errorf("Resample16Mono(same rate) len = %d, want %d", len(got), len(in))
	}
}

func TestResample16MonoAlloc(t *testing.T) {
	in := []byte{0, 0, 1, 0}
	got := audio.Resample16MonoAlloc(in, 8000, 16000)
	if got == nil {
		t.Fatal("Resample16MonoAlloc returned nil")
	}
	// 2 input samples at 8k -> 4 output samples at 16k = 8 bytes
	if len(got) != 8 {
		t.Errorf("Resample16MonoAlloc len = %d, want 8", len(got))
	}
}

func TestMixMono(t *testing.T) {
	user := []byte{0x00, 0x00, 0x02, 0x00} // 0, 2 (LE)
	bot := []byte{0x04, 0x00, 0x06, 0x00}  // 4, 6
	got := audio.MixMono(user, bot)
	if len(got) != 4 {
		t.Fatalf("MixMono len = %d, want 4", len(got))
	}
	// (0+4)/2=2, (2+6)/2=4 -> 0x02,0x00, 0x04,0x00
	if got[0] != 2 || got[1] != 0 || got[2] != 4 || got[3] != 0 {
		t.Errorf("MixMono = %v, want [2,0,4,0]", got)
	}
	empty := audio.MixMono(nil, nil)
	if empty != nil {
		t.Errorf("MixMono(nil,nil) = %v, want nil", empty)
	}
}

func TestInterleaveStereo(t *testing.T) {
	left := []byte{0x01, 0x00, 0x03, 0x00}
	right := []byte{0x02, 0x00, 0x04, 0x00}
	got := audio.InterleaveStereo(left, right)
	// 2 samples -> 4 bytes per channel -> 8 bytes output (L,R,L,R)
	if len(got) != 8 {
		t.Fatalf("InterleaveStereo len = %d, want 8", len(got))
	}
	if got[0] != 1 || got[1] != 0 || got[2] != 2 || got[3] != 0 ||
		got[4] != 3 || got[5] != 0 || got[6] != 4 || got[7] != 0 {
		t.Errorf("InterleaveStereo = %v", got)
	}
}
