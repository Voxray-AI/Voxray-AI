// Package services defines interfaces and implementations for LLM, STT, and TTS.
// Use the factory functions (NewLLMFromConfig, NewSTTFromConfig, NewTTSFromConfig) to construct
// services by provider name; see Supported*Providers for capability matrix. For RealtimeService
// use realtime.NewFromConfig(cfg, provider) to avoid an import cycle.
package services

import (
	"context"

	"voila-go/pkg/config"
	"voila-go/pkg/services/anthropic"
	"voila-go/pkg/services/aws"
	"voila-go/pkg/services/cerebras"
	"voila-go/pkg/services/deepseek"
	"voila-go/pkg/services/elevenlabs"
	"voila-go/pkg/services/grok"
	"voila-go/pkg/services/groq"
	"voila-go/pkg/services/mistral"
	"voila-go/pkg/services/openai"
	"voila-go/pkg/services/sarvam"
	"voila-go/pkg/services/stt"
	"voila-go/pkg/services/tts"
)

const (
	ProviderOpenAI     = "openai"
	ProviderGroq       = "groq"
	ProviderSarvam     = "sarvam"
	ProviderGrok       = "grok"
	ProviderCerebras   = "cerebras"
	ProviderElevenLabs = "elevenlabs"
	ProviderAWS        = "aws"
	ProviderMistral    = "mistral"
	ProviderDeepSeek   = "deepseek"
	ProviderAnthropic  = "anthropic"
)

// SupportedLLMProviders lists provider keys that can be passed to NewLLMFromConfig.
var SupportedLLMProviders = []string{
	ProviderOpenAI,
	ProviderGroq,
	ProviderGrok,
	ProviderCerebras,
	ProviderAWS,
	ProviderMistral,
	ProviderDeepSeek,
	ProviderAnthropic,
}

// SupportedSTTProviders lists provider keys that can be passed to NewSTTFromConfig.
var SupportedSTTProviders = []string{ProviderOpenAI, ProviderGroq, ProviderSarvam, ProviderElevenLabs, ProviderAWS}

// SupportedTTSProviders lists provider keys that can be passed to NewTTSFromConfig.
var SupportedTTSProviders = []string{ProviderOpenAI, ProviderGroq, ProviderSarvam, ProviderElevenLabs, ProviderAWS}

// SupportedRealtimeProviders lists provider keys for realtime (use realtime.NewFromConfig to construct).
var SupportedRealtimeProviders = []string{ProviderOpenAI}

// apiKeyForProvider returns the API key for the given provider (e.g. openai -> OPENAI_API_KEY, groq -> GROQ_API_KEY).
func apiKeyForProvider(cfg *config.Config, provider string) string {
	switch provider {
	case ProviderAnthropic:
		return cfg.GetAPIKey("anthropic", "ANTHROPIC_API_KEY")
	case ProviderGroq:
		return cfg.GetAPIKey("groq", "GROQ_API_KEY")
	case ProviderSarvam:
		return cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	case ProviderGrok:
		return cfg.GetAPIKey("xai", "XAI_API_KEY")
	case ProviderCerebras:
		return cfg.GetAPIKey("cerebras", "CEREBRAS_API_KEY")
	case ProviderElevenLabs:
		return cfg.GetAPIKey("elevenlabs", "ELEVENLABS_API_KEY")
	case ProviderAWS:
		return cfg.GetAPIKey("aws", "AWS_SECRET_ACCESS_KEY")
	case ProviderMistral:
		return cfg.GetAPIKey("mistral", "MISTRAL_API_KEY")
	case ProviderDeepSeek:
		return cfg.GetAPIKey("deepseek", "DEEPSEEK_API_KEY")
	case ProviderOpenAI:
		return cfg.GetAPIKey("openai", "OPENAI_API_KEY")
	default:
		return cfg.GetAPIKey(provider, "OPENAI_API_KEY")
	}
}

func getAWSRegion(cfg *config.Config) string {
	r := cfg.GetAPIKey("aws_region", "AWS_REGION")
	if r == "" {
		return "us-east-1"
	}
	return r
}

