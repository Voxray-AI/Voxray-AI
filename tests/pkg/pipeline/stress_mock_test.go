// Package pipeline_test contains stress tests for the voice pipeline using mock STT, LLM, TTS
// and in-memory transport. Same flow as web/index.html: mic → STT → LLM → TTS → speaker.
package pipeline_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services/mock"
	"voxray-go/pkg/transport/memory"
)

// minAudioBytes is the minimum audio to trigger STT (voice.MinSTTBufferMs = 500ms at 16kHz mono 16-bit).
const minAudioBytes = 16000

// runMockSession runs one pipeline session: in-memory transport + mock STT/LLM/TTS, same chain as WebRTC flow.
// Pushes synthetic audio, reads until one TTSAudioRawFrame or timeout. Returns true if TTS was received.
func runMockSession(ctx context.Context, mockSTT *mock.STT, mockLLM *mock.LLM, mockTTS *mock.TTS) (gotTTS bool, err error) {
	tr := memory.NewTransport()
	pl := pipeline.New()
	pl.Add(voice.NewSTTProcessor("stt", mockSTT, 16000, 1))
	pl.Add(voice.NewLLMProcessorWithSystemPrompt("llm", mockLLM, "You are a helpful voice assistant. Reply briefly."))
	pl.Add(voice.NewTTSProcessor("tts", mockTTS, 24000))
	pl.Add(pipeline.NewSink("sink", tr.Output()))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := pipeline.NewRunner(pl, tr, frames.NewStartFrame())
	go func() {
		_ = runner.Run(runCtx)
	}()

	// Enough audio to exceed STT buffer (500ms at 16kHz mono)
	pcm := make([]byte, minAudioBytes)
	audioFrame := frames.NewAudioRawFrame(pcm, 16000, 1, 0)
	if !tr.SendInput(ctx, audioFrame) {
		return false, ctx.Err()
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline:
			cancel()
			return gotTTS, nil
		case f, ok := <-tr.Out():
			if !ok {
				return gotTTS, nil
			}
			switch f.(type) {
			case *frames.TTSAudioRawFrame:
				gotTTS = true
				cancel()
				<-runner.Done()
				return true, nil
			case *frames.ErrorFrame:
				cancel()
				<-runner.Done()
				return false, nil
			}
		}
	}
}

// TestMockPipeline_SingleSession verifies one session completes: synthetic audio → mock STT → mock LLM → mock TTS → output.
func TestMockPipeline_SingleSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mockSTT := mock.NewSTTWithTranscript("hello")
	mockLLM := mock.NewLLMWithResponse("hi there")
	mockTTS := mock.NewTTS()

	gotTTS, err := runMockSession(ctx, mockSTT, mockLLM, mockTTS)
	if err != nil {
		t.Fatalf("session error: %v", err)
	}
	if !gotTTS {
		t.Fatal("expected at least one TTSAudioRawFrame from mock pipeline")
	}
}

// TestMockPipeline_Stress runs many concurrent sessions (same pipeline flow as web/index.html) with shared mocks.
func TestMockPipeline_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const concurrency = 50
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Shared mocks (stateless for STT/TTS; LLM is read-only response string).
	mockSTT := mock.NewSTTWithTranscript("hello")
	mockLLM := mock.NewLLMWithResponse("hi")
	mockTTS := mock.NewTTS()

	var done int32
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, _ := runMockSession(ctx, mockSTT, mockLLM, mockTTS)
			if got {
				atomic.AddInt32(&done, 1)
			}
		}()
	}
	wg.Wait()

	got := atomic.LoadInt32(&done)
	if got < concurrency/2 {
		t.Errorf("stress: only %d/%d sessions got TTS (expected at least half)", got, concurrency)
	}
}
