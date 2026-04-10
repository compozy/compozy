package model

import "maps"

// MCPServer describes one resolved MCP server attachment for an ACP session.
// V1 only needs stdio transport because reusable-agent `mcp.json` currently
// normalizes to command/args/env declarations.
type MCPServer struct {
	Stdio *MCPServerStdio
}

// MCPServerStdio describes one stdio-backed MCP server attachment.
type MCPServerStdio struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// CloneMCPServers returns a deep copy of the provided MCP server slice.
func CloneMCPServers(src []MCPServer) []MCPServer {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]MCPServer, 0, len(src))
	for idx := range src {
		item := src[idx]
		if item.Stdio == nil {
			cloned = append(cloned, MCPServer{})
			continue
		}
		stdio := &MCPServerStdio{
			Name:    item.Stdio.Name,
			Command: item.Stdio.Command,
			Args:    append([]string(nil), item.Stdio.Args...),
			Env:     maps.Clone(item.Stdio.Env),
		}
		cloned = append(cloned, MCPServer{Stdio: stdio})
	}
	return cloned
}