// NewLLMFromConfig returns an LLMService for the given provider and model.
// Provider must be one of SupportedLLMProviders; model is the chat model (e.g. cfg.Model).
func NewLLMFromConfig(cfg *config.Config, provider, model string) LLMService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderAnthropic:
		return anthropic.NewLLMService(apiKey, model)
	case ProviderGroq:
		return groq.NewLLMService(apiKey, model)
	case ProviderGrok:
		return grok.NewLLMService(apiKey, model)
	case ProviderCerebras:
		return cerebras.NewLLMService(apiKey, model)
	case ProviderAWS:
		svc, err := aws.NewLLMWithRegion(context.Background(), getAWSRegion(cfg), model)
		if err != nil {
			return nil
		}
		return svc
	case ProviderMistral:
		return mistral.NewLLMService(apiKey, model)
	case ProviderDeepSeek:
		return deepseek.NewLLMService(apiKey, model)
	case ProviderOpenAI:
		fallthrough
	default:
		if model == "" {
			model = "gpt-3.5-turbo"
		}
		return openai.NewService(apiKey, model)
	}
}

// NewSTTFromConfig returns an STTService for the given provider.
// Provider must be one of SupportedSTTProviders; cfg.STTModel is used when supported (e.g. Groq).
func NewSTTFromConfig(cfg *config.Config, provider string) STTService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderGroq:
		return stt.NewGroqWithModel(apiKey, cfg.STTModel)
	case ProviderSarvam:
		return sarvam.NewSTTWithLanguage(apiKey, cfg.STTModel, cfg.STTLanguage)
	case ProviderElevenLabs:
		return elevenlabs.NewSTT(apiKey, cfg.STTModel)
	case ProviderAWS:
		svc, err := aws.NewSTTWithRegion(context.Background(), getAWSRegion(cfg), "")
		if err != nil {
			return nil
		}
		return svc
	case ProviderOpenAI:
		fallthrough
	default:
		return stt.NewOpenAI(apiKey)
	}
}

// NewTTSFromConfig returns a TTSService for the given provider, model, and voice.
// Provider must be one of SupportedTTSProviders; model and voice are typically cfg.TTSModel and cfg.TTSVoice.
func NewTTSFromConfig(cfg *config.Config, provider, model, voice string) TTSService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderGroq:
		return tts.NewGroq(apiKey, model, voice)
	case ProviderSarvam:
		return sarvam.NewTTS(apiKey, model, voice)
	case ProviderElevenLabs:
		return elevenlabs.NewTTS(apiKey, voice, model, "")
	case ProviderAWS:
		if voice == "" {
			voice = "Joanna"
		}
		svc, err := aws.NewTTSWithRegion(context.Background(), getAWSRegion(cfg), voice, "")
		if err != nil {
			return nil
		}
		return svc
	case ProviderOpenAI:
		fallthrough
	default:
		return tts.NewOpenAI(apiKey, model)
	}
}

// NewServicesFromConfig returns LLM, STT, and TTS services based on cfg.
// Resolves provider per task (stt_provider/llm_provider/tts_provider or provider); uses task-specific model/voice when set.
func NewServicesFromConfig(cfg *config.Config) (LLMService, STTService, TTSService) {
	sttProvider := cfg.STTProvider()
	if sttProvider == "" {
		sttProvider = ProviderOpenAI
	}
	llmProvider := cfg.LLMProvider()
	if llmProvider == "" {
		llmProvider = ProviderOpenAI
	}
	ttsProvider := cfg.TTSProvider()
	if ttsProvider == "" {
		ttsProvider = ProviderOpenAI
	}
	model := cfg.Model
	if model == "" && llmProvider == ProviderOpenAI {
		model = "gpt-3.5-turbo"
	}
	llm := NewLLMFromConfig(cfg, llmProvider, model)
	sttSvc := NewSTTFromConfig(cfg, sttProvider)
	ttsSvc := NewTTSFromConfig(cfg, ttsProvider, cfg.TTSModel, cfg.TTSVoice)
	return llm, sttSvc, ttsSvc
}
