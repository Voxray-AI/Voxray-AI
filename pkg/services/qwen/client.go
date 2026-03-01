// Package qwen provides Alibaba DashScope Qwen LLM via OpenAI-compatible API.
package qwen

import (
	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
)

const defaultQwenBaseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"

// NewClient returns an OpenAI-compatible client configured for Qwen (DashScope).
// If apiKey is empty, config.GetEnv("DASHSCOPE_API_KEY", "") or config.GetEnv("QWEN_API_KEY", "") is used.
// Base URL is config.GetEnv("DASHSCOPE_BASE_URL", defaultQwenBaseURL).
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("DASHSCOPE_API_KEY", "")
	}
	if apiKey == "" {
		apiKey = config.GetEnv("QWEN_API_KEY", "")
	}
	baseURL := config.GetEnv("DASHSCOPE_BASE_URL", defaultQwenBaseURL)
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return openai.NewClientWithConfig(cfg)
}
