package pipeline_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services"
)

// TestSarvamVoicePipeline_E2E exercises an end-to-end voice pipeline using Sarvam
// for both STT and TTS:
// AudioRawFrame (hello.wav) -> Sarvam STT -> LLM -> Sarvam TTS -> TTSAudioRawFrame.
//
// It is skipped automatically if SARVAM_API_KEY is not configured in the environment
// or config.json.
func TestSarvamVoicePipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Sarvam voice E2E test in short mode")
	}

	// Locate the hello.wav fixture, mirroring the Groq E2E test behavior.
	candidatePaths := []string{
		filepath.Join("tests", "testdata", "hello.wav"),
		filepath.Join("..", "..", "testdata", "hello.wav"),
	}

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			candidatePaths = append(candidatePaths,
				filepath.Join(dir, "tests", "testdata", "hello.wav"),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var audioPath string
	for _, p := range candidatePaths {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		audioPath = p
		break
	}
	if audioPath == "" {
		t.Skipf("hello.wav fixture missing; add a small spoken-phrase WAV file at tests/testdata/hello.wav")
	}

	// Load config.json from common locations (same pattern as Groq E2E test).
	configPaths := []string{
		"config.json",
		filepath.Join("..", "..", "config.json"),
	}

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			configPaths = append(configPaths,
				filepath.Join(dir, "config.json"),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var (
		cfg *config.Config
		err error
	)
	for _, p := range configPaths {
		cfg, err = config.LoadConfig(p)
		if err == nil {
			break
		}
	}
	if cfg == nil {
		t.Skipf("config.json not available in expected locations: %v", err)
	}

	// Ensure we have a Sarvam API key either from config or environment.
	apiKey := cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	if apiKey == "" {
		t.Skip("SARVAM_API_KEY not configured and no sarvam key in config; skipping Sarvam voice E2E pipeline test")
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{}
	}
	cfg.APIKeys["sarvam"] = apiKey

	// Use Sarvam for STT and TTS. Leave LLM provider as-is (cfg.Provider / cfg.LlmProvider)
	// so that it uses whatever chat provider the user has configured.
	cfg.SttProvider = services.ProviderSarvam
	cfg.TtsProvider = services.ProviderSarvam
	if cfg.STTModel == "" {
		cfg.STTModel = "saarika:v2.5"
	}
	if cfg.TTSModel == "" {
		cfg.TTSModel = "bulbul:v2"
	}

	llmSvc, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)

	pl := pipeline.New()
	outCh := make(chan frames.Frame, 64)

	sttProc := voice.NewSTTProcessor("stt", sttSvc, 16000, 1)
	llmProc := voice.NewLLMProcessorWithSystemPrompt("llm", llmSvc, "You are a helpful voice assistant. Reply briefly and naturally to the user.")
	ttsProc := voice.NewTTSProcessor("tts", ttsSvc, 24000)
	sink := pipeline.NewSink("sink", outCh)

	// Minimal logging pipeline: STT -> LLM -> TTS -> sink.
	pl.Link(sttProc, llmProc, ttsProc, sink)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pl.Setup(ctx); err != nil {
		t.Fatalf("pipeline setup failed: %v", err)
	}
	defer pl.Cleanup(ctx)

	if err := pl.Start(ctx, frames.NewStartFrame()); err != nil {
		t.Fatalf("pipeline start failed: %v", err)
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("reading audio fixture %s: %v", audioPath, err)
	}

	audioFrame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)
	if err := pl.Push(ctx, audioFrame); err != nil {
		t.Fatalf("pushing AudioRawFrame into pipeline: %v", err)
	}

	// Wait for a TTSAudioRawFrame or fail if we only see timeout.
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for TTSAudioRawFrame from Sarvam voice pipeline")
		case f := <-outCh:
			switch v := f.(type) {
			case *frames.TTSAudioRawFrame:
				if len(v.Audio) == 0 {
					t.Fatal("received TTSAudioRawFrame with empty audio payload")
				}
				if v.SampleRate <= 0 {
					t.Fatalf("invalid sample rate on TTSAudioRawFrame: %d", v.SampleRate)
				}
				outPath := filepath.Join("tests", "testdata", "sarvam_tts_output.wav")
				if err := saveTTSAudioAsWAV(outPath, v.Audio, v.SampleRate, v.NumChannels); err != nil {
					t.Logf("failed to save Sarvam TTS output to %s: %v", outPath, err)
				}
				return
			case *frames.ErrorFrame:
				t.Fatalf("received ErrorFrame from processor %q: %s", v.Processor, v.Error)
			default:
				// ignore other frames
				_ = fmt.Sprintf("%T", f)
			}
		}
	}
}

