// Package llmapi defines LLM and tool-calling interfaces so that implementers (e.g. openai)
// and consumers (e.g. mcp) can depend on it without import cycles with the full services package.
package llmapi

import (
	"context"

	"voxray-go/pkg/adapters/schemas"
	"voxray-go/pkg/frames"
)

// LLMService provides chat completion; may stream text frames.
type LLMService interface {
	Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error
}

// ToolHandler is called when the LLM requests a tool call. Returns the result string or error.
type ToolHandler func(ctx context.Context, toolName string, arguments map[string]any) (string, error)

// LLMServiceWithTools is an LLM service that supports registering tools (e.g. from MCP).
type LLMServiceWithTools interface {
	LLMService
	RegisterTool(schema schemas.FunctionSchema, handler ToolHandler)
	ToolsSchema() *schemas.ToolsSchema
}
