// Package deepseek provides DeepSeek-backed LLM via OpenAI-compatible API.
package deepseek

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

const deepseekBaseURL = "https://api.deepseek.com/v1"

// NewClient returns an OpenAI-compatible client configured for DeepSeek.
// If apiKey is empty, config.GetEnv("DEEPSEEK_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("DEEPSEEK_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = deepseekBaseURL
	return openai.NewClientWithConfig(cfg)
}
