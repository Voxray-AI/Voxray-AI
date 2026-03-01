package google

import (
	"context"

	"voila-go/pkg/frames"

	"google.golang.org/genai"
)

// DefaultLLMModel is the default Gemini model when none is specified.
const DefaultLLMModel = "gemini-2.5-flash"

// LLMService implements services.LLMService using the Google Gemini API (native genai SDK).
type LLMService struct {
	client *genai.Client
	model  string
}

// NewLLMService creates a Google Gemini LLM service.
// If apiKey is empty, config env GOOGLE_API_KEY or GEMINI_API_KEY is used.
// If model is empty, DefaultLLMModel is used.
func NewLLMService(ctx context.Context, apiKey, model string) (*LLMService, error) {
	if model == "" {
		model = DefaultLLMModel
	}
	client, err := NewClient(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	return &LLMService{client: client, model: model}, nil
}

// Chat runs a completion and calls onToken for each streamed content delta (as LLMTextFrame).
func (s *LLMService) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	contents, systemInstruction := messagesToContents(messages)
	config := &genai.GenerateContentConfig{}
	if systemInstruction != "" {
		config.SystemInstruction = genai.NewContentFromText(systemInstruction, genai.RoleUser)
	}

	for result, err := range s.client.Models.GenerateContentStream(ctx, s.model, contents, config) {
		if err != nil {
			return err
		}
		delta := result.Text()
		if delta == "" {
			continue
		}
		tf := &frames.LLMTextFrame{}
		tf.TextFrame = frames.TextFrame{
			DataFrame:              frames.DataFrame{Base: frames.NewBase()},
			Text:                   delta,
			AppendToContext:        true,
			IncludesInterFrameSpace: true,
		}
		if onToken != nil {
			onToken(tf)
		}
	}
	return nil
}

// messagesToContents converts OpenAI-style messages (role/content) into genai Content slice.
// A system message is extracted and returned as the second return value for SystemInstruction.
func messagesToContents(messages []map[string]any) ([]*genai.Content, string) {
	var contents []*genai.Content
	var systemInstruction string
	for _, m := range messages {
		role := "user"
		if r, ok := m["role"].(string); ok && r != "" {
			role = r
		}
		content := ""
		if c, ok := m["content"].(string); ok {
			content = c
		}
		if role == "system" {
			if content != "" {
				systemInstruction = content
			}
			continue
		}
		var genaiRole genai.Role
		if role == "assistant" || role == "model" {
			genaiRole = genai.RoleModel
		} else {
			genaiRole = genai.RoleUser
		}
		contents = append(contents, genai.NewContentFromText(content, genaiRole))
	}
	return contents, systemInstruction
}
