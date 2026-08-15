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

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"

	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"

	"github.com/compozy/compozy/internal/sse"
)

const clientTasksPath = "/api/tasks"

func (c *daemonClient) doJSON(
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

func (c *daemonClient) doAgentJSON(
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

func (c *daemonClient) decodeJSONResponse(
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
	return decodeJSONResponseBody(method, path, response.Body, responseBody)
}

func (c *daemonClient) doSSE(
	ctx context.Context,
	path string,
	query url.Values,
	lastEventID string,
	handler SSEHandler,
) (err error) {
	if err := c.validateTargetOperation(http.MethodGet, path); err != nil {
		return err
	}
	query, err = c.withFreshStreamTicket(ctx, query)
	if err != nil {
		return err
	}
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
	if err := decodeSSE(ctx, response.Body, handler); err != nil {
		if c.target.isRemoteGateway() && ctx.Err() == nil {
			return newGatewayStreamInterruptedError(c.target, err)
		}
		return err
	}
	return nil
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

func (c *daemonClient) doRequest(
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

func (c *daemonClient) doRequestWithCredentials(
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
func (c *daemonClient) doRequestWithCredentialsAndClient(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	lastEventID string,
	credentials agentidentity.Credentials,
	client *http.Client,
) (*http.Response, error) {
	var body io.Reader
	contentType := ""
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("cli: encode %s %s request: %w", method, path, err)
		}
		body = bytes.NewReader(payload)
		contentType = "application/json"
	}
	return c.doRequestWithReaderAndClient(ctx, clientReaderRequest{
		Method:      method,
		Path:        path,
		Query:       query,
		Body:        body,
		ContentType: contentType,
		LastEventID: lastEventID,
		Credentials: credentials,
		Client:      client,
	})
}

type clientReaderRequest struct {
	Method      string
	Path        string
	Query       url.Values
	Body        io.Reader
	ContentType string
	LastEventID string
	Credentials agentidentity.Credentials
	Client      *http.Client
}

func (c *daemonClient) doRequestWithReaderAndClient(
	ctx context.Context,
	options clientReaderRequest,
) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("cli: context is required")
	}
	if options.Client == nil {
		return nil, errors.New("cli: http client is required")
	}
	if err := c.validateTargetOperation(options.Method, options.Path); err != nil {
		return nil, err
	}

	target := c.target.requestBaseURL() + options.Path
	if len(options.Query) > 0 {
		target += "?" + options.Query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, options.Method, target, options.Body)
	if err != nil {
		return nil, fmt.Errorf("cli: build %s %s request: %w", options.Method, options.Path, err)
	}
	req.Header.Set("User-Agent", defaultUserAgentName)
	if strings.TrimSpace(options.ContentType) != "" {
		req.Header.Set("Content-Type", options.ContentType)
	}
	if strings.TrimSpace(options.LastEventID) != "" {
		req.Header.Set("Last-Event-ID", strings.TrimSpace(options.LastEventID))
	}
	setAgentIdentityHeaders(req, options.Credentials)
	if c.target.isRemoteGateway() && strings.TrimSpace(c.target.credential) != "" {
		req.Header.Set("Authorization", "Bearer "+c.target.credential)
	}

	response, err := options.Client.Do(req)
	if err != nil {
		if c.target.isRemoteGateway() {
			return nil, newGatewayReachabilityError(c.target, err)
		}
		if c.target.kind == clientTargetLocal && isDaemonUnavailableTransportError(err) {
			return nil, newDaemonUnavailableError(c.target.socketPath, options.Method, options.Path, err)
		}
		return nil, fmt.Errorf("cli: %s %s via %s: %w", options.Method, options.Path, c.target.displayName(), err)
	}
	return response, nil
}

func (c *daemonClient) validateTargetOperation(method string, path string) error {
	if c.target.isRemoteGateway() && isLocalOnlyClientOperation(method, path) {
		return newGatewayLocalOnlyError(method, path)
	}
	return nil
}

func (c *daemonClient) withFreshStreamTicket(ctx context.Context, query url.Values) (url.Values, error) {
	if c == nil || !c.target.isRemoteGateway() {
		return query, nil
	}
	var ticket contract.GatewayStreamTicketPayload
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/gateway/stream-tickets",
		nil,
		nil,
		&ticket,
	); err != nil {
		return nil, fmt.Errorf("cli: acquire gateway stream ticket: %w", err)
	}
	if strings.TrimSpace(ticket.Ticket) == "" {
		return nil, errors.New("cli: gateway returned an empty stream ticket")
	}
	values := url.Values{}
	for key, entries := range query {
		values[key] = append([]string(nil), entries...)
	}
	values.Set("ticket", strings.TrimSpace(ticket.Ticket))
	return values, nil
}

func isLocalOnlyClientOperation(method string, path string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	normalized := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if hasAPIPathPrefix(normalized, "/api/agent") || hasAPIPathPrefix(normalized, "/api/internal") {
		return true
	}
	if hasAPIPathPrefix(normalized, "/api/scheduler") {
		return true
	}
	if hasAPIPathPrefix(normalized, "/api/resources") &&
		(method == http.MethodPut || method == http.MethodPost ||
			method == http.MethodPatch || method == http.MethodDelete) {
		return true
	}
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	return normalized == clientTasksPath || strings.HasPrefix(normalized, clientTasksPath+"/") ||
		strings.HasPrefix(normalized, "/api/task-runs/") ||
		strings.HasPrefix(normalized, "/api/task-reviews/") ||
		strings.HasPrefix(normalized, "/api/runs/")
}

func hasAPIPathPrefix(path string, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func newGatewayLocalOnlyError(method string, path string) error {
	return &gatewayClientError{
		code:       "gateway_local_only_operation",
		statusCode: http.StatusForbidden,
		message:    fmt.Sprintf("%s %s is available only through local UDS or an SSH forward", method, path),
	}
}

func newDaemonUnavailableError(socketPath string, method string, path string, err error) error {
	item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
		ID:       "cli.daemon_unavailable",
		Code:     contract.CodeDaemonUnavailable,
		Category: contract.CategoryDaemon,
		Title:    "Daemon unavailable",
		Message: fmt.Sprintf(
			"CompozyOS daemon is not reachable at %s while requesting %s %s.",
			socketPath,
			method,
			path,
		),
		Severity:      contract.SeverityError,
		DataFreshness: contract.FreshnessOffline,
	},
		diagnosticspkg.WithSuggestedCommand("compozy daemon start"),
		diagnosticspkg.WithEvidence(map[string]any{
			"socket_path":     socketPath,
			"method":          method,
			automationPathKey: path,
			"cause":           err,
		}),
	)
	return diagnosticspkg.NewStructuredError(item, err)
}

func isDaemonUnavailableTransportError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
}

// streamHTTPClient preserves long-lived streams when no dedicated client has been configured.
func (c *daemonClient) streamHTTPClient() *http.Client {
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
}

func decodeSSE(ctx context.Context, reader io.Reader, handler SSEHandler) error {
	return sse.Decode(ctx, reader, handler)
}
