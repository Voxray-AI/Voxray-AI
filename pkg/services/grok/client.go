// Package grok provides xAI Grok-backed LLM via OpenAI-compatible API.
package grok

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

const grokBaseURL = "https://api.x.ai/v1"

// NewClient returns an OpenAI-compatible client configured for xAI Grok.
// If apiKey is empty, config.GetEnv("XAI_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("XAI_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = grokBaseURL
	return openai.NewClientWithConfig(cfg)
}
