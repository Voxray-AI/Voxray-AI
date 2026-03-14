package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pion/webrtc/v3"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/server"
	"voxray-go/pkg/services"
	"voxray-go/pkg/transport"
)

// TestWebRTCSarvamGroqVoicePipeline_E2E exercises the voice pipeline used for the WebRTC flow:
// Sarvam STT + Groq LLM + Sarvam TTS, with in-memory transport (no real WebRTC).
// Validates that the same pipeline configuration works for the WebRTC integration.
func TestWebRTCSarvamGroqVoicePipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WebRTC Sarvam+Groq voice E2E test in short mode")
	}

	candidatePaths := []string{
		filepath.Join("tests", "testdata", "hello.wav"),
		filepath.Join("..", "..", "testdata", "hello.wav"),
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			candidatePaths = append(candidatePaths, filepath.Join(dir, "tests", "testdata", "hello.wav"))
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

	configPaths := []string{"config.json", filepath.Join("..", "..", "config.json")}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			configPaths = append(configPaths, filepath.Join(dir, "config.json"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var cfg *config.Config
	var err error
	for _, p := range configPaths {
		cfg, err = config.LoadConfig(p)
		if err == nil {
			break
		}
	}
	if cfg == nil {
		t.Skipf("config.json not available in expected locations: %v", err)
	}

	sarvamKey := cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	groqKey := cfg.GetAPIKey("groq", "GROQ_API_KEY")
	if sarvamKey == "" || groqKey == "" {
		t.Skip("SARVAM_API_KEY and GROQ_API_KEY required for WebRTC Sarvam+Groq E2E test")
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{}
	}
	cfg.APIKeys["sarvam"] = sarvamKey
	cfg.APIKeys["groq"] = groqKey

	cfg.SttProvider = services.ProviderSarvam
	cfg.LlmProvider = services.ProviderGroq
	cfg.TtsProvider = services.ProviderSarvam
	if cfg.Model == "" {
		cfg.Model = "llama-3.1-8b-instant"
	}
	if cfg.STTModel == "" {
		cfg.STTModel = "saarika:v2.5"
	}
	if cfg.TTSModel == "" {
		cfg.TTSModel = "bulbul:v2"
	}

	llmSvc, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)
	pl := pipeline.New()
	outCh := make(chan frames.Frame, 64)

	pl.Link(
		voice.NewSTTProcessor("stt", sttSvc, 16000, 1),
		voice.NewLLMProcessorWithSystemPrompt("llm", llmSvc, "You are a helpful voice assistant. Reply briefly and naturally to the user."),
		voice.NewTTSProcessor("tts", ttsSvc, 24000),
		pipeline.NewSink("sink", outCh),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pl.Setup(ctx); err != nil {
		t.Fatalf("pipeline setup failed: %v", err)
	}
	defer pl.Cleanup(ctx)

	if err := pl.Start(ctx, frames.NewStartFrame()); err != nil {
		t.Fatalf("pipeline start failed: %v", err)
	}

	wavData, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("reading audio fixture %s: %v", audioPath, err)
	}
	pcm, sampleRate, err := audio.DecodeWAVToPCM(wavData)
	if err != nil {
		t.Fatalf("decoding WAV: %v", err)
	}
	if sampleRate != 16000 {
		pcm = audio.Resample16MonoAlloc(pcm, sampleRate, 16000)
	}
	audioFrame := frames.NewAudioRawFrame(pcm, 16000, 1, 0)
	if err := pl.Push(ctx, audioFrame); err != nil {
		t.Fatalf("pushing AudioRawFrame: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for TTSAudioRawFrame from Sarvam+Groq voice pipeline")
		case f := <-outCh:
			switch v := f.(type) {
			case *frames.TTSAudioRawFrame:
				if len(v.Audio) == 0 {
					t.Fatal("received TTSAudioRawFrame with empty audio payload")
				}
				if v.SampleRate <= 0 {
					t.Fatalf("invalid sample rate on TTSAudioRawFrame: %d", v.SampleRate)
				}
				outPath := filepath.Join("tests", "testdata", "webrtc_sarvam_groq_tts_output.wav")
				if err := saveTTSAudioAsWAV(outPath, v.Audio, v.SampleRate, v.NumChannels); err != nil {
					t.Logf("failed to save TTS output to %s: %v", outPath, err)
				}
				return
			case *frames.ErrorFrame:
				t.Fatalf("received ErrorFrame from processor %q: %s", v.Processor, v.Error)
			default:
				_ = fmt.Sprintf("%T", f)
			}
		}
	}
}

