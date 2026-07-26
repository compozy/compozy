package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/compozy/agh/internal/version"
	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	hostAPIServerName          = "agh-host-api"
	jsonNullLiteral            = "null"
	hostAPISessionCloseTimeout = 5 * time.Second
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
	stderr io.Writer,
) (err error) {
	serveSessionID := uuid.NewString()
	mcpServer, err := newHostAPIMCPServer(invoker, serveSessionID, workspace)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeHostAPISession(invoker, serveSessionID))
	}()
	stdio := server.NewStdioServer(mcpServer)
	stdio.SetErrorLogger(log.New(stderr, "", log.LstdFlags))
	if err := stdio.Listen(ctx, stdin, stdout); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("mcp: serve stdio: %w", err)
	}
	return nil
}

func newHostAPIMCPServer(
	invoker HostAPIInvoker,
	serveSessionID string,
	workspace string,
) (*server.MCPServer, error) {
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
	mcpServer := server.NewMCPServer(
		hostAPIServerName,
		version.Current().Version,
		server.WithToolCapabilities(false),
	)
	for _, tool := range tools {
		method, ok := hostAPIMethodFromToolName(tool.Name)
		if !ok {
			return nil, fmt.Errorf("mcp: projected tool %q is not reversible", tool.Name)
		}
		mcpServer.AddTool(tool, hostAPIToolHandler(invoker, serveSessionID, workspace, string(method)))
	}
	return mcpServer, nil
}

func hostAPIToolHandler(
	invoker HostAPIInvoker,
	serveSessionID string,
	workspace string,
	method string,
) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		params, err := json.Marshal(request.GetRawArguments())
		if err != nil {
			return nil, fmt.Errorf("mcp: encode %q arguments: %w", method, err)
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
		return mcpgo.NewToolResultJSON(structured)
	}
}

func closeHostAPISession(invoker HostAPIInvoker, serveSessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), hostAPISessionCloseTimeout)
	defer cancel()
	if err := invoker.CloseHostAPISession(ctx, serveSessionID); err != nil {
		return fmt.Errorf("mcp: close host api session: %w", err)
	}
	return nil
}
