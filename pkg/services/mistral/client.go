// Package mistral provides Mistral AI-backed LLM via OpenAI-compatible API.
package mistral

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

const mistralBaseURL = "https://api.mistral.ai/v1"

// NewClient returns an OpenAI-compatible client configured for Mistral.
// If apiKey is empty, config.GetEnv("MISTRAL_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("MISTRAL_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = mistralBaseURL
	return openai.NewClientWithConfig(cfg)
}