// TestWebRTCSignaling_SarvamGroq starts the HTTP server with transport=both and Sarvam+Groq config,
// then POSTs a valid WebRTC offer to /webrtc/offer and asserts a 200 response with an SDP answer.
func TestWebRTCSignaling_SarvamGroq(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WebRTC signaling test in short mode")
	}

	configPaths := []string{"config.json", filepath.Join("..", "..", "config.json")}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			configPaths = append(configPaths, filepath.Join(dir, "config.json"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var cfg *config.Config
	var err error
	for _, p := range configPaths {
		cfg, err = config.LoadConfig(p)
		if err == nil {
			break
		}
	}
	if cfg == nil {
		t.Skipf("config.json not available: %v", err)
	}

	cfg.Transport = "both"
	cfg.Port = 18080
	cfg.SttProvider = services.ProviderSarvam
	cfg.LlmProvider = services.ProviderGroq
	cfg.TtsProvider = services.ProviderSarvam
	if cfg.Model == "" {
		cfg.Model = "llama-3.1-8b-instant"
	}
	if cfg.STTModel == "" {
		cfg.STTModel = "saarika:v2.5"
	}
	if cfg.TTSModel == "" {
		cfg.TTSModel = "bulbul:v2"
	}
	sarvamKey := cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	groqKey := cfg.GetAPIKey("groq", "GROQ_API_KEY")
	if sarvamKey == "" || groqKey == "" {
		t.Skip("SARVAM_API_KEY and GROQ_API_KEY required")
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{}
	}
	cfg.APIKeys["sarvam"] = sarvamKey
	cfg.APIKeys["groq"] = groqKey

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	onTransport := func(ctx context.Context, tr transport.Transport) {
		llm, stt, tts := services.NewServicesFromConfig(cfg)
		pl := pipeline.New()
		pl.Add(voice.NewSTTProcessor("stt", stt, 16000, 1))
		pl.Add(voice.NewLLMProcessorWithSystemPrompt("llm", llm, "You are a helpful assistant. Reply briefly."))
		pl.Add(voice.NewTTSProcessor("tts", tts, 24000))
		pl.Add(pipeline.NewSink("sink", tr.Output()))
		runner := pipeline.NewRunner(pl, tr, nil)
		go func() { _ = runner.Run(ctx) }()
	}

	done := make(chan struct{})
	go func() {
		_ = server.StartServers(ctx, cfg, onTransport, nil, nil, nil)
		close(done)
	}()
	time.Sleep(1 * time.Second)

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		cancel()
		t.Fatalf("new peer connection: %v", err)
	}
	defer pc.Close()

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		cancel()
		t.Fatalf("create offer: %v", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		cancel()
		t.Fatalf("set local description: %v", err)
	}

	offerBytes, _ := json.Marshal(offer)
	reqBody := []byte(fmt.Sprintf(`{"offer":%q}`, string(offerBytes)))
	resp, err := http.Post("http://localhost:18080/webrtc/offer", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		cancel()
		<-done
		t.Skipf("POST /webrtc/offer failed (is server running?): %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		<-done
		t.Fatalf("POST /webrtc/offer: status %d", resp.StatusCode)
	}

	var result struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cancel()
		<-done
		t.Fatalf("decode answer: %v", err)
	}
	if result.Answer == "" {
		t.Fatal("empty answer in response")
	}

	cancel()
	<-done
}
