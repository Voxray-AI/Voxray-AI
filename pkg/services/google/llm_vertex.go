package google

import (
	"context"

	"voila-go/pkg/frames"

	"google.golang.org/genai"
)

// DefaultVertexLLMModel is the default model for Vertex AI when none is specified.
const DefaultVertexLLMModel = "gemini-2.5-flash"

// VertexLLMService implements services.LLMService using Google Vertex AI (genai SDK with BackendVertexAI).
// It uses Application Default Credentials; no API key is required.
type VertexLLMService struct {
	client *genai.Client
	model  string
}

// NewVertexLLMService creates a Vertex AI LLM service.
// project and location can be empty to use GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION env vars.
// If model is empty, DefaultVertexLLMModel is used.
func NewVertexLLMService(ctx context.Context, project, location, model string) (*VertexLLMService, error) {
	if model == "" {
		model = DefaultVertexLLMModel
	}
	client, err := NewVertexClient(ctx, project, location)
	if err != nil {
		return nil, err
	}
	return &VertexLLMService{client: client, model: model}, nil
}

// Chat runs a completion and calls onToken for each streamed content delta (as LLMTextFrame).
func (s *VertexLLMService) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
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
			DataFrame:               frames.DataFrame{Base: frames.NewBase()},
			Text:                    delta,
			AppendToContext:         true,
			IncludesInterFrameSpace: true,
		}
		if onToken != nil {
			onToken(tf)
		}
	}
	return nil
}
