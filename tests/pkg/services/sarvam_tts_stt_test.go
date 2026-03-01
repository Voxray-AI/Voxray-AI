package services_test

import (
	"context"
	"os"
	"testing"

	"voxray-go/pkg/config"
	"voxray-go/pkg/services"
)

// TestSarvamServices_FromConfig wires Sarvam STT/TTS via the shared factory.
// It is skipped automatically when SARVAM_API_KEY and a minimal config.json
// for Sarvam are not available.
func TestSarvamServices_FromConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Sarvam integration test in short mode")
	}
	if os.Getenv("SARVAM_API_KEY") == "" {
		t.Skip("SARVAM_API_KEY not set; skipping Sarvam integration test")
	}

	cfg := &config.Config{
		Provider:   services.ProviderSarvam,
		SttProvider: services.ProviderSarvam,
		TtsProvider: services.ProviderSarvam,
		STTModel:   "",
		TTSModel:   "",
		TTSVoice:   "",
	}

	llm, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)
	if llm == nil {
		t.Fatalf("expected non-nil LLM service")
	}
	if sttSvc == nil {
		t.Fatalf("expected non-nil STT service")
	}
	if ttsSvc == nil {
		t.Fatalf("expected non-nil TTS service")
	}

	ctx := context.Background()

	// Basic TTS smoke test (empty text is a no-op).
	if _, err := ttsSvc.Speak(ctx, "hello from Sarvam", 0); err != nil {
		t.Fatalf("Sarvam TTS Speak failed: %v", err)
	}
}

