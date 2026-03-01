// Package openai provides OpenAI-based LLM (and optionally STT/TTS) for Voila.
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	openai "github.com/sashabaranov/go-openai"
	"voila-go/pkg/adapters/schemas"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/services/llmapi"
)

// Service implements services.LLMService and optionally services.LLMServiceWithTools.
type Service struct {
	client *openai.Client
	model  string

	mu             sync.Mutex
	toolsSchema    []schemas.FunctionSchema
	toolsHandlers  map[string]llmapi.ToolHandler
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
	return &Service{client: client, model: model, toolsHandlers: make(map[string]llmapi.ToolHandler)}
}

// RegisterTool implements llmapi.LLMServiceWithTools.
func (s *Service) RegisterTool(schema schemas.FunctionSchema, handler llmapi.ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolsSchema = append(s.toolsSchema, schema)
	s.toolsHandlers[schema.Name] = handler
}

// ToolsSchema implements llmapi.LLMServiceWithTools.
func (s *Service) ToolsSchema() *schemas.ToolsSchema {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]schemas.FunctionSchema, len(s.toolsSchema))
	copy(copied, s.toolsSchema)
	return &schemas.ToolsSchema{StandardTools: copied}
}

// Chat runs a completion and calls onToken for each streamed content delta (as LLMTextFrame).
// When tools are registered, tool_calls in the stream are executed and Chat is invoked again with the tool results.
func (s *Service) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	reqMessages, err := s.buildMessages(messages)
	if err != nil {
		return err
	}

	s.mu.Lock()
	tools := s.buildOpenAIToolsLocked()
	s.mu.Unlock()

	req := openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: reqMessages,
		Stream:   true,
	}
	if len(tools) > 0 {
		req.Tools = tools
		req.ToolChoice = "auto"
	}

	stream, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	var contentBuf string
	toolCallsAccum := make(map[int]struct {
		ID       string
		Name     string
		Arguments string
	})

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
		choice := response.Choices[0]
		delta := choice.Delta

		if delta.Content != "" {
			contentBuf += delta.Content
			tf := &frames.LLMTextFrame{}
			tf.TextFrame = frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: delta.Content, AppendToContext: true}
			tf.IncludesInterFrameSpace = true
			if onToken != nil {
				onToken(tf)
			}
		}
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			acc := toolCallsAccum[idx]
			acc.ID = tc.ID
			acc.Name = tc.Function.Name
			acc.Arguments += tc.Function.Arguments
			toolCallsAccum[idx] = acc
		}

		if choice.FinishReason == openai.FinishReasonToolCalls {
			// Execute tool calls in index order and recurse
			maxIdx := -1
			for i := range toolCallsAccum {
				if i > maxIdx {
					maxIdx = i
				}
			}
			if maxIdx < 0 {
				break
			}
			newMessages := make([]map[string]any, 0, len(messages)+2+maxIdx+1)
			newMessages = append(newMessages, messages...)
			var toolCallsList []map[string]any
			for i := 0; i <= maxIdx; i++ {
				acc := toolCallsAccum[i]
				if acc.Name == "" {
					continue
				}
				toolCallsList = append(toolCallsList, map[string]any{
					"id":   acc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      acc.Name,
						"arguments": acc.Arguments,
					},
				})
			}
			assistantMsg := map[string]any{
				"role":       "assistant",
				"content":   contentBuf,
				"tool_calls": toolCallsList,
			}
			newMessages = append(newMessages, assistantMsg)

			s.mu.Lock()
			handlers := s.toolsHandlers
			s.mu.Unlock()
			for i := 0; i <= maxIdx; i++ {
				acc := toolCallsAccum[i]
				if acc.Name == "" {
					continue
				}
				handler := handlers[acc.Name]
				var args map[string]any
				if acc.Arguments != "" {
					_ = json.Unmarshal([]byte(acc.Arguments), &args)
				}
				if args == nil {
					args = make(map[string]any)
				}
				result := ""
				if handler != nil {
					var errTool error
					result, errTool = handler(ctx, acc.Name, args)
					if errTool != nil {
						result = "error: " + errTool.Error()
					}
				} else {
					result = "tool not found"
				}
				newMessages = append(newMessages, map[string]any{
					"role":         "tool",
					"tool_call_id": acc.ID,
					"content":      result,
				})
			}
			return s.Chat(ctx, newMessages, onToken)
		}
	}
	return nil
}

func (s *Service) buildMessages(messages []map[string]any) ([]openai.ChatCompletionMessage, error) {
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
		msg := openai.ChatCompletionMessage{Role: role, Content: content}
		if role == openai.ChatMessageRoleAssistant {
			if tc, ok := m["tool_calls"].([]map[string]any); ok {
				for _, t := range tc {
					id, _ := t["id"].(string)
					fn, _ := t["function"].(map[string]any)
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
						ID:   id,
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      name,
							Arguments: args,
						},
					})
				}
			}
		}
		if role == "tool" {
			if id, ok := m["tool_call_id"].(string); ok {
				msg.ToolCallID = id
			}
		}
		reqMessages = append(reqMessages, msg)
	}
	return reqMessages, nil
}

func (s *Service) buildOpenAIToolsLocked() []openai.Tool {
	if len(s.toolsSchema) == 0 {
		return nil
	}
	tools := make([]openai.Tool, 0, len(s.toolsSchema))
	for _, schema := range s.toolsSchema {
		params := schema.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        schema.Name,
				Description: schema.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}
