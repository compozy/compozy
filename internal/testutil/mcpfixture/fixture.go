package mcpfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	echoToolName    = "echo"
	statusToolName  = "status"
	confirmToolName = "confirm"
	messageField    = "message"

	// ModernProtocolVersion is the latest MCP revision exercised by the fixture.
	ModernProtocolVersion = "2026-07-28"
	// LegacyProtocolVersion is the legacy revision used after negotiation fallback.
	LegacyProtocolVersion = "2025-11-25"
	// ToolListTTLMs is the deterministic cache hint returned by modern tools/list.
	ToolListTTLMs = 60_000
)

// Fixture exposes one deterministic official Go SDK MCP server profile.
type Fixture struct {
	profile Profile
	server  *mcp.Server
}

// New constructs a deterministic server for profile.
func New(profile Profile) (*Fixture, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "compozy-mcp-fixture",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		PageSize: 1,
		Capabilities: &mcp.ServerCapabilities{
			Tools: &mcp.ToolCapabilities{},
		},
	})
	server.AddReceivingMiddleware(cacheableToolListMiddleware())
	addCommonTools(server)
	if profile == ProfileInputRequired {
		addInputRequiredTool(server)
	}

	return &Fixture{profile: profile, server: server}, nil
}

// MustNew constructs a fixture and panics only for a programmer-supplied invalid profile.
func MustNew(profile Profile) *Fixture {
	fixture, err := New(profile)
	if err != nil {
		panic(err)
	}
	return fixture
}

// Profile reports the fixture's interoperability profile.
func (f *Fixture) Profile() Profile {
	return f.profile
}

// Server returns the official Go SDK server used by this fixture.
func (f *Fixture) Server() *mcp.Server {
	return f.server
}

// RunStdio serves newline-delimited MCP JSON over input and output until ctx ends.
func (f *Fixture) RunStdio(ctx context.Context, input io.ReadCloser, output io.WriteCloser) error {
	if f == nil || f.server == nil {
		return fmt.Errorf("mcp fixture: run stdio with nil fixture")
	}
	if input == nil {
		return fmt.Errorf("mcp fixture: stdio input is required")
	}
	if output == nil {
		return fmt.Errorf("mcp fixture: stdio output is required")
	}
	return f.server.Run(ctx, &mcp.IOTransport{Reader: input, Writer: output})
}

func addCommonTools(server *mcp.Server) {
	server.AddTool(echoTool(), echoHandler)
	server.AddTool(statusTool(), statusHandler)
}

func echoTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        echoToolName,
		Description: "Returns the supplied message.",
		InputSchema: json.RawMessage(
			`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`,
		),
		OutputSchema: json.RawMessage(
			`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`,
		),
	}
}

func statusTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        statusToolName,
		Description: "Returns deterministic fixture status.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
		OutputSchema: json.RawMessage(
			`{"type":"object","properties":{"status":{"type":"string"}},"required":["status"],"additionalProperties":false}`,
		),
	}
}

func echoHandler(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(request.Params.Arguments, &input); err != nil {
		return nil, fmt.Errorf("mcp fixture: decode echo input: %w", err)
	}
	return structuredToolResult(map[string]string{messageField: input.Message}, echoToolName+": "+input.Message), nil
}

func statusHandler(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return structuredToolResult(map[string]string{"status": "ready"}, "status: ready"), nil
}

func structuredToolResult(value any, text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: value,
	}
}

func addInputRequiredTool(server *mcp.Server) {
	server.AddTool(&mcp.Tool{
		Name:        confirmToolName,
		Description: "Requests one deterministic confirmation input.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
	}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if len(request.Params.InputResponses) == 0 {
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{
					confirmToolName: &mcp.ElicitParams{Message: "Confirm fixture action."},
				},
				RequestState: "fixture-confirmation",
			}, nil
		}
		return structuredToolResult(map[string]bool{"confirmed": true}, "confirmed"), nil
	})
}

func cacheableToolListMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != "tools/list" {
				return result, err
			}
			toolList, ok := result.(*mcp.ListToolsResult)
			if !ok {
				return result, nil
			}
			toolList.TTLMs = ToolListTTLMs
			toolList.CacheScope = "private"
			return toolList, nil
		}
	}
}
