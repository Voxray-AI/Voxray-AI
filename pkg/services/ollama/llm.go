package ollama

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
)

// DefaultLLMModel is the default Ollama model when none is specified.
const DefaultLLMModel = "llama3.2"

// LLMService implements services.LLMService using Ollama (OpenAI-compatible).
type LLMService struct {
	client *openai.Client
	model  string
}

// NewLLMService creates an Ollama LLM service.
// If apiKey is empty, config.GetEnv("OLLAMA_API_KEY", "ollama") is used (placeholder is fine).
// If model is empty, DefaultLLMModel is used.
func NewLLMService(apiKey, model string) *LLMService {
	if apiKey == "" {
		apiKey = config.GetEnv("OLLAMA_API_KEY", ollamaPlaceholderKey)
	}
	if apiKey == "" {
		apiKey = ollamaPlaceholderKey
	}
	if model == "" {
		model = DefaultLLMModel
	}
	return &LLMService{client: NewClient(apiKey), model: model}
}

// Chat runs a completion and calls onToken for each streamed content delta (as LLMTextFrame).
func (s *LLMService) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
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
