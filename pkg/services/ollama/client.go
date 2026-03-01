// Package ollama provides Ollama-backed LLM via OpenAI-compatible API (localhost or custom base URL).
package ollama

import (
	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
)

const defaultOllamaBaseURL = "http://localhost:11434/v1"

// ollamaPlaceholderKey is used when no API key is set; Ollama does not require auth.
const ollamaPlaceholderKey = "ollama"

// NewClient returns an OpenAI-compatible client configured for Ollama.
// If apiKey is empty, ollamaPlaceholderKey is used (Ollama does not require auth).
// Base URL is config.GetEnv("OLLAMA_BASE_URL", "http://localhost:11434/v1").
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("OLLAMA_API_KEY", ollamaPlaceholderKey)
	}
	if apiKey == "" {
		apiKey = ollamaPlaceholderKey
	}
	baseURL := config.GetEnv("OLLAMA_BASE_URL", defaultOllamaBaseURL)
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return openai.NewClientWithConfig(cfg)
}
