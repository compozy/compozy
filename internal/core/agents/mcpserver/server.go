package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	reservedToolName        = "run_agent"
	reservedToolDescription = "Run a reusable Compozy agent by name and return its structured result."
)

// Server serves the reserved Compozy MCP tool surface over stdio.
type Server struct {
	engine         *Engine
	implementation *mcp.Implementation
}

// ServerOption configures the stdio MCP server.
type ServerOption func(*Server)

// WithEngine overrides the nested-agent execution engine.
func WithEngine(engine *Engine) ServerOption {
	return func(server *Server) {
		if engine != nil {
			server.engine = engine
		}
	}
}

// WithImplementation overrides the advertised MCP implementation metadata.
func WithImplementation(impl *mcp.Implementation) ServerOption {
	return func(server *Server) {
		if impl != nil {
			server.implementation = impl
		}
	}
}

// NewServer constructs the reserved stdio MCP server.
func NewServer(opts ...ServerOption) *Server {
	server := &Server{
		engine: NewEngine(),
		implementation: &mcp.Implementation{
			Name:    reusableagents.ReservedMCPServerName,
			Version: version.Version,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(server)
		}
	}
	return server
}

// ServeStdio runs the reserved MCP server over stdin/stdout until the client disconnects.
func ServeStdio(ctx context.Context, host HostContext, opts ...ServerOption) error {
	return NewServer(opts...).RunStdio(ctx, host)
}

// RunStdio runs the reserved MCP server over stdin/stdout until the client disconnects.
func (s *Server) RunStdio(ctx context.Context, host HostContext) error {
	server := mcp.NewServer(s.impl(), nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        reservedToolName,
		Description: reservedToolDescription,
	}, s.runAgentTool(host))
	return server.Run(ctx, &mcp.StdioTransport{})
}

// LoadHostContextFromEnv loads the host-owned reserved-server runtime payload from
// COMPOZY_RUN_AGENT_CONTEXT. When the variable is unset, it returns a zero-value
// HostContext so the internal server can still be exercised in tests.
func LoadHostContextFromEnv() (HostContext, error) {
	return loadHostContextFromEnv(os.LookupEnv)
}

func (s *Server) impl() *mcp.Implementation {
	if s != nil && s.implementation != nil {
		return s.implementation
	}
	return &mcp.Implementation{
		Name:    reusableagents.ReservedMCPServerName,
		Version: version.Version,
	}
}

func (s *Server) engineOrDefault() *Engine {
	if s != nil && s.engine != nil {
		return s.engine
	}
	return NewEngine()
}

func (s *Server) runAgentTool(host HostContext) mcp.ToolHandlerFor[RunAgentRequest, RunAgentResult] {
	engine := s.engineOrDefault()
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input RunAgentRequest,
	) (*mcp.CallToolResult, RunAgentResult, error) {
		result := engine.RunAgent(ctx, host, input)
		if result.Success {
			return nil, result, nil
		}

		toolResult := &mcp.CallToolResult{}
		toolResult.SetError(fmt.Errorf("%s", strings.TrimSpace(result.Error)))
		return toolResult, result, nil
	}
}

func loadHostContextFromEnv(lookupEnv func(string) (string, bool)) (HostContext, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	raw, ok := lookupEnv(reusableagents.RunAgentContextEnvVar)
	if !ok || strings.TrimSpace(raw) == "" {
		return HostContext{}, nil
	}

	var payload reusableagents.ReservedServerRuntimeContext
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return HostContext{}, fmt.Errorf("decode %s: %w", reusableagents.RunAgentContextEnvVar, err)
	}

	return HostContext{
		BaseRuntime: payload.BaseRuntime,
		Nested:      payload.Nested,
	}, nil
}
