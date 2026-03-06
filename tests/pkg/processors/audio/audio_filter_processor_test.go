package audio_test

import (
	"context"
	"testing"

	audiofilters "voxray-go/pkg/audio/filters"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	procaudio "voxray-go/pkg/processors/audio"
)

func TestAudioFilterProcessor_appliesGainFilter(t *testing.T) {
	ctx := context.Background()
	sink := &collectProcessor{}

	// Simple 2-sample PCM16 buffer: [1000, -1000].
	audio := []byte{0xE8, 0x03, 0x18, 0xFC}
	frame := frames.NewAudioRawFrame(audio, 16000, 1, 0)

	chain := audiofilters.NewChain(audiofilters.NewGainFilter(0.5))
	afp := procaudio.NewAudioFilterProcessor("af", chain)
	afp.SetNext(sink)

	if err := afp.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame error: %v", err)
	}
	if len(sink.received) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(sink.received))
	}
	got, ok := sink.received[0].(*frames.AudioRawFrame)
	if !ok {
		t.Fatalf("expected AudioRawFrame, got %T", sink.received[0])
	}
	if len(got.Audio) != len(audio) {
		t.Fatalf("expected same length audio, got %d", len(got.Audio))
	}
	if string(got.Audio) == string(audio) {
		t.Fatalf("expected filtered audio to differ from input")
	}
	if got.NumFrames == 0 {
		t.Fatalf("expected NumFrames to be recomputed, got 0")
	}
}

