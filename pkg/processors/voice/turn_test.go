package voice

import (
	"context"
	"testing"
	"time"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/audio/turn"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

type fakeVAD struct {
	isSpeech bool
}

func (f *fakeVAD) IsSpeech(_ audio.Frame) (bool, error) {
	return f.isSpeech, nil
}

func (f *fakeVAD) SetSampleRate(_ int) {}

type fakeAnalyzer struct {
	state          turn.EndOfTurnState
	speechTriggered bool
	resultCh       chan turn.EndOfTurnResult
}

func (f *fakeAnalyzer) AppendAudio(_ []byte, _ bool) turn.EndOfTurnState {
	f.speechTriggered = true
	return f.state
}

func (f *fakeAnalyzer) AnalyzeEndOfTurn(ctx context.Context) (turn.EndOfTurnState, error) {
	return f.state, nil
}

func (f *fakeAnalyzer) AnalyzeEndOfTurnAsync(ctx context.Context) <-chan turn.EndOfTurnResult {
	if f.resultCh == nil {
		f.resultCh = make(chan turn.EndOfTurnResult, 1)
	}
	return f.resultCh
}

func (f *fakeAnalyzer) SpeechTriggered() bool {
	return f.speechTriggered
}

func (f *fakeAnalyzer) SetSampleRate(_ int) {}

func (f *fakeAnalyzer) Clear() {
	f.speechTriggered = false
}

func (f *fakeAnalyzer) UpdateVADStartSecs(_ float64) {}

func (f *fakeAnalyzer) UpdateParams(_ turn.Params) {}

type collectProcessor struct {
	received []frames.Frame
}

func (c *collectProcessor) ProcessFrame(_ context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Downstream {
		c.received = append(c.received, f)
	}
	return nil
}

func (c *collectProcessor) SetNext(processors.Processor)  {}
func (c *collectProcessor) SetPrev(processors.Processor)  {}
func (c *collectProcessor) Setup(context.Context) error   { return nil }
func (c *collectProcessor) Cleanup(context.Context) error { return nil }
func (c *collectProcessor) Name() string                  { return "collector" }

// findFrame searches the collected frames for the first instance of the given type.
func (c *collectProcessor) findFrame(target func(frames.Frame) bool) frames.Frame {
	for _, f := range c.received {
		if target(f) {
			return f
		}
	}
	return nil
}

func TestTurnProcessorSyncModeEmitsOnComplete(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Complete}
	v := &fakeVAD{isSpeech: true}
	p := NewTurnProcessor("turn", v, an, 16000, 1, false)

	col := &collectProcessor{}
	p.SetNext(col)

	audioData := []byte{0x00, 0x00, 0x01, 0x00}
	frame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)

	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame returned error: %v", err)
	}

	var out *frames.AudioRawFrame
	for _, f := range col.received {
		if af, ok := f.(*frames.AudioRawFrame); ok {
			out = af
			break
		}
	}
	if out == nil {
		t.Fatalf("expected downstream AudioRawFrame, got %T", col.received[0])
	}
	if len(out.Audio) != len(audioData) {
		t.Fatalf("expected audio length %d, got %d", len(audioData), len(out.Audio))
	}
}

func TestTurnProcessorAsyncModeEmitsOnAsyncComplete(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Incomplete}
	v := &fakeVAD{isSpeech: true}
	p := NewTurnProcessor("turn", v, an, 16000, 1, true)

	col := &collectProcessor{}
	p.SetNext(col)

	chunk1 := []byte{0x00, 0x00, 0x01, 0x00}
	chunk2 := []byte{0x02, 0x00, 0x03, 0x00}

	frame1 := frames.NewAudioRawFrame(chunk1, 16000, 1, 0)
	if err := p.ProcessFrame(ctx, frame1, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(1) returned error: %v", err)
	}

	// At this point an async analysis should be pending; provide a complete result.
	if an.resultCh == nil {
		t.Fatalf("expected async result channel to be initialized")
	}
	an.resultCh <- turn.EndOfTurnResult{State: turn.Complete, Err: nil}

	frame2 := frames.NewAudioRawFrame(chunk2, 16000, 1, 0)
	if err := p.ProcessFrame(ctx, frame2, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(2) returned error: %v", err)
	}

	var out *frames.AudioRawFrame
	for _, f := range col.received {
		if af, ok := f.(*frames.AudioRawFrame); ok {
			out = af
			break
		}
	}
	if out == nil {
		t.Fatalf("expected downstream AudioRawFrame, got %T", col.received[0])
	}
	expectedLen := len(chunk1) + len(chunk2)
	if len(out.Audio) != expectedLen {
		t.Fatalf("expected audio length %d, got %d", expectedLen, len(out.Audio))
	}
}

func TestTurnProcessorCancelClearsState(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Incomplete}
	v := &fakeVAD{isSpeech: true}
	p := NewTurnProcessor("turn", v, an, 16000, 1, true)

	col := &collectProcessor{}
	p.SetNext(col)

	frame := frames.NewAudioRawFrame([]byte{0x00, 0x00}, 16000, 1, 0)
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame audio returned error: %v", err)
	}

	cancel := frames.NewCancelFrame("test")
	if err := p.ProcessFrame(ctx, cancel, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame cancel returned error: %v", err)
	}

	if len(col.received) == 0 {
		t.Fatalf("expected at least one frame downstream")
	}
	if _, ok := col.received[len(col.received)-1].(*frames.CancelFrame); !ok {
		t.Fatalf("expected last frame to be CancelFrame, got %T", col.received[len(col.received)-1])
	}
}

