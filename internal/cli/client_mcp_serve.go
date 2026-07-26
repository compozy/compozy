package cli

import (
	"context"
	"encoding/json"
	"net/http"

	mcppkg "github.com/compozy/agh/internal/mcp"
)

var _ mcppkg.HostAPIInvoker = (*unixSocketClient)(nil)

func (c *unixSocketClient) InvokeHostAPI(
	ctx context.Context,
	serveSessionID string,
	workspace string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	request := mcppkg.HostAPIInvokeRequest{
		ServeSessionID: serveSessionID,
		Workspace:      workspace,
		Method:         method,
		Params:         params,
	}
	var response mcppkg.HostAPIInvokeResponse
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/internal/mcp/host-api/invoke",
		nil,
		request,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Result, nil
}

func (c *unixSocketClient) CloseHostAPISession(ctx context.Context, serveSessionID string) error {
	return c.doJSON(
		ctx,
		http.MethodPost,
		"/api/internal/mcp/host-api/session/close",
		nil,
		mcppkg.HostAPISessionCloseRequest{ServeSessionID: serveSessionID},
		nil,
	)
}
