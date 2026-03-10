package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/services/httpclient"
)

// DefaultLLMModel is the default Anthropic model when none is specified.
const DefaultLLMModel = "claude-3-sonnet-20240229"

// Service implements a minimal Anthropic messages API client compatible with services.LLMService.
type Service struct {
	apiKey string
	model  string
	client *http.Client
}

// NewLLMService creates a new Anthropic LLM service.
// apiKey should be the Anthropic API key; model is the messages model (e.g. "claude-3-sonnet-20240229").
func NewLLMService(apiKey, model string) *Service {
	if model == "" {
		model = DefaultLLMModel
	}
	return &Service{
		apiKey: apiKey,
		model:  model,
		client: httpclient.Client(60 * time.Second),
	}
}

type anthropicMessage struct {
	Role    string                 `json:"role"`
	Content []anthropicTextBlock   `json:"content"`
	Meta    map[string]interface{} `json:"-"`
}

type anthropicTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	TopK        *int               `json:"top_k,omitempty"`
}

type anthropicResponse struct {
	ID      string               `json:"id"`
	Content []anthropicTextBlock `json:"content"`
}

// Chat runs a completion using Anthropic's messages API.
// For now this sends a single aggregated text frame with the full response
// (no incremental token streaming).
func (s *Service) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	if s.apiKey == "" {
		return fmt.Errorf("anthropic: missing API key")
	}

	var systemText string
	reqMessages := make([]anthropicMessage, 0, len(messages))

	for _, m := range messages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)

		switch role {
		case "system":
			if content != "" {
				if systemText != "" {
					systemText += "\n"
				}
				systemText += content
			}
			continue
		case "assistant":
			role = "assistant"
		default:
			// Treat anything else as user.
			role = "user"
		}

		if content == "" {
			continue
		}

		reqMessages = append(reqMessages, anthropicMessage{
			Role: role,
			Content: []anthropicTextBlock{
				{Type: "text", Text: content},
			},
		})
	}

	if len(reqMessages) == 0 && systemText == "" {
		return fmt.Errorf("anthropic: no messages provided")
	}

	body, err := json.Marshal(anthropicRequest{
		Model:     s.model,
		Messages:  reqMessages,
		System:    systemText,
		MaxTokens: 1024,
		Stream:    false,
	})
	if err != nil {
		return fmt.Errorf("anthropic: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("anthropic: unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return fmt.Errorf("anthropic: decode response: %w", err)
	}

	var combined string
	for _, c := range ar.Content {
		if c.Type != "text" || c.Text == "" {
			continue
		}
		combined += c.Text
	}
	if combined == "" || onToken == nil {
		return nil
	}

	tf := &frames.LLMTextFrame{}
	tf.TextFrame = frames.TextFrame{
		DataFrame:       frames.DataFrame{Base: frames.NewBase()},
		Text:            combined,
		AppendToContext: true,
	}
	tf.IncludesInterFrameSpace = true
	onToken(tf)

	return nil
}
