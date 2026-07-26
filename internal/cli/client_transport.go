package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"
	"os"

	"strings"
	"syscall"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"

	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"

	"github.com/compozy/agh/internal/sse"
)

func (c *unixSocketClient) doJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	responseBody any,
) (err error) {
	response, err := c.doRequest(ctx, method, path, query, requestBody)
	if err != nil {
		return err
	}
	defer mergeResponseBodyCloseError(&err, response, method, path)

	return c.decodeJSONResponse(ctx, method, path, response, responseBody)
}

func (c *unixSocketClient) doAgentJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	credentials agentidentity.Credentials,
	responseBody any,
) (err error) {
	response, err := c.doRequestWithCredentials(ctx, method, path, query, requestBody, "", credentials)
	if err != nil {
		return err
	}
	defer mergeResponseBodyCloseError(&err, response, method, path)

	return c.decodeJSONResponse(ctx, method, path, response, responseBody)
}

func (c *unixSocketClient) decodeJSONResponse(
	_ context.Context,
	method string,
	path string,
	response *http.Response,
	responseBody any,
) error {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readAPIError(response)
	}
	if responseBody == nil {
		return drainResponseBody(method, path, response.Body)
	}
	if err := json.NewDecoder(response.Body).Decode(responseBody); err != nil {
		return fmt.Errorf("cli: decode %s %s response: %w", method, path, err)
	}
	return nil
}

func (c *unixSocketClient) doSSE(
	ctx context.Context,
	path string,
	query url.Values,
	lastEventID string,
	handler SSEHandler,
) (err error) {
	response, err := c.doRequestWithCredentialsAndClient(
		ctx,
		http.MethodGet,
		path,
		query,
		nil,
		lastEventID,
		agentidentity.Credentials{},
		c.streamHTTPClient(),
	)
	if err != nil {
		return err
	}
	defer mergeResponseBodyCloseError(&err, response, http.MethodGet, path)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readAPIError(response)
	}

	if handler == nil {
		return drainResponseBody(http.MethodGet, path, response.Body)
	}
	return decodeSSE(ctx, response.Body, handler)
}

func mergeResponseBodyCloseError(
	target *error,
	response *http.Response,
	method string,
	path string,
) {
	if target == nil || response == nil || response.Body == nil {
		return
	}
	if err := response.Body.Close(); err != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("cli: close %s %s response body: %w", method, path, err),
		)
	}
}

func (c *unixSocketClient) doRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
) (*http.Response, error) {
	return c.doRequestWithCredentials(
		ctx,
		method,
		path,
		query,
		requestBody,
		"",
		agentidentity.Credentials{},
	)
}

func (c *unixSocketClient) doRequestWithCredentials(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	lastEventID string,
	credentials agentidentity.Credentials,
) (*http.Response, error) {
	return c.doRequestWithCredentialsAndClient(
		ctx,
		method,
		path,
		query,
		requestBody,
		lastEventID,
		credentials,
		c.httpClient,
	)
}

// doRequestWithCredentialsAndClient lets SSE streams opt out of the JSON request timeout.
func (c *unixSocketClient) doRequestWithCredentialsAndClient(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	lastEventID string,
	credentials agentidentity.Credentials,
	client *http.Client,
) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("cli: context is required")
	}
	if client == nil {
		return nil, errors.New("cli: http client is required")
	}

	target := baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("cli: encode %s %s request: %w", method, path, err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("cli: build %s %s request: %w", method, path, err)
	}
	req.Header.Set("User-Agent", defaultUserAgentName)
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(lastEventID) != "" {
		req.Header.Set("Last-Event-ID", strings.TrimSpace(lastEventID))
	}
	setAgentIdentityHeaders(req, credentials)

	response, err := client.Do(req)
	if err != nil {
		if isDaemonUnavailableTransportError(err) {
			return nil, newDaemonUnavailableError(c.socketPath, method, path, err)
		}
		return nil, fmt.Errorf("cli: %s %s via %s: %w", method, path, c.socketPath, err)
	}
	return response, nil
}

func newDaemonUnavailableError(socketPath string, method string, path string, err error) error {
	item := diagnosticspkg.NewItem(
		"cli.daemon_unavailable",
		contract.CodeDaemonUnavailable,
		contract.CategoryDaemon,
		"Daemon unavailable",
		fmt.Sprintf("AGH daemon is not reachable at %s while requesting %s %s.", socketPath, method, path),
		contract.SeverityError,
		contract.FreshnessOffline,
		diagnosticspkg.WithSuggestedCommand("agh daemon start"),
		diagnosticspkg.WithEvidence(map[string]any{
			"socket_path": socketPath,
			"method":      method,
			"path":        path,
			"cause":       err,
		}),
	)
	return diagnosticspkg.NewStructuredError(item, err)
}

func isDaemonUnavailableTransportError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
}

// streamHTTPClient preserves long-lived streams when no dedicated client has been configured.
func (c *unixSocketClient) streamHTTPClient() *http.Client {
	if c != nil && c.streamClient != nil {
		return c.streamClient
	}
	if c == nil {
		return nil
	}
	return c.httpClient
}

func setAgentIdentityHeaders(req *http.Request, credentials agentidentity.Credentials) {
	if req == nil {
		return
	}
	if sessionID := strings.TrimSpace(credentials.SessionID); sessionID != "" {
		req.Header.Set(agentidentity.HeaderSessionID, sessionID)
	}
	if agentName := strings.TrimSpace(credentials.AgentName); agentName != "" {
		req.Header.Set(agentidentity.HeaderAgent, agentName)
	}
	if workspaceID := strings.TrimSpace(credentials.WorkspaceID); workspaceID != "" {
		req.Header.Set(agentidentity.HeaderWorkspaceID, workspaceID)
	}
}

func decodeSSE(ctx context.Context, body io.ReadCloser, handler SSEHandler) error {
	return sse.Decode(ctx, body, handler)
}
