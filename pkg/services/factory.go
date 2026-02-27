// Package services defines interfaces and implementations for LLM, STT, and TTS.
package services

import (
	"voila-go/pkg/config"
	"voila-go/pkg/services/groq"
	"voila-go/pkg/services/openai"
	"voila-go/pkg/services/stt"
	"voila-go/pkg/services/tts"
)

const (
	ProviderOpenAI = "openai"
	ProviderGroq   = "groq"
)

// NewLLMFromConfig returns an LLMService for the given provider and model.
// Provider is "groq" or "openai" (default). API key comes from cfg.APIKeys, GROQ_API_KEY, or OPENAI_API_KEY.
func NewLLMFromConfig(cfg *config.Config, provider, model string) LLMService {
	if provider == ProviderGroq {
		return groq.NewLLMService(cfg.GetAPIKey("groq", "GROQ_API_KEY"), model)
	}
	return openai.NewService(cfg.GetAPIKey("openai", "OPENAI_API_KEY"), model)
}

// NewSTTFromConfig returns an STTService for the given provider.
func NewSTTFromConfig(cfg *config.Config, provider string) STTService {
	if provider == ProviderGroq {
		return stt.NewGroq(cfg.GetAPIKey("groq", "GROQ_API_KEY"))
	}
	return stt.NewOpenAI(cfg.GetAPIKey("openai", "OPENAI_API_KEY"))
}

// NewTTSFromConfig returns a TTSService for the given provider, model, and voice.
// For Groq, model/voice default to Orpheus and "alloy"; for OpenAI, to tts-1 and alloy.
func NewTTSFromConfig(cfg *config.Config, provider, model, voice string) TTSService {
	if provider == ProviderGroq {
		return tts.NewGroq(cfg.GetAPIKey("groq", "GROQ_API_KEY"), model, voice)
	}
	return tts.NewOpenAI(cfg.GetAPIKey("openai", "OPENAI_API_KEY"), model)
}

// NewServicesFromConfig returns LLM, STT, and TTS services based on cfg.
// Uses cfg.Provider ("groq" or "openai") and cfg.Model.
func NewServicesFromConfig(cfg *config.Config) (LLMService, STTService, TTSService) {
	provider := cfg.Provider
	if provider == "" {
		provider = ProviderOpenAI
	}
	model := cfg.Model
	if model == "" && provider == ProviderOpenAI {
		model = "gpt-3.5-turbo"
	}
	llm := NewLLMFromConfig(cfg, provider, model)
	sttSvc := NewSTTFromConfig(cfg, provider)
	ttsSvc := NewTTSFromConfig(cfg, provider, model, "")
	return llm, sttSvc, ttsSvc
}