func TestTurnProcessorEmitsUserTurnFrames(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Complete}
	v := &fakeVAD{isSpeech: true}
	p := NewTurnProcessor("turn", v, an, 16000, 1, false)

	col := &collectProcessor{}
	p.SetNext(col)

	audioData := []byte{0x00, 0x00, 0x01, 0x00}
	frame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)

	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame returned error: %v", err)
	}

	if col.findFrame(func(f frames.Frame) bool {
		_, ok := f.(*frames.UserStartedSpeakingFrame)
		return ok
	}) == nil {
		t.Fatal("expected UserStartedSpeakingFrame downstream")
	}
}

// TestTurnProcessorUserStopAndIdleTimeout mirrors the behavior verified by the
// upstream Python user turn controller tests in a simplified form: when VAD
// reports speech, a UserStartedSpeakingFrame is emitted; when speech stops and
// the stop timeout elapses, a UserStoppedSpeakingFrame is emitted; if the bot
// has finished speaking and the idle timeout elapses with no further user
// speech, a UserIdleFrame is emitted.
func TestTurnProcessorUserStopAndIdleTimeout(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Incomplete}
	v := &fakeVAD{isSpeech: true}

	// Use short timeouts so the test completes quickly.
	const stopTimeout = 0.1
	const idleTimeout = 0.1

	p := NewTurnProcessorWithUserTurn("turn", v, an, 16000, 1, false, stopTimeout, idleTimeout, 0)

	col := &collectProcessor{}
	p.SetNext(col)

	// While user is speaking, only UserStartedSpeakingFrame should appear.
	audioData := []byte{0x00, 0x00, 0x01, 0x00}
	frame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(speaking) returned error: %v", err)
	}
	if col.findFrame(func(f frames.Frame) bool {
		_, ok := f.(*frames.UserStartedSpeakingFrame)
		return ok
	}) == nil {
		t.Fatal("expected UserStartedSpeakingFrame while speaking")
	}
	if col.findFrame(func(f frames.Frame) bool {
		_, ok := f.(*frames.UserStoppedSpeakingFrame)
		return ok
	}) != nil {
		t.Fatal("did not expect UserStoppedSpeakingFrame while still speaking")
	}

	// User stops speaking: flip VAD to false and wait for stop timeout.
	v.isSpeech = false
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(quiet) returned error: %v", err)
	}

	time.Sleep(time.Duration(stopTimeout*float64(time.Second)) + 20*time.Millisecond)

	if col.findFrame(func(f frames.Frame) bool {
		_, ok := f.(*frames.UserStoppedSpeakingFrame)
		return ok
	}) == nil {
		t.Fatal("expected UserStoppedSpeakingFrame after stop timeout")
	}

	// Simulate bot finishing speaking so idle timeout can trigger.
	if p.userTurnController == nil {
		t.Fatal("expected userTurnController to be initialized")
	}
	p.userTurnController.NotifyBotStoppedSpeaking(ctx)

	time.Sleep(time.Duration(idleTimeout*float64(time.Second)) + 20*time.Millisecond)

	if col.findFrame(func(f frames.Frame) bool {
		_, ok := f.(*frames.UserIdleFrame)
		return ok
	}) == nil {
		t.Fatal("expected UserIdleFrame after idle timeout")
	}
}

// TestTurnProcessorBargeInEmitsUserStartedSpeakingWhenReSpeaking verifies that when the user
// starts speaking again while still in a turn (e.g. interrupts the bot before the stop timeout),
// UserStartedSpeakingFrame is emitted so barge-in works.
func TestTurnProcessorBargeInEmitsUserStartedSpeakingWhenReSpeaking(t *testing.T) {
	ctx := context.Background()
	an := &fakeAnalyzer{state: turn.Incomplete}
	v := &fakeVAD{isSpeech: true}

	// Long stop timeout so we stay in "user turn" when user re-speaks (barge-in).
	const stopTimeout = 1.0
	const idleTimeout = 0.0

	p := NewTurnProcessorWithUserTurn("turn", v, an, 16000, 1, false, stopTimeout, idleTimeout, 0)

	col := &collectProcessor{}
	p.SetNext(col)

	audioData := []byte{0x00, 0x00, 0x01, 0x00}
	frame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)

	// 1) User speaks -> UserStartedSpeakingFrame (and turn starts).
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(speak) returned error: %v", err)
	}
	var startedCount int
	for _, f := range col.received {
		if _, ok := f.(*frames.UserStartedSpeakingFrame); ok {
			startedCount++
		}
	}
	if startedCount != 1 {
		t.Fatalf("after first speech: expected 1 UserStartedSpeakingFrame, got %d", startedCount)
	}

	// 2) User stops (VAD false) - do not wait for stop timeout, so userTurn remains true.
	v.isSpeech = false
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(silence) returned error: %v", err)
	}

	// 3) User speaks again (barge-in) -> must emit UserStartedSpeakingFrame again.
	v.isSpeech = true
	if err := p.ProcessFrame(ctx, frame, processors.Downstream); err != nil {
		t.Fatalf("ProcessFrame(re-speak) returned error: %v", err)
	}
	startedCount = 0
	for _, f := range col.received {
		if _, ok := f.(*frames.UserStartedSpeakingFrame); ok {
			startedCount++
		}
	}
	if startedCount != 2 {
		t.Fatalf("after re-speak (barge-in): expected 2 UserStartedSpeakingFrames total, got %d", startedCount)
	}
}

