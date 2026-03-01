package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"voxray-go/pkg/frames"
)

// DefaultBedrockModel is the default Bedrock model when none is specified.
const DefaultBedrockModel = "anthropic.claude-3-haiku-20240307-v1:0"

// LLMService implements services.LLMService using AWS Bedrock Converse API (streaming).
type LLMService struct {
	client  *bedrockruntime.Client
	modelID string
}

// NewLLM creates an AWS Bedrock LLM service from an existing AWS config.
func NewLLM(cfg aws.Config, modelID string) *LLMService {
	if modelID == "" {
		modelID = DefaultBedrockModel
	}
	return &LLMService{
		client:  bedrockruntime.NewFromConfig(cfg),
		modelID: modelID,
	}
}

// NewLLMWithRegion creates an AWS Bedrock LLM service by loading default config (env/profile) for the given region.
func NewLLMWithRegion(ctx context.Context, region, modelID string) (*LLMService, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws bedrock: load config: %w", err)
	}
	return NewLLM(cfg, modelID), nil
}

// Chat runs a completion via Bedrock ConverseStream and calls onToken for each content delta.
func (s *LLMService) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	converseMessages := make([]types.Message, 0, len(messages))
	for _, m := range messages {
		role := types.ConversationRoleUser
		if r, ok := m["role"].(string); ok {
			switch r {
			case "assistant":
				role = types.ConversationRoleAssistant
			case "system":
				role = types.ConversationRoleUser
			}
		}
		content := ""
		if c, ok := m["content"].(string); ok {
			content = c
		}
		converseMessages = append(converseMessages, types.Message{
			Role: role,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: content},
			},
		})
	}
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  aws.String(s.modelID),
		Messages: converseMessages,
	}
	output, err := s.client.ConverseStream(ctx, input)
	if err != nil {
		return err
	}
	defer output.GetStream().Close()
	for event := range output.GetStream().Events() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if delta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok && delta.Value != "" {
				tf := &frames.LLMTextFrame{}
				tf.TextFrame = frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: delta.Value, AppendToContext: true}
				tf.IncludesInterFrameSpace = true
				if onToken != nil {
					onToken(tf)
				}
			}
		}
	}
	return nil
}
