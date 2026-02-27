package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"voila-go/pkg/audio"
	"voila-go/pkg/audio/vad"
	"voila-go/pkg/logger"
	"voila-go/pkg/realtime"
	"voila-go/pkg/services"
)

// realtime-demo is a small CLI that wires together:
// - WAV decoding
// - a simple VAD
// - the OpenAI-backed RealtimeService
//
// It reads a WAV file (default hello.wav), runs VAD over 100ms frames, sends
// voiced frames to the realtime session as audio, and prints any text events.
func main() {
	wavPath := flag.String("wav", "hello.wav", "Path to input WAV file (16-bit PCM)")
	model := flag.String("model", "", "OpenAI chat model (defaults to library default)")
	flag.Parse()

	data, err := os.ReadFile(*wavPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav: %v\n", err)
		os.Exit(1)
	}

	pcm, sampleRate, err := audio.DecodeWAVToPCM(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode wav: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Construct a minimal OpenAI realtime provider (API key from env).
	rt := realtime.NewOpenAIRealtime("", *model)
	session, err := rt.NewSession(ctx, services.RealtimeConfig{
		Provider: "openai",
		Model:    *model,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "new session: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = session.Close(context.Background()) }()

	// Start receiver goroutine for events.
	go func() {
		for ev := range session.Events() {
			if ev.Text != nil {
				logger.Info("LLM text: %s", ev.Text.Text)
			}
		}
	}()

	// Run a simple energy-based VAD over 100ms frames and send only voiced frames.
	detector := vad.NewEnergyDetector()
	frameBytes := (sampleRate / 10) * 2 // 100ms of mono 16-bit
	now := time.Now()
	for offset := 0; offset < len(pcm); offset += frameBytes {
		end := offset + frameBytes
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := pcm[offset:end]
		f := audio.Frame{
			Data:        chunk,
			SampleRate:  sampleRate,
			NumChannels: 1,
			Timestamp:   now.Add(time.Duration(offset*int(time.Second)/max(1, len(pcm)))),
		}
		isSpeech, _ := detector.IsSpeech(f)
		if !isSpeech {
			continue
		}
		if err := session.SendAudio(ctx, chunk, sampleRate, 1); err != nil {
			fmt.Fprintf(os.Stderr, "send audio: %v\n", err)
			break
		}
	}

	// Optionally send a text prompt tying everything together.
	if err := session.SendText(ctx, "Please summarize what the user said."); err != nil {
		fmt.Fprintf(os.Stderr, "send text: %v\n", err)
	}

	// Allow some time for responses before exit.
	time.Sleep(5 * time.Second)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

