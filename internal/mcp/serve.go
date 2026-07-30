package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/version"
	"github.com/google/uuid"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	hostAPIServerName          = "compozy-host-api"
	jsonNullLiteral            = "null"
	hostAPISessionCloseTimeout = 5 * time.Second
	hostedToolListCacheTTLMs   = 60_000
)

// HostAPIInvoker dispatches a workspace-bound Host API call through the running daemon.
type HostAPIInvoker interface {
	InvokeHostAPI(
		ctx context.Context,
		serveSessionID string,
		workspace string,
		method string,
		params json.RawMessage,
	) (json.RawMessage, error)
	CloseHostAPISession(ctx context.Context, serveSessionID string) error
}

// HostAPIInvokeRequest is the daemon-internal UDS relay contract for one projected call.
type HostAPIInvokeRequest struct {
	ServeSessionID string          `json:"serve_session_id"`
	Workspace      string          `json:"workspace"`
	Method         string          `json:"method"`
	Params         json.RawMessage `json:"params"`
}

// HostAPISessionCloseRequest identifies one MCP serve client lifecycle to release.
type HostAPISessionCloseRequest struct {
	ServeSessionID string `json:"serve_session_id"`
}

// HostAPIInvokeResponse carries the canonical Host API result without reshaping it.
type HostAPIInvokeResponse struct {
	Result json.RawMessage `json:"result"`
}

// ServeStdio exposes the projected Host API over MCP stdio.
func ServeStdio(
	ctx context.Context,
	invoker HostAPIInvoker,
	workspace string,
	stdin io.Reader,
	stdout io.Writer,
	_ io.Writer,
) (err error) {
	serveSessionID := uuid.NewString()
	mcpServer, err := newHostAPIMCPServer(invoker, serveSessionID, workspace)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeHostAPISession(invoker, serveSessionID))
	}()
	transport := protocolRestrictedTransport{Transport: &mcpgo.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{Writer: stdout}}}
	if err := mcpServer.Run(ctx, transport); !isExpectedStdioServerTermination(err) {
		return fmt.Errorf("mcp: serve stdio: %w", err)
	}
	return nil
}

func isExpectedStdioServerTermination(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, mcpgo.ErrConnectionClosed) {
		return true
	}
	// The SDK's JSON-RPC transport currently formats a normal stdin EOF as
	// "server is closing: EOF" without retaining EOF in its error chain.
	return strings.HasPrefix(err.Error(), "server is closing:") && strings.HasSuffix(err.Error(), ": EOF")
}

func newHostAPIMCPServer(
	invoker HostAPIInvoker,
	serveSessionID string,
	workspace string,
) (*mcpgo.Server, error) {
	if invoker == nil {
		return nil, errors.New("mcp: host api invoker is required")
	}
	serveSessionID = strings.TrimSpace(serveSessionID)
	if serveSessionID == "" {
		return nil, errors.New("mcp: serve session id is required")
	}
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil, errors.New("mcp: workspace is required")
	}
	if missing := HostAPIProjectionCoverage(); len(missing) > 0 {
		return nil, fmt.Errorf("mcp: host api projection decisions missing for %v", missing)
	}

	tools, err := projectedHostAPITools()
	if err != nil {
		return nil, err
	}
	mcpServer := mcpgo.NewServer(&mcpgo.Implementation{Name: hostAPIServerName, Version: version.Current().Version}, &mcpgo.ServerOptions{
		Logger:       slog.Default(),
		Capabilities: &mcpgo.ServerCapabilities{Tools: &mcpgo.ToolCapabilities{}},
	})
	mcpServer.AddReceivingMiddleware(privateToolListCacheMiddleware())
	mcpServer.AddReceivingMiddleware(protocolVersionMiddleware())
	for _, tool := range tools {
		method, ok := hostAPIMethodFromToolName(tool.Name)
		if !ok {
			return nil, fmt.Errorf("mcp: projected tool %q is not reversible", tool.Name)
		}
		mcpServer.AddTool(&tool, hostAPIToolHandler(invoker, serveSessionID, workspace, string(method)))
	}
	return mcpServer, nil
}

func privateToolListCacheMiddleware() mcpgo.Middleware {
	return func(next mcpgo.MethodHandler) mcpgo.MethodHandler {
		return func(ctx context.Context, method string, request mcpgo.Request) (mcpgo.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != "tools/list" {
				return result, err
			}
			if tools, ok := result.(*mcpgo.ListToolsResult); ok {
				tools.CacheScope = "private"
				tools.TTLMs = hostedToolListCacheTTLMs
			}
			return result, nil
		}
	}
}

func hostAPIToolHandler(
	invoker HostAPIInvoker,
	serveSessionID string,
	workspace string,
	method string,
) mcpgo.ToolHandler {
	return func(ctx context.Context, request *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		params := append(json.RawMessage(nil), request.Params.Arguments...)
		if len(params) == 0 {
			params = json.RawMessage("{}")
		}
		if string(params) == jsonNullLiteral {
			params = json.RawMessage("{}")
		}
		result, err := invoker.InvokeHostAPI(ctx, serveSessionID, workspace, method, params)
		if err != nil {
			return nil, err
		}

		var structured any
		if err := json.Unmarshal(result, &structured); err != nil {
			return nil, fmt.Errorf("mcp: decode %q result: %w", method, err)
		}
		return &mcpgo.CallToolResult{
			Content:           []mcpgo.Content{&mcpgo.TextContent{Text: string(result)}},
			StructuredContent: structured,
		}, nil
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func closeHostAPISession(invoker HostAPIInvoker, serveSessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), hostAPISessionCloseTimeout)
	defer cancel()
	if err := invoker.CloseHostAPISession(ctx, serveSessionID); err != nil {
		return fmt.Errorf("mcp: close host api session: %w", err)
	}
	return nil
}
