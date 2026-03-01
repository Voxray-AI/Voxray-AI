package audio_test

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/audio"
)

func TestAudioBufferProcessor_TurnCallbacks(t *testing.T) {
	ctx := context.Background()
	var userTurnBuf []byte
	var botTurnBuf []byte
	p := audio.NewAudioBufferProcessor("buf", 16000, 1, 0, true)
	p.OnUserTurnAudioData = func(buf []byte, sampleRate, numChannels int) {
		userTurnBuf = append(userTurnBuf, buf...)
	}
	p.OnBotTurnAudioData = func(buf []byte, sampleRate, numChannels int) {
		botTurnBuf = append(botTurnBuf, buf...)
	}
	p.StartRecording()

	col := &collectProcessor{}
	p.SetNext(col)

	start := frames.NewStartFrame()
	start.AudioOutSampleRate = 16000
	_ = p.ProcessFrame(ctx, start, processors.Downstream)

	_ = p.ProcessFrame(ctx, frames.NewUserStartedSpeakingFrame(), processors.Downstream)
	chunk := []byte{0x01, 0x00, 0x02, 0x00}
	_ = p.ProcessFrame(ctx, frames.NewAudioRawFrame(chunk, 16000, 1, 0), processors.Downstream)
	_ = p.ProcessFrame(ctx, frames.NewUserStoppedSpeakingFrame(), processors.Downstream)

	if len(userTurnBuf) == 0 {
		t.Error("expected OnUserTurnAudioData to be called with user audio")
	}

	_ = p.ProcessFrame(ctx, frames.NewBotStartedSpeakingFrame(), processors.Downstream)
	ttsChunk := []byte{0x03, 0x00, 0x04, 0x00}
	_ = p.ProcessFrame(ctx, frames.NewTTSAudioRawFrame(ttsChunk, 16000), processors.Downstream)
	_ = p.ProcessFrame(ctx, frames.NewBotStoppedSpeakingFrame(), processors.Downstream)

	if len(botTurnBuf) == 0 {
		t.Error("expected OnBotTurnAudioData to be called with bot audio")
	}
}

func TestAudioBufferProcessor_BufferSizeCallbacks(t *testing.T) {
	ctx := context.Background()
	var merged [][]byte
	var trackUser, trackBot [][]byte
	p := audio.NewAudioBufferProcessor("buf", 16000, 1, 8, false)
	p.OnAudioData = func(m []byte, _, _ int) {
		merged = append(merged, m)
	}
	p.OnTrackAudioData = func(u, b []byte, _, _ int) {
		trackUser = append(trackUser, u)
		trackBot = append(trackBot, b)
	}
	p.StartRecording()

	col := &collectProcessor{}
	p.SetNext(col)

	start := frames.NewStartFrame()
	start.AudioOutSampleRate = 16000
	_ = p.ProcessFrame(ctx, start, processors.Downstream)

	userChunk := make([]byte, 16)
	for i := range userChunk {
		userChunk[i] = byte(i)
	}
	_ = p.ProcessFrame(ctx, frames.NewAudioRawFrame(userChunk, 16000, 1, 0), processors.Downstream)

	if len(merged) == 0 || len(trackUser) == 0 {
		t.Error("expected OnAudioData and OnTrackAudioData when buffer size reached")
	}
}

func TestAudioBufferProcessor_StopRecordingFlushes(t *testing.T) {
	ctx := context.Background()
	var merged [][]byte
	p := audio.NewAudioBufferProcessor("buf", 16000, 1, 0, false)
	p.OnAudioData = func(m []byte, _, _ int) {
		merged = append(merged, m)
	}
	p.StartRecording()
	p.SetNext(&collectProcessor{})

	start := frames.NewStartFrame()
	start.AudioOutSampleRate = 16000
	_ = p.ProcessFrame(ctx, start, processors.Downstream)
	_ = p.ProcessFrame(ctx, frames.NewAudioRawFrame([]byte{1, 0, 2, 0}, 16000, 1, 0), processors.Downstream)

	p.StopRecording(ctx)
	if len(merged) == 0 {
		t.Error("expected OnAudioData on StopRecording flush")
	}
}

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
func (c *collectProcessor) Setup(context.Context) error  { return nil }
func (c *collectProcessor) Cleanup(context.Context) error { return nil }
func (c *collectProcessor) Name() string                 { return "collector" }
