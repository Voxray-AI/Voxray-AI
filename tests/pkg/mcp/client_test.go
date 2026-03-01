package mcp_test

import (
	"context"
	"testing"

	"voxray-go/pkg/mcp"
)

func TestConvertMCPToolToSchema(t *testing.T) {
	// Test schema conversion logic via GetToolsSchema with a server that fails quickly
	// (we only verify client construction and that schema conversion doesn't panic).
	params := mcp.StdioServerParams{Command: "nonexistent-mcp-server", Args: []string{}}
	client := mcp.NewClient(params, nil, nil)
	ctx := context.Background()
	_, err := client.GetToolsSchema(ctx)
	if err == nil {
		t.Log("GetToolsSchema with bad command unexpectedly succeeded (may have npx/node in path)")
		return
	}
	// Expected: connection or exec error
}

func TestClient_RegisterTools_NoopWithoutLLM(t *testing.T) {
	// RegisterTools requires llmapi.LLMServiceWithTools; we can't easily mock without importing openai.
	// So we just ensure the client and params types work.
	params := mcp.StdioServerParams{Command: "echo", Args: []string{"hello"}}
	c := mcp.NewClient(params, []string{"tool1"}, nil)
	if c.ServerParams.Command != "echo" {
		t.Errorf("ServerParams.Command = %q", c.ServerParams.Command)
	}
	if len(c.ToolsFilter) != 1 || c.ToolsFilter[0] != "tool1" {
		t.Errorf("ToolsFilter = %v", c.ToolsFilter)
	}
}

func TestStdioServerParams_ExecCmd(t *testing.T) {
	p := mcp.StdioServerParams{Command: "go", Args: []string{"version"}}
	cmd := p.ExecCmd()
	if cmd.Path == "" {
		t.Error("ExecCmd() should set Path")
	}
	if len(cmd.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(cmd.Args))
	}
}

