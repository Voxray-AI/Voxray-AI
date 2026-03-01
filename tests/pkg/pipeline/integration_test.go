package pipeline_test

import (
	"context"
	"testing"
	"time"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/audio/vad"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/audio/turn"
)

// sinkCollector is a simple sink processor used to collect frames at the end of
// an integration pipeline.
type sinkCollector struct {
	*processors.BaseProcessor
	collected []frames.Frame
}

func newSinkCollector() *sinkCollector {
	return &sinkCollector{
		BaseProcessor: processors.NewBaseProcessor("sink"),
		collected:     make([]frames.Frame, 0),
	}
}

func (s *sinkCollector) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Downstream {
		s.collected = append(s.collected, f)
	}
	return s.BaseProcessor.ProcessFrame(ctx, f, dir)
}

// voiceTestAnalyzer is a minimal implementation of turn.Analyzer used only for
// this integration test. It treats the first few appended chunks as ongoing
// speech and reports completion once a fixed threshold has been reached.
type voiceTestAnalyzer struct {
	samples int
}

func (a *voiceTestAnalyzer) AppendAudio(buf []byte, _ bool) turn.EndOfTurnState {
	const threshold = 4
	if len(buf) == 0 {
		return turn.Incomplete
	}
	a.samples++
	if a.samples >= threshold {
		return turn.Complete
	}
	return turn.Incomplete
}

func (*voiceTestAnalyzer) AnalyzeEndOfTurn(context.Context) (turn.EndOfTurnState, error) {
	return turn.Complete, nil
}

func (*voiceTestAnalyzer) AnalyzeEndOfTurnAsync(context.Context) <-chan turn.EndOfTurnResult {
	ch := make(chan turn.EndOfTurnResult, 1)
	ch <- turn.EndOfTurnResult{State: turn.Complete}
	return ch
}

func (a *voiceTestAnalyzer) SpeechTriggered() bool {
	return a.samples > 0
}

func (*voiceTestAnalyzer) SetSampleRate(int) {}
func (a *voiceTestAnalyzer) Clear()          { a.samples = 0 }
func (*voiceTestAnalyzer) UpdateVADStartSecs(float64) {}
func (*voiceTestAnalyzer) UpdateParams(turn.Params)   {}

// TestVoiceTurnIntegration builds a small pipeline using TurnProcessor that
// receives audio frames, performs VAD + user turn analysis, and emits a single
// concatenated AudioRawFrame when the turn completes, along with high-level
// user turn control frames. This mirrors the style of upstream pipeline tests
// that exercise multi-processor flows end-to-end.
func TestVoiceTurnIntegration(t *testing.T) {
	ctx := context.Background()

	// Use the default energy-based VAD and a simple analyzer.
	detector := vad.NewEnergyDetector()
	analyzer := &voiceTestAnalyzer{}

	turnProc := voice.NewTurnProcessor("turn", detector, analyzer, audio.DefaultInSampleRate, 1, false)
	sink := newSinkCollector()

	pl := pipeline.New()
	pl.Link(turnProc, sink)

	if err := pl.Setup(ctx); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer pl.Cleanup(ctx)

	if err := pl.Start(ctx, frames.NewStartFrame()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Push several small audio chunks that should be treated as continuous speech.
	chunk := make([]byte, 320)
	for i := 0; i < len(chunk); i += 2 {
		chunk[i] = 0xFF
		chunk[i+1] = 0x7F
	}

	for i := 0; i < 3; i++ {
		if err := pl.Push(ctx, frames.NewAudioRawFrame(chunk, audio.DefaultInSampleRate, 1, 0)); err != nil {
			t.Fatalf("Push audio chunk %d failed: %v", i, err)
		}
	}

	// Push a final chunk and then allow the analyzer to mark the turn complete.
	if err := pl.Push(ctx, frames.NewAudioRawFrame(chunk, audio.DefaultInSampleRate, 1, 0)); err != nil {
		t.Fatalf("Push final audio chunk failed: %v", err)
	}

	// Wait for the sink to receive the concatenated AudioRawFrame.
	deadline := time.Now().Add(2 * time.Second)
	var gotAudio *frames.AudioRawFrame
	for time.Now().Before(deadline) {
		for _, f := range sink.collected {
			if af, ok := f.(*frames.AudioRawFrame); ok {
				gotAudio = af
			}
		}
		if gotAudio != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if gotAudio == nil {
		t.Fatal("expected concatenated AudioRawFrame at sink")
	}
	if gotAudio.SampleRate != audio.DefaultInSampleRate {
		t.Fatalf("unexpected SampleRate: got %d", gotAudio.SampleRate)
	}
	// We pushed 4 chunks; each should have the same length.
	expectedLen := len(chunk) * 4
	if len(gotAudio.Audio) != expectedLen {
		t.Fatalf("expected concatenated audio length %d, got %d", expectedLen, len(gotAudio.Audio))
	}
}

