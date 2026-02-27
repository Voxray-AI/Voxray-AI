// Package openai provides OpenAI-based LLM (and optionally STT/TTS) for Voila.
package openai

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
)

// Service implements services.LLMService using OpenAI Chat Completions.
type Service struct {
	client *openai.Client
	model  string
}

// NewService creates an OpenAI LLM service. API key is read from config.GetEnv("OPENAI_API_KEY", "").
func NewService(apiKey, model string) *Service {
	if apiKey == "" {
		apiKey = config.GetEnv("OPENAI_API_KEY", "")
	}
	if model == "" {
		model = openai.GPT3Dot5Turbo
	}
	client := openai.NewClient(apiKey)
	return &Service{client: client, model: model}
}

// Chat runs a completion and calls onToken for each streamed content delta (as LLMTextFrame).
func (s *Service) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	reqMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		role := openai.ChatMessageRoleUser
		if r, ok := m["role"].(string); ok {
			role = r
		}
		content := ""
		if c, ok := m["content"].(string); ok {
			content = c
		}
		reqMessages = append(reqMessages, openai.ChatCompletionMessage{Role: role, Content: content})
	}

	stream, err := s.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: reqMessages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if len(response.Choices) == 0 {
			continue
		}
		delta := response.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		tf := &frames.LLMTextFrame{}
		tf.TextFrame = frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: delta, AppendToContext: true}
		tf.IncludesInterFrameSpace = true
		if onToken != nil {
			onToken(tf)
		}
	}
	return nil
}
