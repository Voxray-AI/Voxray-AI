// Package groq provides Groq-backed LLM, STT, and TTS via OpenAI-compatible API.
package groq

import (
	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
)

const groqBaseURL = "https://api.groq.com/openai/v1"

// NewClient returns an OpenAI-compatible client configured for Groq.
// If apiKey is empty, config.GetEnv("GROQ_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("GROQ_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = groqBaseURL
	return openai.NewClientWithConfig(cfg)
}
