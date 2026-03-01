// Package mcp: Client connects to an MCP server, lists tools, converts schemas,
// and can register them with an LLMServiceWithTools.
package mcp

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"voila-go/pkg/adapters/schemas"
	"voila-go/pkg/logger"
	"voila-go/pkg/services/llmapi"
)

// Client is an MCP client that discovers tools from an MCP server and can register
// them with an LLM (Option A: LLMServiceWithTools).
type Client struct {
	ServerParams      StdioServerParams
	ToolsFilter        []string                    // if non-nil, only these tool names are registered
	ToolsOutputFilters map[string]func(any) any    // optional per-tool output transform
	mu                sync.Mutex
}

// NewClient returns an MCP client for the given stdio server params.
func NewClient(params StdioServerParams, toolsFilter []string, outputFilters map[string]func(any) any) *Client {
	return &Client{
		ServerParams:      params,
		ToolsFilter:       toolsFilter,
		ToolsOutputFilters: outputFilters,
	}
}

// GetToolsSchema connects to the MCP server, lists tools, and returns their schema in Pipecat format.
func (c *Client) GetToolsSchema(ctx context.Context) (*schemas.ToolsSchema, error) {
	transport := &mcp.CommandTransport{Command: c.ServerParams.ExecCmd()}
	client := mcp.NewClient(&mcp.Implementation{Name: "voila-mcp-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	listRes, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	var standardTools []schemas.FunctionSchema
	for _, tool := range listRes.Tools {
		if tool == nil {
			continue
		}
		if c.ToolsFilter != nil && !contains(c.ToolsFilter, tool.Name) {
			logger.Debug("mcp: skipping tool %q (not in filter)", tool.Name)
			continue
		}
		fs, err := convertMCPToolToSchema(tool)
		if err != nil {
			logger.Error("mcp: convert tool %q: %v", tool.Name, err)
			continue
		}
		standardTools = append(standardTools, fs)
	}
	return &schemas.ToolsSchema{StandardTools: standardTools}, nil
}

// RegisterTools gets the tools schema from the MCP server and registers each tool
// with the LLM. llm must implement llmapi.LLMServiceWithTools.
func (c *Client) RegisterTools(ctx context.Context, llm llmapi.LLMServiceWithTools) (*schemas.ToolsSchema, error) {
	toolsSchema, err := c.GetToolsSchema(ctx)
	if err != nil {
		return nil, err
	}
	for _, fs := range toolsSchema.StandardTools {
		name := fs.Name
		handler := c.toolWrapper(name)
		llm.RegisterTool(fs, handler)
	}
	return toolsSchema, nil
}

// toolWrapper returns a ToolHandler that calls the MCP server's tool and optionally applies an output filter.
func (c *Client) toolWrapper(toolName string) llmapi.ToolHandler {
	return func(ctx context.Context, name string, arguments map[string]any) (string, error) {
		transport := &mcp.CommandTransport{Command: c.ServerParams.ExecCmd()}
		client := mcp.NewClient(&mcp.Implementation{Name: "voila-mcp-client", Version: "1.0"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			return "error: " + err.Error(), nil
		}
		defer session.Close()

		params := &mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		}
		res, err := session.CallTool(ctx, params)
		if err != nil {
			return "error: " + err.Error(), nil
		}
		response := extractTextFromResult(res)
		if c.ToolsOutputFilters != nil {
			if fn := c.ToolsOutputFilters[name]; fn != nil {
				response = toString(fn(response))
			}
		}
		if response == "" {
			response = "Sorry, could not call the mcp tool"
		}
		return response, nil
	}
}

func extractTextFromResult(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	var s string
	for _, content := range res.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			s += tc.Text
		}
	}
	return s
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	bs, _ := json.Marshal(v)
	return string(bs)
}

func convertMCPToolToSchema(tool *mcp.Tool) (schemas.FunctionSchema, error) {
	params := make(map[string]any)
	params["type"] = "object"
	if tool.InputSchema != nil {
		if m, ok := tool.InputSchema.(map[string]any); ok {
			if p, has := m["properties"]; has {
				params["properties"] = p
			}
			if r, has := m["required"]; has {
				params["required"] = r
			}
		}
	}
	if _, ok := params["properties"]; !ok {
		params["properties"] = map[string]any{}
	}
	return schemas.FunctionSchema{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  params,
	}, nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
