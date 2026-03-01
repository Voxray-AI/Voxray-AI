package services_test

import (
	"context"
	"testing"

	"voxray-go/pkg/config"
	"voxray-go/pkg/services"
)

// TestNewLLMFromConfig_ConstructsAllSupportedProviders verifies that the factory
// returns a non-nil LLMService for each supported LLM provider (no API calls).
func TestNewLLMFromConfig_ConstructsAllSupportedProviders(t *testing.T) {
	cfg := &config.Config{Model: "test-model"}
	ctx := context.Background()

	for _, provider := range services.SupportedLLMProviders {
		svc := services.NewLLMFromConfig(cfg, provider, "test-model")
		if svc == nil {
			t.Errorf("NewLLMFromConfig(%q) returned nil", provider)
			continue
		}
		// Optional: run a no-op to ensure the service is usable (Chat with empty messages may still hit API for some providers;
		// we only check construction here).
		_ = ctx
	}
}

// TestNewLLMFromConfig_MistralAndDeepSeek verifies Mistral and DeepSeek LLM construction.
func TestNewLLMFromConfig_MistralAndDeepSeek(t *testing.T) {
	cfg := &config.Config{Model: "mistral-small-latest"}
	if svc := services.NewLLMFromConfig(cfg, services.ProviderMistral, "mistral-small-latest"); svc == nil {
		t.Fatal("NewLLMFromConfig(mistral) returned nil")
	}

	cfg.Model = "deepseek-chat"
	if svc := services.NewLLMFromConfig(cfg, services.ProviderDeepSeek, "deepseek-chat"); svc == nil {
		t.Fatal("NewLLMFromConfig(deepseek) returned nil")
	}
}

// TestNewSTTFromConfig_ConstructsAllSupportedProviders verifies STT factory for each supported provider.
func TestNewSTTFromConfig_ConstructsAllSupportedProviders(t *testing.T) {
	cfg := &config.Config{}
	for _, provider := range services.SupportedSTTProviders {
		svc := services.NewSTTFromConfig(cfg, provider)
		if svc == nil {
			t.Errorf("NewSTTFromConfig(%q) returned nil", provider)
		}
	}
}

// TestNewTTSFromConfig_ConstructsAllSupportedProviders verifies TTS factory for each supported provider.
func TestNewTTSFromConfig_ConstructsAllSupportedProviders(t *testing.T) {
	cfg := &config.Config{}
	for _, provider := range services.SupportedTTSProviders {
		svc := services.NewTTSFromConfig(cfg, provider, "", "")
		if svc == nil {
			t.Errorf("NewTTSFromConfig(%q) returned nil", provider)
		}
	}
}
