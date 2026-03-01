// Package mcp provides an MCP (Model Context Protocol) client for integrating
// external tools with LLMs (mirrors upstream pipecat mcp_service.py).
package mcp

import (
	"os/exec"
)

// StdioServerParams configures an MCP server that is launched as a subprocess
// and communicates over stdin/stdout (newline-delimited JSON).
type StdioServerParams struct {
	Command string   // executable name or path
	Args    []string // arguments (e.g. []string{"run", "server.go"})
}

// ExecCmd returns an exec.Cmd that runs the MCP server. The command is not started.
func (p *StdioServerParams) ExecCmd() *exec.Cmd {
	if len(p.Args) == 0 {
		return exec.Command(p.Command)
	}
	return exec.Command(p.Command, p.Args...)
}
